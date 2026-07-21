package core

// decision_branch_floor_test.go — cycle-1002 RED contract for ADR-0072 S4
// Task 3 (wire-floor-override-consumption). decideAfterRetro and
// decideAfterRetroRouted gain a 4th return value (*cyclestate.SystemFailureSignal)
// and consume failure-decision.json under a Go-floor override:
//
//   - a floor category (verdict-incoherence / infra-systemic) HALTS even when
//     the orchestrator decision (or the router) proposes a retry — the override
//     must bite in the LIVE routed path, ABOVE the router (F1);
//   - the cycle-1001 audit-declared system class halts both deterministically
//     (dossier candidate, dec absent) and via judgment (dec says halt);
//   - with no artifact and no floor, the branch/env/reason fall back
//     BYTE-IDENTICAL to failureadapter.Decide (R4 regression guard).
//
// These fail RED until Builder adds the 4th return + applyFailureDecisionFloor.

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/failureadapter"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func floorOrchestrator(strat router.RoutingStrategy) *Orchestrator {
	return &Orchestrator{
		ledger:        &fakeLedger{},
		now:           coverNow,
		strategy:      strat,
		cfg:           config.RoutingConfig{Stage: config.StageAdvisory},
		sm:            NewStateMachine(),
		failurePolicy: policy.DefaultSystemFailurePolicy(),
	}
}

// F1 / R5 / AC3 — the LIVE path. The router proposes a `tdd` retry, and
// failure-decision.json also says "retry" — but the cycle is verdict-incoherent
// (green artifacts contradict the recorded FAIL), so the Go floor OVERRIDES both
// to a halt. The override must sit above o.strategy.Decide, or a routed retry
// survives (the exact defect the premise-challenge flagged).
func TestDecideAfterRetroFloor_RoutedRetryOverriddenToHalt(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
	dir := t.TempDir()
	writeVerdicts(t, dir, "PASS", "PASS") // green → recorded FAIL is incoherent
	writeDecision(t, dir, `{"category":"verdict-incoherence","level":"system","action":"retry-with-fix","fix_type":"pipeline-repair"}`)
	cs := CycleState{CycleID: 1002, WorkspacePath: dir}

	next, _, _, sig := o.decideAfterRetroRouted(context.Background(), 1002, cs, 1, VerdictFAIL, nil, router.RouteInput{})

	if next != PhaseEnd {
		t.Errorf("next = %s, want end (floor overrides the routed/orchestrator retry)", next)
	}
	if sig == nil {
		t.Fatal("a floor category must produce a SystemFailure signal")
	}
	if !sig.Halt {
		t.Error("verdict-incoherence is a floor category → Halt must be true")
	}
	if sig.Category != policy.CategoryVerdictIncoherence {
		t.Errorf("sig.Category = %q, want verdict-incoherence", sig.Category)
	}
}

// F2 / R6-a — the cycle-1001 shape caught DETERMINISTICALLY (orchestrator
// absent). The audit self-declared a structured system class; the dossier
// candidate is infra-systemic; with no failure-decision.json the Go floor still
// halts.
func TestDecideAfterRetroFloor_Cycle1001DeterministicHalt(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
	dir := t.TempDir()
	writeAuditWithFailure(t, dir, "FAIL", "infra-systemic",
		"all CLI families exhausted; systemic infrastructure teardown")
	cs := CycleState{CycleID: 1001, WorkspacePath: dir}

	next, _, _, sig := o.decideAfterRetroRouted(context.Background(), 1001, cs, 1, VerdictFAIL, nil, router.RouteInput{})

	if next != PhaseEnd {
		t.Errorf("next = %s, want end (audit-declared system class halts deterministically)", next)
	}
	if sig == nil || !sig.Halt {
		t.Fatalf("dossier infra-systemic candidate must halt even with dec==nil; sig=%v", sig)
	}
	if sig.Category != policy.CategoryInfraSystemic {
		t.Errorf("sig.Category = %q, want infra-systemic", sig.Category)
	}
}

// F2 / R6-b — the cycle-1001 shape caught via JUDGMENT. The audit's structured
// class is task-level (code-audit-fail) so the deterministic dossier candidate
// is empty, but the orchestrator classified it infra-systemic in
// failure-decision.json → the floor halts on the orchestrator's own category.
func TestDecideAfterRetroFloor_Cycle1001JudgmentHalt(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
	dir := t.TempDir()
	writeAuditWithFailure(t, dir, "FAIL", "code-audit-fail",
		"SYSTEM-class shared-state lost write: state.json carryoverTodos clobbered")
	writeDecision(t, dir, `{"category":"infra-systemic","level":"system","evidence":"prose-declared SYSTEM-class lost write","action":"halt-and-diagnose","fix_type":"pipeline-repair"}`)
	cs := CycleState{CycleID: 1001, WorkspacePath: dir}

	next, _, _, sig := o.decideAfterRetroRouted(context.Background(), 1001, cs, 1, VerdictFAIL, nil, router.RouteInput{})

	if next != PhaseEnd {
		t.Errorf("next = %s, want end (orchestrator-classified floor category halts)", next)
	}
	if sig == nil || !sig.Halt || sig.Category != policy.CategoryInfraSystemic {
		t.Fatalf("orchestrator infra-systemic classification must halt; sig=%v", sig)
	}
}

// R4 / AC5 — REGRESSION GUARD. With no failure-decision.json and no floor
// candidate, the retro branch returns a nil signal and its branch/reason are
// BYTE-IDENTICAL to the pre-S4 deterministic failureadapter output. Driven
// through the live routed path with a StaticPreset advisor (which agrees with
// the deterministic branch, so the operator-facing contract string is
// preserved) over an EMPTY workspace (no artifacts → no floor, no decision).
func TestDecideAfterRetroFloor_FallbackByteIdentical(t *testing.T) {
	o := floorOrchestrator(router.StaticPreset{})
	cs := CycleState{CycleID: 1002, WorkspacePath: t.TempDir()}

	// The pre-S4 deterministic expectation for empty history.
	want := failureadapter.Decide(nil, failureadapter.Options{Now: coverNow()})
	wantReason := "proceed: " + want.Reason

	next, _, reason, sig := o.decideAfterRetroRouted(context.Background(), 1002, cs, 1, VerdictFAIL, nil, router.RouteInput{})

	if sig != nil {
		t.Errorf("no floor + no decision must yield a nil signal, got %+v", sig)
	}
	if next != PhaseEnd {
		t.Errorf("next = %s, want end (deterministic PROCEED on empty history)", next)
	}
	if reason != wantReason {
		t.Errorf("reason = %q, want byte-identical %q", reason, wantReason)
	}
}
