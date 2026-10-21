package phasecmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func registerStub(t *testing.T, stub *stubPhase) {
	t.Helper()
	restore := registry.SnapshotForTest()
	t.Cleanup(restore)
	registry.ResetForTesting()
	registry.Register("intent", func(core.PhaseRequest) core.PhaseRunner { return stub })
}

func emptyRegistryProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "architecture")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "phase-registry.json"), []byte(`{"schema_version":4,"phases":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRunPhase_LintIsDispatchedCaseInsensitively(t *testing.T) {
	for _, verb := range []string{"lint", "LINT", "Lint"} {
		t.Run(verb, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := NewRunPhase(nil, nil)([]string{verb}, nil, &stdout, &stderr)

			if code != 10 || !strings.Contains(stderr.String(), "usage: evolve phase lint <name>") {
				t.Errorf("code=%d stderr=%q, want the lint usage error (10)", code, stderr.String())
			}
		})
	}
}

func TestRunPhase_UnmarshalableResponseExits1WithNothingOnStdout(t *testing.T) {
	registerStub(t, &stubPhase{resp: core.PhaseResponse{Phase: "intent", CostUSD: math.NaN()}})
	var stdout, stderr bytes.Buffer

	code := NewRunPhase(nil, nil)([]string{"intent"}, strings.NewReader(`{"cycle":1}`), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("code=%d, want 1", code)
	}
	if !strings.HasPrefix(stderr.String(), "evolve phase: marshal response:") {
		t.Errorf("stderr=%q, want the marshal failure named", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout=%q, want nothing on a marshal failure", stdout.String())
	}
}

func TestRunPhase_SpecKnownAtTheCwdRootButUnloadableAtTheRequestRootExits1(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", specPhaseProject(t))
	broken := t.TempDir()
	reqJSON, _ := json.Marshal(core.PhaseRequest{Cycle: 3, ProjectRoot: broken})
	var stdout, stderr bytes.Buffer

	code := NewRunPhase(nil, nil)([]string{"plan-review"}, bytes.NewReader(reqJSON), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("code=%d stderr=%q, want 1", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "evolve phase: plan-review: load phase catalog") {
		t.Errorf("stderr=%q, want the catalog failure named for the phase", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout=%q, want nothing when no runner resolves", stdout.String())
	}
}

func TestResolvePhaseRunner_ExitCodes(t *testing.T) {
	spec := specPhaseProject(t)
	cases := []struct {
		name     string
		phase    string
		root     string
		wantCode int
		wantErr  string
	}{
		{"spec phase resolves", "plan-review", spec, 0, ""},
		{"unrunnable spec is unknown", "memo", spec, 10, `unknown phase "memo"`},
		{"missing catalog is an error", "plan-review", t.TempDir(), 1, "evolve phase: plan-review: load phase catalog"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer

			runner, code := resolvePhaseRunner(tc.phase, core.PhaseRequest{ProjectRoot: tc.root}, &stderr)

			if code != tc.wantCode {
				t.Fatalf("code=%d, want %d (stderr=%q)", code, tc.wantCode, stderr.String())
			}
			if (runner != nil) != (tc.wantCode == 0) {
				t.Errorf("runner=%v, want non-nil only on success", runner)
			}
			if !strings.Contains(stderr.String(), tc.wantErr) {
				t.Errorf("stderr=%q, want %q", stderr.String(), tc.wantErr)
			}
		})
	}
}

func TestRunPhase_CycleFlagDerivesTheRequestFromCycleState(t *testing.T) {
	root, state := writeRerunFixture(t, 9)
	stub := &stubPhase{resp: core.PhaseResponse{Phase: "intent", Verdict: core.VerdictPASS}}
	registerStub(t, stub)
	var stdout, stderr bytes.Buffer

	code := NewRunPhase(nil, nil)([]string{"intent", "--cycle", "9", "--project-root", root}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("code=%d stderr=%q, want 0", code, stderr.String())
	}
	if stub.got.Cycle != 9 || stub.got.Worktree != state.ActiveWorktree || stub.got.RunID != "01RERUN" {
		t.Errorf("runner got %+v, want the request derived from cycle 9's state", stub.got)
	}
}

func TestPhaseRequest_FlagAndCycleFailuresMapToExitCodes(t *testing.T) {
	root, _ := writeRerunFixture(t, 9)
	cases := []struct {
		name     string
		flags    []string
		wantCode int
		wantErr  string
	}{
		{"unknown flag", []string{"--bogus"}, exitCycleUsage, "flag provided but not defined"},
		{"non-numeric cycle", []string{"--cycle", "nine"}, exitCycleUsage, "invalid value"},
		{"negative cycle", []string{"--cycle", "-1", "--project-root", root}, exitCycleUsage, "not a positive cycle number"},
		{"cycle without state", []string{"--cycle", "10", "--project-root", root}, exitCycleRefused, "evolve phase: cycle 10 state not found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer

			req, code := phaseRequest("scout", tc.flags, nil, &stderr)

			if code != tc.wantCode {
				t.Fatalf("code=%d, want %d (stderr=%q)", code, tc.wantCode, stderr.String())
			}
			if !reflect.DeepEqual(req, core.PhaseRequest{}) {
				t.Errorf("req=%+v, want the zero request on failure", req)
			}
			if !strings.Contains(stderr.String(), tc.wantErr) {
				t.Errorf("stderr=%q, want %q", stderr.String(), tc.wantErr)
			}
		})
	}
}

func TestRequestForCycle_ReadsTheRootFromTheEnvironmentWhenNoFlagGiven(t *testing.T) {
	root, _ := writeRerunFixture(t, 4)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)

	req, err := RequestForCycle("", 4, nil)

	if err != nil {
		t.Fatalf("RequestForCycle: %v", err)
	}
	if req.ProjectRoot != root || req.Workspace != paths.RunWorkspace(root, 4) {
		t.Errorf("req root=%q workspace=%q, want the env root %q", req.ProjectRoot, req.Workspace, root)
	}
}

func TestRequestForCycle_RefusesACycleHeldByALiveRun(t *testing.T) {
	root, _ := writeRerunFixture(t, 9)
	lease := runlease.Lease{RunID: "01LIVE", OwnerPID: os.Getpid()}
	if err := runlease.Write(paths.RunWorkspace(root, 9), lease, time.Now()); err != nil {
		t.Fatal(err)
	}

	_, err := RequestForCycle(root, 9, nil)

	if err == nil || !strings.Contains(err.Error(), "cycle 9 is held by live run 01LIVE") {
		t.Fatalf("err=%v, want the live-run refusal", err)
	}
	if CycleRequestExitCode(err) != exitCycleRefused {
		t.Errorf("exit code=%d, want refused (%d)", CycleRequestExitCode(err), exitCycleRefused)
	}
}

func TestRequestForCycle_IgnoresAStaleLease(t *testing.T) {
	root, _ := writeRerunFixture(t, 9)
	stale := time.Now().Add(-runlease.DefaultTTL - time.Minute)
	if err := runlease.Write(paths.RunWorkspace(root, 9), runlease.Lease{RunID: "01DEAD", OwnerPID: os.Getpid()}, stale); err != nil {
		t.Fatal(err)
	}

	if _, err := RequestForCycle(root, 9, nil); err != nil {
		t.Errorf("RequestForCycle: %v, want a stale lease ignored", err)
	}
}

func TestRequestForCycle_StdinReadFailureIsRefused(t *testing.T) {
	root, _ := writeRerunFixture(t, 9)

	_, err := RequestForCycle(root, 9, iotest.ErrReader(errors.New("pipe broke")))

	if err == nil || err.Error() != "read stdin: pipe broke" {
		t.Fatalf("err=%v, want read stdin: pipe broke", err)
	}
	if CycleRequestExitCode(err) != exitCycleRefused {
		t.Errorf("exit code=%d, want refused", CycleRequestExitCode(err))
	}
}

func TestRequestForCycle_WorktreeProblemsAreRefused(t *testing.T) {
	cases := []struct {
		name     string
		worktree func(string) string
		want     string
	}{
		{"no worktree recorded", func(string) string { return "" }, "cycle 9 records no worktree"},
		{"worktree removed", func(root string) string { return filepath.Join(root, "gone") }, "cycle 9 worktree"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, state := writeRerunFixture(t, 9)
			state.ActiveWorktree = tc.worktree(root)
			raw, _ := json.Marshal(state)
			if err := os.WriteFile(filepath.Join(paths.RunWorkspace(root, 9), core.CycleStateFile), raw, 0o644); err != nil {
				t.Fatal(err)
			}

			_, err := RequestForCycle(root, 9, nil)

			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestRequestForCycle_UnparseableStateIsRefused(t *testing.T) {
	root, _ := writeRerunFixture(t, 9)
	if err := os.WriteFile(filepath.Join(paths.RunWorkspace(root, 9), core.CycleStateFile), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := RequestForCycle(root, 9, nil)

	if err == nil || !strings.Contains(err.Error(), "cycle 9 state at") || !strings.Contains(err.Error(), "is unparseable") {
		t.Fatalf("err=%v, want the unparseable-state refusal", err)
	}
}

func TestStdinCarriesRequest(t *testing.T) {
	regular := filepath.Join(t.TempDir(), "req.json")
	if err := os.WriteFile(regular, []byte(`{"cycle":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	blank := filepath.Join(t.TempDir(), "blank")
	if err := os.WriteFile(blank, []byte(" \n\t"), 0o644); err != nil {
		t.Fatal(err)
	}
	open := func(path string) *os.File {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = f.Close() })
		return f
	}
	cases := []struct {
		name  string
		stdin func() io.Reader
		want  bool
	}{
		{"character device is a terminal", func() io.Reader { return open(os.DevNull) }, false},
		{"regular file with a request", func() io.Reader { return open(regular) }, true},
		{"regular file with only whitespace", func() io.Reader { return open(blank) }, false},
		{"unicode body", func() io.Reader { return strings.NewReader("é") }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := stdinCarriesRequest(tc.stdin())
			if err != nil || got != tc.want {
				t.Errorf("stdinCarriesRequest = (%v, %v), want (%v, nil)", got, err, tc.want)
			}
		})
	}
}

func TestStdinCarriesRequest_ClosedFileFailsTheRead(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	got, err := stdinCarriesRequest(f)

	if err == nil || got {
		t.Errorf("stdinCarriesRequest(closed) = (%v, %v), want (false, error)", got, err)
	}
}

func TestResolveRunner_RunnableSpecBuildsASpecRunner(t *testing.T) {
	spec := specPhaseProject(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", spec)
	for name, root := range map[string]string{"explicit root": spec, "env root": ""} {
		t.Run(name, func(t *testing.T) {
			runner, found, err := ResolveRunner("Plan-Review", core.PhaseRequest{ProjectRoot: root})

			if err != nil || !found || runner == nil {
				t.Fatalf("ResolveRunner = (%v, %v, %v), want a spec runner", runner, found, err)
			}
			if runner.Name() != "plan-review" {
				t.Errorf("runner.Name() = %q, want plan-review", runner.Name())
			}
		})
	}
}

func TestFormatUnknownPhaseError_ListsSpecsOnlyWhenTheCatalogHasThem(t *testing.T) {
	withSpecs := FormatUnknownPhaseError("evolve phase", "nope", specPhaseProject(t))
	if !strings.Contains(withSpecs, `evolve phase: unknown phase "nope" (known built-in: `) ||
		!strings.Contains(withSpecs, "; known spec: ") || !strings.Contains(withSpecs, "plan-review") {
		t.Errorf("with specs = %q, want built-in and spec lists", withSpecs)
	}

	noSpecs := FormatUnknownPhaseError("evolve phase", "nope", emptyRegistryProject(t))
	if !strings.HasPrefix(noSpecs, `evolve phase: unknown phase "nope" (known: `) || strings.Contains(noSpecs, "known spec") ||
		strings.Contains(noSpecs, "unavailable") {
		t.Errorf("no specs = %q, want only the built-in list", noSpecs)
	}
}

func TestPhasesCheckProvenance_ExitCodes(t *testing.T) {
	cases := []struct {
		name       string
		report     string
		args       []string
		stdout     func() (any, *bytes.Buffer)
		breakRoot  bool
		wantCode   int
		wantStderr string
		wantStdout string
	}{
		{name: "unknown flag is usage", report: "# r\n", args: []string{"--bogus"}, wantCode: provenanceExitUsage, wantStderr: "flag provided but not defined"},
		{name: "zero cycle is usage", report: "# r\n", args: []string{"--cycle", "0"}, wantCode: provenanceExitUsage, wantStderr: "usage: evolve phases check-provenance"},
		{name: "phase mismatch is a violation", report: "<!-- evolve:provenance phase=scout cycle=5 -->\n# r\n", args: []string{"--cycle", "5"}, wantCode: provenanceExitViolation, wantStdout: `phase mismatch: got "scout", want "build"`},
		{name: "cycle mismatch is a violation as JSON", report: "<!-- evolve:provenance phase=build cycle=4 -->\n# r\n", args: []string{"--cycle", "5", "--json"}, wantCode: provenanceExitViolation, wantStdout: `"severity": "ERROR"`},
		{name: "matching header is clean", report: "<!-- evolve:provenance phase=build cycle=5 -->\n# r\n", args: []string{"--cycle", "5"}, wantCode: provenanceExitClean, wantStdout: "1 artifact(s) checked, 0 provenance finding(s)"},
		{name: "missing catalog is unreadable", report: "# r\n", args: []string{"--cycle", "5"}, breakRoot: true, wantCode: provenanceExitUnreadable, wantStderr: "check-provenance: load phase catalog"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provenanceFixture(t, tc.report)
			if tc.breakRoot {
				t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
			}
			var stdout, stderr bytes.Buffer

			code := RunPhases(append([]string{"check-provenance"}, tc.args...), nil, &stdout, &stderr)

			if code != tc.wantCode {
				t.Fatalf("code=%d, want %d (stdout=%q stderr=%q)", code, tc.wantCode, stdout.String(), stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.wantStderr) || !strings.Contains(stdout.String(), tc.wantStdout) {
				t.Errorf("stdout=%q stderr=%q, want stdout %q and stderr %q", stdout.String(), stderr.String(), tc.wantStdout, tc.wantStderr)
			}
		})
	}
}

func TestPhasesCheckProvenance_WriteFailureIsUnreadable(t *testing.T) {
	provenanceFixture(t, "# r\n")
	var stderr bytes.Buffer

	code := phasesCheckProvenance(phasesCall{project: os.Getenv("EVOLVE_PROJECT_ROOT"), args: []string{"--cycle", "5"}, stdout: errWriter{}, stderr: &stderr})

	if code != provenanceExitUnreadable || !strings.Contains(stderr.String(), "check-provenance: write: disk full") {
		t.Errorf("code=%d stderr=%q, want %d and the write failure", code, stderr.String(), provenanceExitUnreadable)
	}
}
