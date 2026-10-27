package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
)

type twoPassStub struct {
	calls    []core.PhaseRequest
	planPath string
}

func (s *twoPassStub) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	s.calls = append(s.calls, req)
	if len(s.calls) == 1 && s.planPath != "" {
		_ = os.MkdirAll(filepath.Dir(s.planPath), 0o755)
		_ = os.WriteFile(s.planPath, []byte("# scripted plan\n"), 0o644)
	}
	return core.PhaseResponse{Phase: "build", Verdict: core.VerdictPASS}, nil
}

func TestRunPlanAndExecute_TwoPass_HappyPath(t *testing.T) {
	planDir := t.TempDir()
	planPath := filepath.Join(planDir, "build-plan.md")
	stub := &twoPassStub{planPath: planPath}

	defer registry.SnapshotForTest()()
	registry.ResetForTesting()
	registry.Register("build", func(req core.PhaseRequest) core.PhaseRunner {
		return &realStubRunner{stub: stub}
	})

	t.Setenv("EVOLVE_BUILD_PERMISSION_MODE", "bypassPermissions")

	envJSON, _ := json.Marshal(core.PhaseRequest{Cycle: 7, ProjectRoot: "/tmp", Workspace: "/tmp"})
	var stdout, stderr bytes.Buffer
	code := runPlanAndExecute(
		[]string{"--plan-output", planPath, "build"},
		bytes.NewReader(envJSON), &stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("code=%d, want 0; stderr=%s", code, stderr.String())
	}
	if len(stub.calls) != 2 {
		t.Fatalf("expected 2 phase calls; got %d", len(stub.calls))
	}
	if got := stub.calls[0]; got.PermissionMode != "plan" || got.Cycle != 7 {
		t.Errorf("pass A request = cycle %d permission %q, want cycle 7 in plan mode", got.Cycle, got.PermissionMode)
	}
	if got := stub.calls[1]; got.PermissionMode != "acceptEdits" || got.Cycle != 7 {
		t.Errorf("pass B request = cycle %d permission %q, want cycle 7 in acceptEdits mode", got.Cycle, got.PermissionMode)
	}
}

type realStubRunner struct{ stub *twoPassStub }

func (r *realStubRunner) Name() string { return "build" }
func (r *realStubRunner) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	return r.stub.Run(ctx, req)
}

func TestRunPlanAndExecute_MissingPhase_Exit10(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runPlanAndExecute(nil, bytes.NewReader(nil), &stdout, &stderr)
	if code != 10 {
		t.Errorf("code=%d, want 10", code)
	}
	if !strings.Contains(stderr.String(), "missing <phase>") {
		t.Errorf("stderr should explain missing phase; got %q", stderr.String())
	}
}

func TestRunPlanAndExecute_UnknownPhase_Exit10(t *testing.T) {
	defer registry.SnapshotForTest()()
	registry.ResetForTesting()
	var stdout, stderr bytes.Buffer
	code := runPlanAndExecute([]string{"nope"}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 10 {
		t.Errorf("code=%d, want 10", code)
	}
	if !strings.Contains(stderr.String(), "unknown phase") {
		t.Errorf("stderr should explain unknown phase; got %q", stderr.String())
	}
}

func TestRunPlanAndExecute_SkipExecute_OnlyRunsPassA(t *testing.T) {
	planDir := t.TempDir()
	planPath := filepath.Join(planDir, "build-plan.md")
	stub := &twoPassStub{planPath: planPath}
	defer registry.SnapshotForTest()()
	registry.ResetForTesting()
	registry.Register("build", func(req core.PhaseRequest) core.PhaseRunner {
		return &realStubRunner{stub: stub}
	})
	envJSON, _ := json.Marshal(core.PhaseRequest{Cycle: 1})
	var stdout, stderr bytes.Buffer
	code := runPlanAndExecute(
		[]string{"--plan-output", planPath, "--skip-execute", "build"},
		bytes.NewReader(envJSON), &stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("code=%d, want 0", code)
	}
	if len(stub.calls) != 1 {
		t.Errorf("expected 1 phase call (pass A only); got %d", len(stub.calls))
	}
}

func TestRunPlanAndExecute_PlanArtifactMissing_Exit11(t *testing.T) {
	planDir := t.TempDir()
	planPath := filepath.Join(planDir, "no-plan-written.md")
	stub := &twoPassStub{}
	defer registry.SnapshotForTest()()
	registry.ResetForTesting()
	registry.Register("build", func(req core.PhaseRequest) core.PhaseRunner {
		return &realStubRunner{stub: stub}
	})
	envJSON, _ := json.Marshal(core.PhaseRequest{Cycle: 1})
	var stdout, stderr bytes.Buffer
	code := runPlanAndExecute(
		[]string{"--plan-output", planPath, "build"},
		bytes.NewReader(envJSON), &stdout, &stderr,
	)
	if code != 11 {
		t.Errorf("code=%d, want 11 (missing plan artifact); stderr=%s", code, stderr.String())
	}
}

func TestRunPlanAndExecute_AMalformedRequestExits1(t *testing.T) {
	defer registry.SnapshotForTest()()
	registry.ResetForTesting()
	registry.Register("build", func(req core.PhaseRequest) core.PhaseRunner { return &realStubRunner{stub: &twoPassStub{}} })
	var stdout, stderr bytes.Buffer
	if code := runPlanAndExecute([]string{"build"}, strings.NewReader("{"), &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "decode the phase request") {
		t.Errorf("code=%d stderr=%q, want 1 and a decode error", code, stderr.String())
	}
}

func TestJoinNames_EmptyAndMulti(t *testing.T) {
	t.Parallel()
	if got := joinNames(nil); got != "" {
		t.Errorf("nil → %q, want empty", got)
	}
	if got := joinNames([]string{"a"}); got != "a" {
		t.Errorf("single → %q, want a", got)
	}
	if got := joinNames([]string{"a", "b", "c"}); got != "a, b, c" {
		t.Errorf("multi → %q, want 'a, b, c'", got)
	}
}

type failingPassStub struct {
	twoPassStub
	failOn int
}

func (s *failingPassStub) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	resp, _ := s.twoPassStub.Run(ctx, req)
	if len(s.calls) == s.failOn {
		return resp, errors.New("pass failed")
	}
	return resp, nil
}

func registerPlanStub(t *testing.T, runner core.PhaseRunner) {
	t.Helper()
	t.Cleanup(registry.SnapshotForTest())
	registry.ResetForTesting()
	registry.Register("build", func(core.PhaseRequest) core.PhaseRunner { return runner })
}

func TestRunPlanAndExecute_ABadFlagOrAnUnreadableStdinStops(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runPlanAndExecute([]string{"--no-such-flag"}, strings.NewReader("{}"), &stdout, &stderr); code != 10 {
		t.Errorf("bad flag code=%d, want 10", code)
	}
	registerPlanStub(t, &realStubRunner{stub: &twoPassStub{}})
	if code := runPlanAndExecute([]string{"build"}, iotest.ErrReader(errors.New("closed")), &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "read stdin") {
		t.Errorf("unreadable stdin code=%d stderr=%q, want 1 and a read error", code, stderr.String())
	}
}

func TestRunPlanAndExecute_ThePlanPathDefaultsToTheWorkspaceThenTheWorkingDirectory(t *testing.T) {
	ws := t.TempDir()
	stub := &twoPassStub{planPath: filepath.Join(ws, "build-plan.md")}
	registerPlanStub(t, &realStubRunner{stub: stub})
	var stdout, stderr bytes.Buffer
	if code := runPlanAndExecute([]string{"--workspace", ws, "build"}, strings.NewReader("{}"), &stdout, &stderr); code != 0 {
		t.Fatalf("workspace plan code=%d stderr=%s", code, stderr.String())
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	cwdStub := &twoPassStub{planPath: "build-plan.md"}
	registerPlanStub(t, &realStubRunner{stub: cwdStub})
	if code := runPlanAndExecute([]string{"build"}, strings.NewReader("{}"), &stdout, &stderr); code != 0 || len(cwdStub.calls) != 2 {
		t.Fatalf("working-directory plan code=%d calls=%d stderr=%s", code, len(cwdStub.calls), stderr.String())
	}
}

func TestRunPlanAndExecute_AFailedPassStopsWithItsCode(t *testing.T) {
	planPath := filepath.Join(t.TempDir(), "build-plan.md")
	for failOn, want := range map[int]int{1: 1, 2: 2} {
		stub := &failingPassStub{twoPassStub: twoPassStub{planPath: planPath}, failOn: failOn}
		registerPlanStub(t, stub)
		var stdout, stderr bytes.Buffer
		if code := runPlanAndExecute([]string{"--plan-output", planPath, "build"}, strings.NewReader("{}"), &stdout, &stderr); code != want {
			t.Errorf("pass %d fails: code=%d, want %d (stderr %s)", failOn, code, want, stderr.String())
		}
	}
}

func (s *failingPassStub) Name() string { return "build" }
