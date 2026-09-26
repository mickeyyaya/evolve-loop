package core

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

func TestDecideAfterRetroFloor_FallbackByteIdentical(t *testing.T) {
	o := floorOrchestrator(router.StaticPreset{})
	cs := CycleState{CycleID: 1002, WorkspacePath: t.TempDir()}

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

func TestDecideAfterRetroFloor_Cycle1603DiagnosedDowngradeIsNotForgery(t *testing.T) {
	o := floorOrchestrator(router.StaticPreset{})
	dir := t.TempDir()
	writeVerdicts(t, dir, "PASS", "PASS") // green → contradicts the prose claim
	writeDecision(t, dir, `{"category":"verdict-incoherence","level":"system","evidence":"audit-report.md declares PASS while acs-verdict.json has ship_eligible false","action":"halt-and-diagnose","fix_type":"pipeline-repair"}`)
	cs := CycleState{CycleID: 1603, WorkspacePath: dir, AuditFailReasons: []string{
		"EGPS: acs-verdict.json ship_eligible=false — the authoritative acssuite SSOT rejects the ship even though red_count==0; a narrative PASS cannot override it",
	}}

	_, _, _, sig := o.decideAfterRetroRouted(context.Background(), 1603, cs, 1, VerdictFAIL, nil, router.RouteInput{})

	if sig != nil {
		t.Fatalf("a diagnosed EGPS downgrade must not halt as forged-verdict on prose alone; sig=%+v", sig)
	}
}

func TestDecideAfterRetroFloor_ProseIncoherenceWithoutDiagnosedReasonsStillHalts(t *testing.T) {
	o := floorOrchestrator(router.StaticPreset{})
	dir := t.TempDir()
	writeVerdicts(t, dir, "FAIL", "PASS") // red report → deterministic detector silent
	writeDecision(t, dir, `{"category":"verdict-incoherence","level":"system","evidence":"recorded verdict contradicts artifacts","action":"halt-and-diagnose","fix_type":"pipeline-repair"}`)
	cs := CycleState{CycleID: 1603, WorkspacePath: dir}

	next, _, _, sig := o.decideAfterRetroRouted(context.Background(), 1603, cs, 1, VerdictFAIL, nil, router.RouteInput{})

	if next != PhaseEnd {
		t.Errorf("next = %s, want end (uncontradicted prose floor classification halts)", next)
	}
	if sig == nil || !sig.Halt || sig.Category != policy.CategoryVerdictIncoherence {
		t.Fatalf("uncontradicted prose verdict-incoherence must still halt; sig=%+v", sig)
	}
}

func TestDecideAfterRetroFloor_DiagnosedShipReasonsAlsoContradictProse(t *testing.T) {
	o := floorOrchestrator(router.StaticPreset{})
	dir := t.TempDir()
	writeVerdicts(t, dir, "PASS", "PASS")
	writeDecision(t, dir, `{"category":"verdict-incoherence","level":"system","evidence":"claimed","action":"halt-and-diagnose","fix_type":"pipeline-repair"}`)
	cs := CycleState{CycleID: 1603, WorkspacePath: dir, ShipFailReasons: []string{"ship gate: repo-contract scanner rejected the tree"}}

	_, _, _, sig := o.decideAfterRetroRouted(context.Background(), 1603, cs, 1, VerdictFAIL, nil, router.RouteInput{})

	if sig != nil {
		t.Fatalf("diagnosed ship-floor reasons must contradict a prose forgery claim; sig=%+v", sig)
	}
}
