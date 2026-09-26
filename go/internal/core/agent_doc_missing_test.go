package core

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
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

func TestOptionalPhaseSkip_FloorPhaseNeverSkipsOnMissingDoc(t *testing.T) {
	o := amplNewSkipOrchestrator(t, nil, []string{"build", "audit", "tdd"}, optionalSpecFor("audit"))
	if o.optionalInfraSkip(Phase("audit"), ErrAgentDocMissing) {
		t.Fatalf("a ship-floor phase must never skip on a missing persona")
	}
}

func TestOptionalPhaseSkip_ConfiguredMandatoryNeverSkipsOnMissingDoc(t *testing.T) {
	o := amplNewSkipOrchestrator(t, []string{"memo"}, nil, optionalSpecFor("memo"))
	if o.optionalInfraSkip(Phase("memo"), ErrAgentDocMissing) {
		t.Fatalf("a cfg.Mandatory phase must never skip on a missing persona")
	}
}

func TestOptionalPhaseSkip_OtherLoadErrorsUnchanged(t *testing.T) {
	cat := catalogOf(t, specWith("defect-disposition-preflight", "evaluate", ""))
	o := &Orchestrator{catalog: cat}
	if o.optionalInfraSkip(Phase("defect-disposition-preflight"), errors.New("load agent: permission denied")) {
		t.Fatalf("only the typed missing-doc sentinel may skip; arbitrary load errors stay fatal")
	}
}

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

func TestOptionalPhaseMissingPersona_LearnsDeterministicallyWithoutRetroAgent(t *testing.T) {
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[Phase("amplify-tests")] = &fakeRunner{name: "amplify-tests",
		failErr: fmt.Errorf("amplify-tests: load agent: %w", ErrAgentDocMissing), failUntil: 99}
	retroR := runners[PhaseRetro].(*fakeRunner)
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
		ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true,
	}); err != nil {
		t.Fatalf("missing persona on an optional phase aborted the cycle: %v", err)
	}
	if retroR.calls != 0 {
		t.Fatalf("retro agent dispatched %d time(s) for a known persona absence — deterministic learning only", retroR.calls)
	}
	lessons, gerr := filepath.Glob(filepath.Join(root, ".evolve", "instincts", "lessons", "cycle-*-phase-*.yaml"))
	if gerr != nil || len(lessons) != 1 {
		t.Fatalf("want exactly one deterministic failure lesson, got %v (err=%v)", lessons, gerr)
	}
	todo := false
	for _, td := range st.state.CarryoverTodos {
		if strings.Contains(td.Action, "amplify-tests") {
			todo = true
		}
	}
	if !todo {
		t.Fatalf("the skip must still queue its carryover todo; got %+v", st.state.CarryoverTodos)
	}
}
