// Package runner provides the in-process and subprocess PhaseRunner
// adapters. The orchestrator never knows which is in play — that's
// the Approach C independence guarantee from plan §2.
package runner

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// fakePhase implements core.PhaseRunner — used as the in-process default.
type fakePhase struct {
	name   string
	gotReq core.PhaseRequest
	resp   core.PhaseResponse
	err    error
	runs   int
}

func (f *fakePhase) Name() string { return f.name }
func (f *fakePhase) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	f.gotReq = req
	f.runs++
	return f.resp, f.err
}

// scriptedCmd is the CmdRunner-shaped stub for SubprocessRunner tests.
type scriptedCmd struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
	gotStdin string
	gotArgs  []string
}

func (s *scriptedCmd) runner() CmdRunner {
	return func(ctx context.Context, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		s.gotArgs = append([]string{name}, args...)
		if stdin != nil {
			b, _ := io.ReadAll(stdin)
			s.gotStdin = string(b)
		}
		_, _ = stdout.Write([]byte(s.stdout))
		_, _ = stderr.Write([]byte(s.stderr))
		return s.exitCode, s.err
	}
}

// TestSubprocess_HappyPath — exit 0, valid response JSON on stdout,
// adapter parses + returns the typed response.
func TestSubprocess_HappyPath(t *testing.T) {
	resp := core.PhaseResponse{
		Phase: "scout", Verdict: "PASS", CostUSD: 0.5,
		ArtifactsDir: "/x/cycle-7",
	}
	jsonOut, _ := json.Marshal(resp)
	sc := &scriptedCmd{exitCode: 0, stdout: string(jsonOut)}
	r := NewSubprocess("scout", "/bin/phase-scout", sc.runner())

	got, err := r.Run(context.Background(), core.PhaseRequest{Cycle: 7, ProjectRoot: "/x"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Phase != "scout" {
		t.Errorf("Phase=%q, want scout", got.Phase)
	}
	if got.Verdict != "PASS" {
		t.Errorf("Verdict=%q, want PASS", got.Verdict)
	}
	if got.CostUSD != 0.5 {
		t.Errorf("CostUSD=%g", got.CostUSD)
	}
	// Argv: binary name only (no extra flags).
	if len(sc.gotArgs) == 0 || sc.gotArgs[0] != "/bin/phase-scout" {
		t.Errorf("argv=%v, want first=/bin/phase-scout", sc.gotArgs)
	}
}

// TestSubprocess_RequestSentAsStdinJSON — the PhaseRequest is
// JSON-serialized to the subprocess stdin so external phase binaries
// can be written in any language.
func TestSubprocess_RequestSentAsStdinJSON(t *testing.T) {
	resp := core.PhaseResponse{Verdict: "PASS"}
	jsonOut, _ := json.Marshal(resp)
	sc := &scriptedCmd{exitCode: 0, stdout: string(jsonOut)}
	r := NewSubprocess("scout", "/x", sc.runner())

	req := core.PhaseRequest{Cycle: 42, ProjectRoot: "/p", GoalHash: "abc"}
	_, _ = r.Run(context.Background(), req)
	var got core.PhaseRequest
	if err := json.Unmarshal([]byte(sc.gotStdin), &got); err != nil {
		t.Fatalf("stdin not JSON: %v", err)
	}
	if got.Cycle != 42 || got.GoalHash != "abc" {
		t.Errorf("decoded stdin=%+v", got)
	}
}

// TestSubprocess_NonZeroExit_Errors — exit ≠ 0 must surface as an
// error including the captured stderr for diagnostics.
func TestSubprocess_NonZeroExit_Errors(t *testing.T) {
	sc := &scriptedCmd{exitCode: 1, stderr: "subagent panic", stdout: ""}
	r := NewSubprocess("scout", "/x", sc.runner())
	_, err := r.Run(context.Background(), core.PhaseRequest{})
	if err == nil {
		t.Fatal("want error on exit 1")
	}
	if !strings.Contains(err.Error(), "subagent panic") {
		t.Errorf("err=%v missing stderr context", err)
	}
}

// TestSubprocess_MalformedJSON_Errors — stdout that isn't valid
// PhaseResponse JSON must surface, not silently zero-value.
func TestSubprocess_MalformedJSON_Errors(t *testing.T) {
	sc := &scriptedCmd{exitCode: 0, stdout: "not json at all"}
	r := NewSubprocess("scout", "/x", sc.runner())
	_, err := r.Run(context.Background(), core.PhaseRequest{})
	if err == nil {
		t.Error("want JSON parse error")
	}
}

// TestSubprocess_RunnerError_Propagated — CmdRunner returning a
// non-nil error (e.g., binary missing) must wrap into the adapter's
// error.
func TestSubprocess_RunnerError_Propagated(t *testing.T) {
	sc := &scriptedCmd{err: errors.New("exec: file not found")}
	r := NewSubprocess("scout", "/no/such/bin", sc.runner())
	_, err := r.Run(context.Background(), core.PhaseRequest{})
	if err == nil {
		t.Error("want error when CmdRunner errors")
	}
}

// TestSubprocess_Name — Name() returns the phase name configured at
// construction.
func TestSubprocess_Name(t *testing.T) {
	r := NewSubprocess("triage", "/x", (&scriptedCmd{}).runner())
	if r.Name() != "triage" {
		t.Errorf("Name=%q, want triage", r.Name())
	}
}

// TestPerPhase_EnvUnset_ReturnsInProc — when EVOLVE_PHASE_<NAME>_BIN
// is unset, PerPhase returns the in-process PhaseRunner verbatim.
func TestPerPhase_EnvUnset_ReturnsInProc(t *testing.T) {
	t.Setenv("EVOLVE_PHASE_SCOUT_BIN", "")
	inproc := &fakePhase{name: "scout", resp: core.PhaseResponse{Verdict: "PASS"}}
	got := PerPhase("scout", inproc, nil)
	if got != inproc {
		t.Errorf("PerPhase returned different runner; want identity-equal to inproc")
	}
	// Sanity: a Run() must reach the fake.
	_, _ = got.Run(context.Background(), core.PhaseRequest{Cycle: 1})
	if inproc.runs != 1 {
		t.Errorf("inProc.runs=%d, want 1", inproc.runs)
	}
}

// TestPerPhase_EnvSet_ReturnsSubprocess — when EVOLVE_PHASE_<NAME>_BIN
// is set, PerPhase returns a SubprocessRunner that forks that binary.
func TestPerPhase_EnvSet_ReturnsSubprocess(t *testing.T) {
	t.Setenv("EVOLVE_PHASE_SCOUT_BIN", "/usr/local/bin/phase-scout")
	resp := core.PhaseResponse{Phase: "scout", Verdict: "PASS"}
	jsonOut, _ := json.Marshal(resp)
	sc := &scriptedCmd{exitCode: 0, stdout: string(jsonOut)}
	inproc := &fakePhase{name: "scout"}

	got := PerPhase("scout", inproc, sc.runner())
	if _, ok := got.(*SubprocessRunner); !ok {
		t.Fatalf("PerPhase returned %T, want *SubprocessRunner", got)
	}
	out, err := got.Run(context.Background(), core.PhaseRequest{Cycle: 1})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.Verdict != "PASS" {
		t.Errorf("Verdict=%q, want PASS", out.Verdict)
	}
	// inproc must NOT have been called.
	if inproc.runs != 0 {
		t.Errorf("inproc.runs=%d, want 0 (subprocess took over)", inproc.runs)
	}
}

// TestPerPhase_NormalizesPhaseName — env-var lookup uppercases the
// phase name to match the CLAUDE.md convention EVOLVE_PHASE_<NAME>_BIN
// where <NAME> is uppercase.
func TestPerPhase_NormalizesPhaseName(t *testing.T) {
	t.Setenv("EVOLVE_PHASE_BUILD_BIN", "/x")
	inproc := &fakePhase{name: "build"}
	got := PerPhase("build", inproc, (&scriptedCmd{stdout: "{}", exitCode: 0}).runner())
	if _, ok := got.(*SubprocessRunner); !ok {
		t.Errorf("expected SubprocessRunner for lowercase phase name + uppercase env")
	}
}

// TestNewSubprocess_NilRunnerUsesExec — production wiring path.
// Can't easily test exec without a binary, but verify the constructor
// doesn't panic.
func TestNewSubprocess_NilRunnerUsesExec(t *testing.T) {
	r := NewSubprocess("scout", "/x", nil)
	if r == nil {
		t.Fatal("NewSubprocess returned nil")
	}
	// Try Run — will fail because /x doesn't exist, but it shouldn't panic.
	_, err := r.Run(context.Background(), core.PhaseRequest{})
	if err == nil {
		t.Error("expected error from missing binary")
	}
}

// TestSubprocess_LongStderrTruncated — error message must be bounded.
// 400-char tail is what fits comfortably in operator logs.
func TestSubprocess_LongStderrTruncated(t *testing.T) {
	big := strings.Repeat("E", 1000)
	sc := &scriptedCmd{exitCode: 1, stderr: big}
	r := NewSubprocess("scout", "/x", sc.runner())
	_, err := r.Run(context.Background(), core.PhaseRequest{})
	if err == nil {
		t.Fatal("want error")
	}
	if len(err.Error()) > 800 {
		t.Errorf("err message too long: %d chars", len(err.Error()))
	}
	if !strings.Contains(err.Error(), "…") {
		t.Errorf("err missing truncation marker: %v", err)
	}
}

// TestExecRunner_ExitsWithCode — drives execRunner's ExitError branch.
func TestExecRunner_ExitsWithCode(t *testing.T) {
	// /usr/bin/false: exits 1 with no output, POSIX universal.
	code, err := execRunner(context.Background(), "/usr/bin/false", nil, nil, io.Discard, io.Discard)
	if err != nil {
		t.Errorf("execRunner /usr/bin/false: err=%v, want nil", err)
	}
	if code != 1 {
		t.Errorf("exitCode=%d, want 1", code)
	}
}

// TestExecRunner_BinaryMissing — non-existent binary → err non-nil.
func TestExecRunner_BinaryMissing(t *testing.T) {
	_, err := execRunner(context.Background(), "/no/such/binary/zzz", nil, nil, io.Discard, io.Discard)
	if err == nil {
		t.Error("execRunner with missing binary: want error")
	}
}

// TestExecRunner_Success — drive the (return 0, nil) happy-path
// branch with /usr/bin/true (POSIX universal exit-0 no-op).
func TestExecRunner_Success(t *testing.T) {
	code, err := execRunner(context.Background(), "/usr/bin/true", nil, nil, io.Discard, io.Discard)
	if err != nil {
		t.Errorf("execRunner /usr/bin/true: err=%v", err)
	}
	if code != 0 {
		t.Errorf("exitCode=%d, want 0", code)
	}
}

// TestSubprocess_ContextCancel — canceled ctx propagates.
func TestSubprocess_ContextCancel(t *testing.T) {
	runner := func(ctx context.Context, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return 0, ctx.Err()
	}
	r := NewSubprocess("scout", "/x", runner)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.Run(ctx, core.PhaseRequest{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err=%v, want context.Canceled", err)
	}
}
