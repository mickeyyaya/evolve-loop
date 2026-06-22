// orchestrator_optional_skip_test.go — RED contract for the cycle-283 killer:
// an advisor-scheduled OPTIONAL phase that exhausts its retries on an
// INFRA-shaped error (bridge artifact timeout exit=81 / transient bridge
// failure) aborted the whole cycle via wrapCycleLevelError, so audit and ship
// never ran and completed spine work was discarded unshipped. The intended
// behavior has been documented on ErrArtifactTimeout since Workstream D
// (errors.go: "an OPTIONAL phase that hits this degrades to WARN+advance
// instead of aborting the whole cycle") but was never implemented.
//
// The contract these tests pin:
//
//  1. Optional phase + infra-shaped exhaustion → synthesized WARN, cycle
//     ADVANCES; audit and ship still run (the operator policy: work that
//     would pass review must reach review; review-PASS must ship).
//  2. A MANDATORY/floor phase (build) with the same infra exhaustion stays
//     cycle-fatal — the skip must never weaken the integrity floor.
//  3. An optional phase failing with a NON-infra error stays cycle-fatal —
//     only infrastructure weather qualifies, never integrity or logic
//     failures (tree-diff guard aborts, gate failures, generic errors).
package core

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// optionalSkipHarness builds the advisory-routing orchestrator of the
// cycle-283 shape: spine runners all green, one catalog-Optional phase
// scheduled after build whose runner behavior the caller controls.
func optionalSkipHarness(t *testing.T, optRunner PhaseRunner) (*Orchestrator, *fakeRunner, *fakeRunner) {
	t.Helper()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[Phase("amplify-tests")] = optRunner
	auditR := runners[PhaseAudit].(*fakeRunner)
	shipR := runners[PhaseShip].(*fakeRunner)

	cat, err := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{
		{Name: "amplify-tests", Optional: true, After: "build"},
	})
	if err != nil {
		t.Fatalf("setup: catalog merge: %v", err)
	}

	cfg := shadowCfg(config.StageAdvisory)
	cfg.Mode = config.ModeDynamicLLM
	cfg.Order = []string{"scout", "triage", "tdd", "build-planner", "build",
		"amplify-tests", "audit", "ship"}

	plan := &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true}, {Phase: "tdd", Run: true},
		{Phase: "build", Run: true}, {Phase: "amplify-tests", Run: true},
		{Phase: "audit", Run: true}, {Phase: "ship", Run: true},
	}}

	o := NewOrchestrator(st, led, runners,
		WithRouting(cfg, router.StaticPreset{}),
		WithCatalog(cat),
		WithPlanner(&fixedPlanner{plan: plan}))
	return o, auditR, shipR
}

func infraTimeoutErr(name string) error {
	return fmt.Errorf("%s: bridge: bridge: launch exit=81: %w", name, ErrArtifactTimeout)
}

// TestOptionalPhaseInfraTimeoutSkipsAndCycleShips: the cycle-283 replay.
// amplify-tests (catalog-Optional, advisor-scheduled) times out on every
// attempt; the cycle must degrade it to WARN and still run audit + ship. RED
// today: wrapCycleLevelError aborts the cycle without consulting optionality,
// so audit/ship never run.
func TestOptionalPhaseInfraTimeoutSkipsAndCycleShips(t *testing.T) {
	t.Parallel()
	opt := &fakeRunner{name: "amplify-tests",
		failErr: infraTimeoutErr("amplify-tests"), failUntil: 99}
	o, auditR, shipR := optionalSkipHarness(t, opt)

	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot:           t.TempDir(),
		GoalHash:              "g",
		DisableWorkspaceGuard: true,
	})
	if err != nil {
		var clf *ErrCycleLevelFailure
		if errors.As(err, &clf) {
			t.Fatalf("RED: optional phase %q infra-timeout aborted the cycle (%v) — audit/ship never ran; "+
				"an optional phase exhausting infra retries must degrade to WARN+advance (errors.go Workstream-D intent, cycle-283)", clf.Phase, err)
		}
		t.Fatalf("RunCycle: %v", err)
	}
	if auditR.calls == 0 {
		t.Error("audit never ran after optional-phase infra skip — review must always be reached")
	}
	if shipR.calls == 0 {
		t.Error("ship never ran after optional-phase infra skip — passed review must ship")
	}
	ranOpt := false
	for _, p := range res.PhasesRun {
		if p == Phase("amplify-tests") {
			ranOpt = true
		}
	}
	if !ranOpt {
		t.Error("amplify-tests missing from PhasesRun — the skip records the phase with its synthesized WARN " +
			"outcome (the infra failure itself lives in failure-learning and the optional_infra_skip ledger entry)")
	}
}

// TestMandatoryPhaseInfraTimeoutStaysCycleFatal: the floor guard. The same
// infra exhaustion on build (mandatory, ship-floor) must still abort the
// cycle — the optional skip must never weaken the integrity floor.
func TestMandatoryPhaseInfraTimeoutStaysCycleFatal(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseBuild] = &fakeRunner{name: string(PhaseBuild),
		failErr: infraTimeoutErr("build"), failUntil: 99}
	o := NewOrchestrator(st, led, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot:           t.TempDir(),
		GoalHash:              "g",
		DisableWorkspaceGuard: true,
	})
	var clf *ErrCycleLevelFailure
	if !errors.As(err, &clf) {
		t.Fatalf("build infra-timeout must stay cycle-fatal, got err=%v", err)
	}
	if clf.Phase != string(PhaseBuild) {
		t.Errorf("cycle-level failure phase=%q, want %q", clf.Phase, PhaseBuild)
	}
}

// TestOptionalPhaseNonInfraErrorStaysCycleFatal: the qualifier guard. An
// optional phase failing with a NON-infra error (generic logic/integrity
// failure) must still abort — only infrastructure weather may be skipped.
func TestOptionalPhaseNonInfraErrorStaysCycleFatal(t *testing.T) {
	t.Parallel()
	opt := &fakeRunner{name: "amplify-tests",
		failErr: errors.New("amplify-tests: deliverable forged challenge token"), failUntil: 99}
	o, _, _ := optionalSkipHarness(t, opt)

	_, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot:           t.TempDir(),
		GoalHash:              "g",
		DisableWorkspaceGuard: true,
	})
	var clf *ErrCycleLevelFailure
	if !errors.As(err, &clf) {
		t.Fatalf("optional phase NON-infra failure must stay cycle-fatal (only infra-shaped errors qualify for the skip), got err=%v", err)
	}
	if clf.Phase != "amplify-tests" {
		t.Errorf("cycle-level failure phase=%q, want amplify-tests", clf.Phase)
	}
}

// TestMandatoryPhaseNeverInfraSkipped pins the generalized infra-skip guard:
// a configured-mandatory phase is NEVER infra-skipped, even if its catalog spec
// is (mis)marked Optional. This generalizes the former ship-only special case —
// the engine consults the mandatory SET (config), not a hardcoded phase name
// (phase-agnostic flow, ADR-0035/0038). RED until optionalInfraSkip reads
// isConfiguredMandatory: the old `if p == PhaseShip` guard protects only ship,
// so a mandatory-but-optional non-ship phase is wrongly skip-eligible.
func TestMandatoryPhaseNeverInfraSkipped(t *testing.T) {
	t.Parallel()
	cat, err := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{
		{Name: "amplify-tests", Optional: true, After: "build"},
	})
	if err != nil {
		t.Fatalf("setup: catalog merge: %v", err)
	}
	cfg := config.RoutingConfig{Mandatory: []string{"amplify-tests"}}
	o := NewOrchestrator(nil, nil, nil, WithCatalog(cat), WithRouting(cfg, nil))

	if o.optionalInfraSkip(Phase("amplify-tests"), infraTimeoutErr("amplify-tests")) {
		t.Error("a configured-mandatory phase must never be infra-skipped, even when its spec is Optional; " +
			"the guard must read the mandatory set (config), not a hardcoded ship check")
	}
}

// TestShipNeverInfraSkipped is the byte-identity characterization for the
// former ship special-case: ship (a mandatory anchor) must never be infra-
// skipped — it is the ship gate itself. Green before AND after the
// generalization (old: hardcoded ship check; new: ship ∈ mandatory).
func TestShipNeverInfraSkipped(t *testing.T) {
	t.Parallel()
	cfg := config.RoutingConfig{Mandatory: []string{"scout", "build", "audit", "ship"}}
	o := NewOrchestrator(nil, nil, nil, WithRouting(cfg, nil))

	if o.optionalInfraSkip(PhaseShip, infraTimeoutErr("ship")) {
		t.Error("ship must never be infra-skipped (it is the ship gate itself)")
	}
}
