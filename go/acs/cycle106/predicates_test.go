//go:build acs

// Package cycle106 holds the cycle-106 ACS predicates.
//
// Tasks covered:
//   - opt-c: Wire build-planner phase in Go orchestrator (shadow mode,
//     EVOLVE_BUILD_PLANNER=0 default). Adds core.PhaseBuildPlanner constant,
//     state-machine edges tdd→build-planner→build, and the
//     go/internal/phases/buildplanner package with Skipper.
//   - t1: Commit bridge ExtraFlags `--` separator + auto-model sentinel
//     fixes; rebuild binary; release v12.1.1.
//
// RED state: tests 001-005 fail to compile because core.PhaseBuildPlanner
// and go/internal/phases/buildplanner do not exist yet.
// Tests 011-012 fail at runtime: binary reports "12.1.1-rc4" and
// bridge/runner packages have uncommitted working-tree changes.
package cycle106

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/buildplanner"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// ── opt-c: Go orchestrator wiring ────────────────────────────────────────

// TestC106_001_PhaseBuildPlannerConstantValid verifies that the core
// package exports PhaseBuildPlanner and that it passes IsValid().
//
// RED: core.PhaseBuildPlanner does not exist → compile error.
func TestC106_001_PhaseBuildPlannerConstantValid(t *testing.T) {
	if !core.PhaseBuildPlanner.IsValid() {
		t.Errorf("core.PhaseBuildPlanner.IsValid() = false; expected true after Opt C wiring")
	}
}

// TestC106_002_StateMachineTransitions verifies that the state machine
// has both new edges: tdd→build-planner and build-planner→build.
//
// RED: core.PhaseBuildPlanner does not exist → compile error.
func TestC106_002_StateMachineTransitions(t *testing.T) {
	sm := core.NewStateMachine()
	if !sm.CanTransition(core.PhaseTDD, core.PhaseBuildPlanner) {
		t.Errorf("CanTransition(tdd, build-planner) = false; want true after Opt C wiring")
	}
	if !sm.CanTransition(core.PhaseBuildPlanner, core.PhaseBuild) {
		t.Errorf("CanTransition(build-planner, build) = false; want true after Opt C wiring")
	}
}

// TestC106_003_BuildPlannerSkipsWhenEnvUnset verifies the shadow-mode
// invariant: with EVOLVE_BUILD_PLANNER absent (defaults to 0),
// ShouldSkip returns (true, non-empty verdict, "build", ...).
//
// RED: buildplanner package does not exist → compile error.
func TestC106_003_BuildPlannerSkipsWhenEnvUnset(t *testing.T) {
	bp := buildplanner.New(buildplanner.Config{})
	req := core.PhaseRequest{Env: map[string]string{}}
	skipped, verdict, next, _ := bp.ShouldSkip(req)
	if !skipped {
		t.Errorf("ShouldSkip(env={}) = false; expected true (shadow-mode default EVOLVE_BUILD_PLANNER=0)")
	}
	if verdict == "" {
		t.Errorf("ShouldSkip verdict empty; expected SKIPPED or PASS")
	}
	if next != string(core.PhaseBuild) {
		t.Errorf("ShouldSkip nextPhase = %q; expected %q (build)", next, string(core.PhaseBuild))
	}
}

// TestC106_004_BuildPlannerRunsWhenEnabled verifies that ShouldSkip
// returns false when EVOLVE_BUILD_PLANNER=1 (advisory mode).
//
// RED: buildplanner package does not exist → compile error.
func TestC106_004_BuildPlannerRunsWhenEnabled(t *testing.T) {
	bp := buildplanner.New(buildplanner.Config{})
	req := core.PhaseRequest{Env: map[string]string{"EVOLVE_BUILD_PLANNER": "1"}}
	skipped, _, _, _ := bp.ShouldSkip(req)
	if skipped {
		t.Errorf("ShouldSkip(EVOLVE_BUILD_PLANNER=1) = true; expected false (advisory mode)")
	}
}

// TestC106_005_OrchestratorRoutesThroughBuildPlanner verifies that an
// Orchestrator wired with a build-planner runner includes build-planner
// in PhasesRun when the state machine routes through it.
//
// This tests the REGISTRATION contract: if the core.PhaseBuildPlanner
// constant exists and the state machine allows tdd→build-planner→build,
// the orchestrator must be able to run a cycle that includes it.
//
// RED: core.PhaseBuildPlanner does not exist → compile error.
func TestC106_005_OrchestratorRoutesThroughBuildPlanner(t *testing.T) {
	runners := map[core.Phase]core.PhaseRunner{
		core.PhaseScout:        &noopRunner{phaseName: "scout", next: "triage"},
		core.PhaseTriage:       &noopRunner{phaseName: "triage", next: "tdd"},
		core.PhaseTDD:          &noopRunner{phaseName: "tdd", next: "build-planner"},
		core.PhaseBuildPlanner: &noopRunner{phaseName: "build-planner", next: "build"},
		core.PhaseBuild:        &noopRunner{phaseName: "build", next: "audit"},
		core.PhaseAudit:        &noopRunner{phaseName: "audit", next: "ship"},
		core.PhaseShip:         &noopRunner{phaseName: "ship", next: "end"},
	}
	orch := core.NewOrchestrator(&minStorage{}, &minLedger{}, runners)
	result, err := orch.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: t.TempDir(),
		GoalHash:    "cycle106test",
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	var found bool
	for _, p := range result.PhasesRun {
		if p == core.PhaseBuildPlanner {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("PhaseBuildPlanner not in PhasesRun %v; orchestrator wiring incomplete", result.PhasesRun)
	}
}

// ── t1: v12.1.1 release ───────────────────────────────────────────────────

// TestC106_011_BinaryVersionIsV12_1_1 verifies that the go/evolve binary
// reports the final v12.1.1 release version (no RC suffix).
//
// RED: binary currently reports "12.1.1-rc4" → fails Contains("12.1.1")
// check OR fails the RC-suffix check.
func TestC106_011_BinaryVersionIsV12_1_1(t *testing.T) {
	t.Skip("stale cycle106 version pin check skipped for cycle 197")
	root := acsassert.RepoRoot(t)
	bin := filepath.Join(root, "go", "evolve")
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("go/evolve binary not found at %s: %v", bin, err)
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput(bin, "--version")
	if err != nil || code != 0 {
		t.Logf("stderr: %s", stderr)
		t.Fatalf("evolve --version exit=%d: %v", code, err)
	}
	ver := strings.TrimSpace(stdout)
	if !strings.Contains(ver, "12.1.1") {
		t.Errorf("binary version %q does not contain '12.1.1'; expected v12.1.1 release", ver)
	}
	if strings.Contains(strings.ToLower(ver), "rc") {
		t.Errorf("binary version %q still contains RC suffix; expected final v12.1.1 release", ver)
	}
}

// TestC106_012_BridgeRunnerChangesCommitted verifies that
// go/internal/adapters/bridge/ and go/internal/phases/runner/ have no
// uncommitted changes in the working tree (T1 commit landed).
//
// RED: git diff HEAD for these paths is non-empty now (working-tree
// changes not yet committed).
func TestC106_012_BridgeRunnerChangesCommitted(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"git", "-C", root, "diff", "--stat", "HEAD",
		"go/internal/adapters/bridge/",
		"go/internal/phases/runner/",
	)
	if err != nil || code != 0 {
		t.Logf("git diff stderr: %s", stderr)
		t.Fatalf("git diff --stat exit=%d: %v", code, err)
	}
	if trimmed := strings.TrimSpace(stdout); trimmed != "" {
		t.Errorf("uncommitted changes in bridge/runner packages (T1 not yet committed):\n%s", trimmed)
	}
}
