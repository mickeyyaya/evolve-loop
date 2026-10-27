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
	if len(sc.gotArgs) == 0 || sc.gotArgs[0] != "/bin/phase-scout" {
		t.Errorf("argv=%v, want first=/bin/phase-scout", sc.gotArgs)
	}
}

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

func TestSubprocess_MalformedJSON_Errors(t *testing.T) {
	sc := &scriptedCmd{exitCode: 0, stdout: "not json at all"}
	r := NewSubprocess("scout", "/x", sc.runner())
	_, err := r.Run(context.Background(), core.PhaseRequest{})
	if err == nil {
		t.Error("want JSON parse error")
	}
}

func TestSubprocess_RunnerError_Propagated(t *testing.T) {
	sc := &scriptedCmd{err: errors.New("exec: file not found")}
	r := NewSubprocess("scout", "/no/such/bin", sc.runner())
	_, err := r.Run(context.Background(), core.PhaseRequest{})
	if err == nil {
		t.Error("want error when CmdRunner errors")
	}
}

func TestSubprocess_Name(t *testing.T) {
	r := NewSubprocess("triage", "/x", (&scriptedCmd{}).runner())
	if r.Name() != "triage" {
		t.Errorf("Name=%q, want triage", r.Name())
	}
}

func TestNewSubprocess_NilRunnerUsesExec(t *testing.T) {
	r := NewSubprocess("scout", "/x", nil)
	if r == nil {
		t.Fatal("NewSubprocess returned nil")
	}
	_, err := r.Run(context.Background(), core.PhaseRequest{})
	if err == nil {
		t.Error("expected error from missing binary")
	}
}

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

func TestExecRunner_ExitsWithCode(t *testing.T) {
	code, err := execRunner(context.Background(), "/usr/bin/false", nil, nil, io.Discard, io.Discard)
	if err != nil {
		t.Errorf("execRunner /usr/bin/false: err=%v, want nil", err)
	}
	if code != 1 {
		t.Errorf("exitCode=%d, want 1", code)
	}
}

func TestExecRunner_BinaryMissing(t *testing.T) {
	_, err := execRunner(context.Background(), "/no/such/binary/zzz", nil, nil, io.Discard, io.Discard)
	if err == nil {
		t.Error("execRunner with missing binary: want error")
	}
}

func TestExecRunner_Success(t *testing.T) {
	code, err := execRunner(context.Background(), "/usr/bin/true", nil, nil, io.Discard, io.Discard)
	if err != nil {
		t.Errorf("execRunner /usr/bin/true: err=%v", err)
	}
	if code != 0 {
		t.Errorf("exitCode=%d, want 0", code)
	}
}

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
