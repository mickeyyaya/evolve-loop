package core

// failure_hook_test.go — ADR-0044 C3 (Slice 6) tests: the orchestrator's
// escalate→advise→promote hook. The plan's named invariant
// (TestPhaseRecovery_ShadowDefault_NoCorrectiveAction) lives here: below
// enforce the advisor is NEVER consulted, and even at enforce the
// deterministic registry is checked FIRST (known panes never pay for an LLM
// call — Rule 5).

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

type fakeAdviser struct {
	mu     sync.Mutex
	calls  int
	advice *recovery.FailureAdvice
	err    error
}

func (f *fakeAdviser) Advise(_ context.Context, _ FailureAdviseInput) (*recovery.FailureAdvice, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.advice, f.err
}

func writeEscalation(t *testing.T, workspace, phase, pane string) {
	t.Helper()
	data, _ := json.Marshal(map[string]any{"phase": phase, "final_pane": pane})
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, phase+"-escalation-report.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func hookOrchestrator(t *testing.T, stage config.Stage, adviser FailureAdviser) *Orchestrator {
	t.Helper()
	cfg := config.RoutingConfig{}
	cfg.PhaseRecovery = stage
	return NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil),
		WithRouting(cfg, nil), WithFailureAdviser(adviser))
}

func TestPhaseRecovery_ShadowDefault_NoCorrectiveAction(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	writeEscalation(t, ws, "build", "⚠ some never-seen fatal pane state, definitely novel")
	fa := &fakeAdviser{advice: &recovery.FailureAdvice{Cause: "dead_shell", PaneSubstr: "never-seen fatal pane", Justification: "j"}}

	// Shadow (the program default) AND off must both keep the advisor cold.
	for _, stage := range []config.Stage{config.StageShadow, config.StageOff} {
		o := hookOrchestrator(t, stage, fa)
		o.adviseOnUnclassifiedFailure(context.Background(), 1, ws, root, PhaseBuild, wrapTimeout(), nil)
	}
	if fa.calls != 0 {
		t.Fatalf("below enforce the advisor must NEVER be consulted; got %d call(s)", fa.calls)
	}
	if entries, _ := os.ReadDir(fatalSignaturesDir(root)); len(entries) != 0 {
		t.Fatal("below enforce nothing may be promoted")
	}
}

func TestPhaseRecovery_Enforce_AdvisesAndPromotes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	writeEscalation(t, ws, "build", "⚠ some never-seen fatal pane state, definitely novel")
	fa := &fakeAdviser{advice: &recovery.FailureAdvice{Cause: "dead_shell", PaneSubstr: "never-seen fatal pane", Justification: "novel shell wedge"}}
	o := hookOrchestrator(t, config.StageEnforce, fa)

	o.adviseOnUnclassifiedFailure(context.Background(), 262, ws, root, PhaseBuild, wrapTimeout(), nil)

	if fa.calls != 1 {
		t.Fatalf("enforce + unclassified pane must consult the advisor exactly once; got %d", fa.calls)
	}
	entries, err := os.ReadDir(fatalSignaturesDir(root))
	if err != nil || len(entries) != 1 {
		t.Fatalf("validated advice must be promoted durably; entries=%d err=%v", len(entries), err)
	}
	// The Slice-5 acceptance: a SECOND occurrence is caught deterministically
	// — zero further advisor calls.
	o.adviseOnUnclassifiedFailure(context.Background(), 263, ws, root, PhaseBuild, wrapTimeout(), nil)
	if fa.calls != 1 {
		t.Fatalf("the promoted signature must make the second occurrence deterministic; advisor called %d time(s)", fa.calls)
	}
}

func TestPhaseRecovery_Enforce_KnownPaneSkipsAdvisor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	// The real cycle-262 claude pane — seeded, therefore deterministic.
	writeEscalation(t, ws, "retro", "⏺ There's an issue with the selected model (auto). It may not exist.")
	fa := &fakeAdviser{advice: &recovery.FailureAdvice{Cause: "model_invalid", PaneSubstr: "irrelevant long substring", Justification: "j"}}
	o := hookOrchestrator(t, config.StageEnforce, fa)

	o.adviseOnUnclassifiedFailure(context.Background(), 1, ws, root, PhaseRetro, wrapTimeout(), nil)
	if fa.calls != 0 {
		t.Fatal("a registry-classified pane must never reach the LLM (deterministic-first)")
	}
}

func TestPhaseRecovery_Enforce_AdvisorErrorIsBestEffort(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	writeEscalation(t, ws, "build", "⚠ some never-seen fatal pane state, definitely novel")
	fa := &fakeAdviser{err: wrapTimeout()}
	o := hookOrchestrator(t, config.StageEnforce, fa)
	// Must not panic, must not promote.
	o.adviseOnUnclassifiedFailure(context.Background(), 1, ws, root, PhaseBuild, wrapTimeout(), nil)
	if entries, _ := os.ReadDir(fatalSignaturesDir(root)); len(entries) != 0 {
		t.Fatal("an advisor failure must promote nothing (escalate only)")
	}
}
