package core

// retro_routed_test.go — failure floor Phase 3, orchestrator half: at
// Stage>=Advisory the retro failure branch goes through the routing
// strategy (advisor failure vocabulary, BLOCK floor intact) and emits a
// routing-decision artifact — failure branches get the same forensic
// trail as happy-path transitions.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func advisoryOrchestrator(led *fakeLedger, strat router.RoutingStrategy) *Orchestrator {
	return &Orchestrator{
		ledger:   led,
		now:      coverNow,
		strategy: strat,
		cfg:      config.RoutingConfig{Stage: config.StageAdvisory},
		sm:       NewStateMachine(),
	}
}

func TestDecideAfterRetro_EmitsRoutingDecisionArtifact(t *testing.T) {
	t.Parallel()
	led := &fakeLedger{}
	o := advisoryOrchestrator(led, router.StaticPreset{})
	ws := t.TempDir()

	next, _, reason, _ := o.decideAfterRetroRouted(context.Background(), 5, CycleState{WorkspacePath: ws}, 3, VerdictFAIL, nil, router.RouteInput{})

	// Empty history, non-strict → adapter PROCEED → end (kernel default).
	// The advisor agrees, so the operator-facing reason contract holds.
	if next != PhaseEnd {
		t.Errorf("next = %s, want end", next)
	}
	if !strings.HasPrefix(reason, "proceed:") {
		t.Errorf("reason = %q, want the deterministic contract prefix when advisor agrees", reason)
	}
	if _, err := os.Stat(filepath.Join(ws, "routing-decision-3.json")); err != nil {
		t.Errorf("failure branch must emit a routing-decision artifact: %v", err)
	}
}

func TestDecideAfterRetro_PassArmShipsWithoutArtifact(t *testing.T) {
	t.Parallel()
	led := &fakeLedger{}
	o := advisoryOrchestrator(led, router.StaticPreset{})
	ws := t.TempDir()

	next, _, _, _ := o.decideAfterRetroRouted(context.Background(), 5, CycleState{WorkspacePath: ws}, 3, VerdictPASS, nil, router.RouteInput{})
	if next != PhaseShip {
		t.Errorf("next = %s, want ship (retro PASS recovers)", next)
	}
	if entries, _ := filepath.Glob(filepath.Join(ws, "routing-decision-*.json")); len(entries) != 0 {
		t.Errorf("PASS arm is not a failure branch; no artifact expected, got %v", entries)
	}
}

// fixedNextStrategy returns a canned decision — used to drive branches a
// real Route would not produce from this fixture.
type fixedNextStrategy struct{ next string }

func (s fixedNextStrategy) Decide(router.RouteInput) router.RouterDecision {
	return router.RouterDecision{NextPhase: s.next, Reason: "stub", Evidence: map[string]interface{}{}}
}
func (s fixedNextStrategy) Recover(in router.RouteInput) router.RouterDecision {
	return router.Recover(in)
}

// The SM has no retro→fault-localization edge yet: a routed insert is
// clamped to the legal retry target (tdd) — kernel disposes — and the
// clamp is visible in the artifact.
func TestDecideAfterRetro_InsertClampedToLegalRetry(t *testing.T) {
	t.Parallel()
	led := &fakeLedger{}
	o := advisoryOrchestrator(led, fixedNextStrategy{next: "fault-localization"})
	ws := t.TempDir()

	next, _, _, _ := o.decideAfterRetroRouted(context.Background(), 5, CycleState{WorkspacePath: ws}, 1, VerdictFAIL, nil, router.RouteInput{})
	if next != PhaseTDD {
		t.Errorf("next = %s, want tdd (insert clamped to legal SM edge)", next)
	}
	raw, err := os.ReadFile(filepath.Join(ws, "routing-decision-1.json"))
	if err != nil {
		t.Fatalf("artifact: %v", err)
	}
	if !strings.Contains(string(raw), "retro-branch-sm-clamped") {
		t.Errorf("artifact must record the SM clamp:\n%s", raw)
	}
}

// A non-insert illegal phase must NOT upgrade the deterministic branch —
// it clamps back to the kernel baseline (end here), never to a retry.
func TestDecideAfterRetro_ArbitraryIllegalPhaseClampsToBaseline(t *testing.T) {
	t.Parallel()
	led := &fakeLedger{}
	o := advisoryOrchestrator(led, fixedNextStrategy{next: "made-up-phase"})
	ws := t.TempDir()

	next, _, _, _ := o.decideAfterRetroRouted(context.Background(), 5, CycleState{WorkspacePath: ws}, 1, VerdictFAIL, nil, router.RouteInput{})
	if next != PhaseEnd {
		t.Errorf("next = %s, want end (SM clamp must not upgrade proceed-to-end into a retry)", next)
	}
}
