package core

// agent_doc_missing_test.go — an OPTIONAL phase whose persona does not exist
// skips with a WARN; it must not kill the lane.
//
// soak-20260824a, cycle-1551: the advisor inserted defect-disposition-preflight
// (optional, on the SELECT menu), whose phase.json declares no agent and whose
// derived persona (agents/evolve-defect-disposition-preflight.md) exists
// nowhere. The load failed, the cycle died rc=4, the ADR-0072 halt stopped the
// whole batch — a full lane killed by an optional extra's missing file. Four
// catalog phases share the defect (two were on the menu).
//
// The remedy reuses optionalInfraSkip's guard rails wholesale: mandatory
// phases, ship-floor phases, and non-optional phases still fail LOUD — only a
// genuinely optional phase degrades, and the skip is recorded, never silent.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestOptionalPhaseSkip_AdmitsMissingAgentDoc(t *testing.T) {
	cat := catalogOf(t, specWith("defect-disposition-preflight", "evaluate", ""))
	o := &Orchestrator{catalog: cat}
	err := fmt.Errorf("defect-disposition-preflight: load agent: %w",
		fmt.Errorf("prompts: read agents/evolve-defect-disposition-preflight.md: %w", ErrAgentDocMissing))
	if !o.optionalInfraSkip(Phase("defect-disposition-preflight"), err) {
		t.Fatalf("an OPTIONAL phase with a missing persona must skip, not kill the lane")
	}
}

// The guard rails hold: the same error on a NON-optional phase stays fatal.
func TestOptionalPhaseSkip_MissingDocOnMandatoryPhaseStaysFatal(t *testing.T) {
	spec := specWith("build", "build", "")
	spec.Optional = false
	cat := catalogOf(t, spec)
	o := &Orchestrator{catalog: cat}
	err := fmt.Errorf("build: load agent: %w", ErrAgentDocMissing)
	if o.optionalInfraSkip(Phase("build"), err) {
		t.Fatalf("a non-optional phase must never skip on a missing persona — that would silently vanish a floor phase")
	}
}

// The ship floor holds even for a catalog-Optional phase: a floor member
// mis-marked Optional must not vanish on a missing persona — the skip may
// never weaken ship => build ∧ audit ∧ tdd.
func TestOptionalPhaseSkip_FloorPhaseNeverSkipsOnMissingDoc(t *testing.T) {
	o := amplNewSkipOrchestrator(t, nil, []string{"build", "audit", "tdd"}, optionalSpecFor("audit"))
	if o.optionalInfraSkip(Phase("audit"), ErrAgentDocMissing) {
		t.Fatalf("a ship-floor phase must never skip on a missing persona")
	}
}

// Configured-mandatory outranks catalog-Optional for the new class too — the
// generic cfg.Mandatory guard, not the floor loop, is what protects ship.
func TestOptionalPhaseSkip_ConfiguredMandatoryNeverSkipsOnMissingDoc(t *testing.T) {
	o := amplNewSkipOrchestrator(t, []string{"memo"}, nil, optionalSpecFor("memo"))
	if o.optionalInfraSkip(Phase("memo"), ErrAgentDocMissing) {
		t.Fatalf("a cfg.Mandatory phase must never skip on a missing persona")
	}
}

// Any other load error (unreadable file, permission) keeps today's behavior.
func TestOptionalPhaseSkip_OtherLoadErrorsUnchanged(t *testing.T) {
	cat := catalogOf(t, specWith("defect-disposition-preflight", "evaluate", ""))
	o := &Orchestrator{catalog: cat}
	if o.optionalInfraSkip(Phase("defect-disposition-preflight"), errors.New("load agent: permission denied")) {
		t.Fatalf("only the typed missing-doc sentinel may skip; arbitrary load errors stay fatal")
	}
}

// End-to-end (cycle-1551 replay): an advisor-scheduled OPTIONAL phase whose
// runner dies with the missing-persona sentinel must not abort the cycle —
// audit+ship still run — and the ledger files the skip under its OWN kind
// (optional_missing_persona_skip), never the infra key: zero retries and no
// infra event happened, and forensics must not merge the classes.
func TestOptionalPhaseMissingPersonaSkipsShipsAndLedgersOwnKind(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[Phase("amplify-tests")] = &fakeRunner{name: "amplify-tests",
		failErr: fmt.Errorf("amplify-tests: load agent: %w", ErrAgentDocMissing), failUntil: 99}
	auditR := runners[PhaseAudit].(*fakeRunner)
	shipR := runners[PhaseShip].(*fakeRunner)
	cat, err := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{{Name: "amplify-tests", Optional: true, After: "build"}})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	cfg := shadowCfg(config.StageAdvisory)
	cfg.Mode = config.ModeDynamicLLM
	cfg.Order = []string{"scout", "triage", "tdd", "build-planner", "build", "amplify-tests", "audit", "ship"}
	plan := &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true}, {Phase: "tdd", Run: true}, {Phase: "build", Run: true},
		{Phase: "amplify-tests", Run: true}, {Phase: "audit", Run: true}, {Phase: "ship", Run: true},
	}}
	o := NewOrchestrator(st, led, runners,
		WithRouting(cfg, router.StaticPreset{}), WithCatalog(cat), WithPlanner(&fixedPlanner{plan: plan}))
	if _, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "g", DisableWorkspaceGuard: true,
	}); err != nil {
		t.Fatalf("missing persona on an optional phase aborted the cycle: %v", err)
	}
	if auditR.calls == 0 || shipR.calls == 0 {
		t.Fatalf("audit(%d)/ship(%d) must still run after the persona skip", auditR.calls, shipR.calls)
	}
	found := ""
	for _, e := range led.entries {
		if e.Role == "amplify-tests" && strings.HasPrefix(e.Kind, "optional_") {
			found = e.Kind
		}
	}
	if found != "optional_missing_persona_skip" {
		t.Fatalf("ledger kind = %q, want optional_missing_persona_skip — the class split must reach the ledger, not just the helper", found)
	}
}
