package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
)

// twoPassStub records the env at each Run call so tests can confirm
// both passes ran with the correct PERMISSION_MODE / PLAN_INPUT /
// PLAN_OUTPUT env vars. It also writes the plan artifact during pass A
// so pass B's existence check succeeds.
type twoPassStub struct {
	calls    []map[string]string
	planPath string
}

func (s *twoPassStub) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	// Capture relevant env keys for assertion.
	snapshot := map[string]string{
		"EVOLVE_BUILD_PERMISSION_MODE": os.Getenv("EVOLVE_BUILD_PERMISSION_MODE"),
		"EVOLVE_BUILD_PLAN_OUTPUT":     os.Getenv("EVOLVE_BUILD_PLAN_OUTPUT"),
		"EVOLVE_BUILD_PLAN_INPUT":      os.Getenv("EVOLVE_BUILD_PLAN_INPUT"),
	}
	s.calls = append(s.calls, snapshot)
	// First call: emulate the plan-mode agent writing the plan file.
	if len(s.calls) == 1 && s.planPath != "" {
		_ = os.MkdirAll(filepath.Dir(s.planPath), 0o755)
		_ = os.WriteFile(s.planPath, []byte("# scripted plan\n"), 0o644)
	}
	return core.PhaseResponse{Phase: "build", Verdict: core.VerdictPASS}, nil
}

// We need a real core.PhaseRunner — the registry expects that interface.
// (realStubRunner below is the canonical core.PhaseRunner adapter.)

// TestRunPlanAndExecute_TwoPass_HappyPath — pass A runs in plan mode
// with PLAN_OUTPUT set; pass B runs in acceptEdits mode with PLAN_INPUT
// set. Both succeed and the overall exit is 0.
func TestRunPlanAndExecute_TwoPass_HappyPath(t *testing.T) {
	planDir := t.TempDir()
	planPath := filepath.Join(planDir, "build-plan.md")
	stub := &twoPassStub{planPath: planPath}

	defer registry.SnapshotForTest()()
	registry.ResetForTesting()
	registry.Register("build", func(req core.PhaseRequest) core.PhaseRunner {
		// Wrap stub in a real core.PhaseRunner-satisfying type.
		return &realStubRunner{stub: stub}
	})

	t.Setenv("EVOLVE_BUILD_PERMISSION_MODE", "")
	t.Setenv("EVOLVE_BUILD_PLAN_INPUT", "")
	t.Setenv("EVOLVE_BUILD_PLAN_OUTPUT", "")

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
	// Pass A: PLAN_OUTPUT set, PERMISSION_MODE=plan
	if stub.calls[0]["EVOLVE_BUILD_PERMISSION_MODE"] != "plan" {
		t.Errorf("pass A PERMISSION_MODE=%q, want plan", stub.calls[0]["EVOLVE_BUILD_PERMISSION_MODE"])
	}
	if stub.calls[0]["EVOLVE_BUILD_PLAN_OUTPUT"] != planPath {
		t.Errorf("pass A PLAN_OUTPUT=%q, want %q", stub.calls[0]["EVOLVE_BUILD_PLAN_OUTPUT"], planPath)
	}
	// Pass B: PLAN_INPUT set, PERMISSION_MODE=acceptEdits
	if stub.calls[1]["EVOLVE_BUILD_PERMISSION_MODE"] != "acceptEdits" {
		t.Errorf("pass B PERMISSION_MODE=%q, want acceptEdits", stub.calls[1]["EVOLVE_BUILD_PERMISSION_MODE"])
	}
	if stub.calls[1]["EVOLVE_BUILD_PLAN_INPUT"] != planPath {
		t.Errorf("pass B PLAN_INPUT=%q, want %q", stub.calls[1]["EVOLVE_BUILD_PLAN_INPUT"], planPath)
	}
}

// realStubRunner adapts twoPassStub to core.PhaseRunner.
type realStubRunner struct{ stub *twoPassStub }

func (r *realStubRunner) Name() string { return "build" }
func (r *realStubRunner) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	return r.stub.Run(ctx, req)
}

// TestRunPlanAndExecute_MissingPhase_Exit10 — bad CLI args.
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

// TestRunPlanAndExecute_UnknownPhase_Exit10 — phase not in registry.
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

// TestRunPlanAndExecute_SkipExecute_OnlyRunsPassA — --skip-execute
// stops after pass A even when the plan artifact exists.
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

// TestRunPlanAndExecute_PlanArtifactMissing_Exit11 — when pass A
// returns success but the plan file doesn't exist, exit 11.
func TestRunPlanAndExecute_PlanArtifactMissing_Exit11(t *testing.T) {
	planDir := t.TempDir()
	planPath := filepath.Join(planDir, "no-plan-written.md")
	// stub.planPath is empty so the stub does NOT write the artifact.
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

// TestJoinNames_EmptyAndMulti — helper coverage.
func TestJoinNames_EmptyAndMulti(t *testing.T) {
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
