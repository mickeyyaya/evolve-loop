package routingtest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

type recordingTBF4 struct {
	testing.TB
	errors []string
	fatals []string
}

type fatalSentinelF4 struct{}

func (r *recordingTBF4) Helper() {}

func (r *recordingTBF4) Errorf(format string, args ...any) {
	r.errors = append(r.errors, sprintfF4(format, args...))
}

func (r *recordingTBF4) Fatalf(format string, args ...any) {
	r.fatals = append(r.fatals, sprintfF4(format, args...))
	panic(fatalSentinelF4{})
}

func sprintfF4(format string, args ...any) string {
	return strings.TrimSpace(strings.NewReplacer("\n", " ").Replace(fmt.Sprintf(format, args...)))
}

func runExpectationsF4(t *testing.T, s ScenarioSpec, res core.CycleResult, decisions []router.RouterDecision, led *fixtures.FakeLedger, agent *scriptedProposer) (rec *recordingTBF4) {
	t.Helper()
	rec = &recordingTBF4{TB: t}
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(fatalSentinelF4); !ok {
				panic(r)
			}
		}
	}()
	assertCycleExpectations(rec, s, res, decisions, led, agent)
	return rec
}

func TestAssertCycleExpectations_ReportsEachViolatedExpectation(t *testing.T) {
	res := core.CycleResult{PhasesRun: []core.Phase{core.PhaseScout, core.PhaseBuild}, RetroDecision: "retro: accept"}
	decisions := []router.RouterDecision{{InsertPhases: []string{"tester"}, Clamps: []router.Clamp{{Rule: "floor"}}}}
	led := &fixtures.FakeLedger{Entries: []core.LedgerEntry{{Kind: "routing_decision"}, {Kind: "other"}}}
	agent := &scriptedProposer{seen: []string{"scout"}}

	cases := []struct {
		name    string
		expect  ExpectSpec
		agent   *scriptedProposer
		wantErr string
	}{
		{"a phase in the wrong position is reported with the whole run", ExpectSpec{PhaseSequence: []core.Phase{core.PhaseScout, core.PhaseAudit}}, agent, "phase[1]=build, want audit"},
		{"phase sequence length mismatch is fatal", ExpectSpec{PhaseSequence: []core.Phase{core.PhaseScout}}, agent, "PhasesRun="},
		{"a phase that must be absent ran", ExpectSpec{PhasesAbsent: []core.Phase{core.PhaseBuild}}, agent, "phase build ran but should be absent"},
		{"expected insert never proposed", ExpectSpec{DecisionInserts: []string{"auditor"}}, agent, `no routing decision proposed inserting "auditor"`},
		{"expected clamp never carried", ExpectSpec{DecisionClamps: []string{"nope"}}, agent, `no routing decision carried clamp "nope"`},
		{"too few routing ledger entries", ExpectSpec{RoutingLedgerMin: 2}, agent, "routing_decision ledger entries=1, want >=2"},
		{"retro marker absent", ExpectSpec{RetroPrefix: "retro: reject"}, agent, `RetroDecision="retro: accept", want marker "retro: reject"`},
		{"proposer cadence differs", ExpectSpec{ProposeAt: []string{"scout", "build"}}, agent, "Proposer invoked at [scout], want exactly [scout build]"},
		{"propose-at without an agent is fatal", ExpectSpec{ProposeAt: []string{"scout"}}, nil, "ProposeAt expects a scripted agent"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := runExpectationsF4(t, ScenarioSpec{Expect: tc.expect}, res, decisions, led, tc.agent)
			all := strings.Join(append(append([]string{}, rec.errors...), rec.fatals...), "\n")
			if !strings.Contains(all, tc.wantErr) {
				t.Fatalf("recorded %q, want it to contain %q", all, tc.wantErr)
			}
		})
	}
}

func TestAssertCycleExpectations_SatisfiedExpectationsRecordNothing(t *testing.T) {
	res := core.CycleResult{PhasesRun: []core.Phase{core.PhaseScout, core.PhaseBuild}, RetroDecision: "retro: accept (extra)"}
	decisions := []router.RouterDecision{{InsertPhases: []string{"tester"}, Clamps: []router.Clamp{{Rule: "floor"}}}}
	led := &fixtures.FakeLedger{Entries: []core.LedgerEntry{{Kind: "routing_decision"}}}
	agent := &scriptedProposer{seen: []string{"build", "scout"}}
	spec := ScenarioSpec{Expect: ExpectSpec{
		PhaseSequence:    []core.Phase{core.PhaseScout, core.PhaseBuild},
		PhasesAbsent:     []core.Phase{core.PhaseShip},
		DecisionInserts:  []string{"tester"},
		DecisionClamps:   []string{"floor"},
		RoutingLedgerMin: 1,
		RetroPrefix:      "retro: accept",
		ProposeAt:        []string{"scout", "build"},
	}}

	rec := runExpectationsF4(t, spec, res, decisions, led, agent)

	if len(rec.errors)+len(rec.fatals) != 0 {
		t.Fatalf("errors=%v fatals=%v", rec.errors, rec.fatals)
	}
}

func TestPhaseEnablesOf_MapsOnAndOffAndDropsUnsetValues(t *testing.T) {
	got := phaseEnablesOf(map[string]config.Enable{
		"triage": config.EnableOff,
		"intent": config.EnableOn,
		"tdd":    config.EnableContent,
	})
	want := map[string]string{"triage": "off", "intent": "on"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("phaseEnablesOf = %v, want %v", got, want)
	}
	if empty := phaseEnablesOf(nil); empty == nil || len(empty) != 0 {
		t.Fatalf("nil input must yield an empty non-nil map, got %#v", empty)
	}
}

func TestRun_FullOrchestratorForwardsAScriptedPlanSoItDrivesTheSpine(t *testing.T) {
	Run(t, Scenario("cycle/plan drives the spine",
		Cycle(), Advisory(), Mandatory("tdd"), MediumCycle(),
		AgentPlan(PlanRun("scout"), PlanRun("tdd"), PlanRun("build"), PlanRun("audit"), PlanRun("ship")),
		ExpectPhases(core.PhaseScout, core.PhaseTDD, core.PhaseBuild, core.PhaseAudit, core.PhaseShip),
		ExpectAbsent(core.PhaseTriage, core.PhaseBuildPlanner),
	))
}

func TestRun_FullOrchestratorMirrorsTheSpecEnablesIntoTheWorkflowConfig(t *testing.T) {
	Run(t, Scenario("cycle/intent forced on",
		Cycle(), Off(), IntentRequired(),
		ExpectPhases(core.PhaseIntent, core.PhaseScout, core.PhaseTriage, core.PhaseTDD, core.PhaseBuildPlanner, core.PhaseBuild, core.PhaseAudit, core.PhaseShip),
	))
}

func TestRun_FullOrchestratorHandsTheCycleACopyOfTheSpecEnv(t *testing.T) {
	var gotEnv map[string]string
	orig := routingtestRunCycle
	t.Cleanup(func() { routingtestRunCycle = orig })
	routingtestRunCycle = func(o *core.Orchestrator, req core.CycleRequest) (core.CycleResult, error) {
		gotEnv = req.Env
		return orig(o, req)
	}
	spec := Scenario("cycle/env copy", Cycle(), Advisory(), AgentPlanError(), PhaseVerdict(core.PhaseScout, "PASS"))
	spec.Env = map[string]string{"EVOLVE_SCENARIO_MARKER": "1"}

	Run(t, spec)

	if gotEnv["EVOLVE_SCENARIO_MARKER"] != "1" {
		t.Fatalf("env not forwarded: %v", gotEnv)
	}
	gotEnv["EVOLVE_SCENARIO_MARKER"] = "mutated"
	if spec.Env["EVOLVE_SCENARIO_MARKER"] != "1" {
		t.Errorf("the cycle must receive a copy: the spec env was mutated through it")
	}
}

func TestRunCycle_OrchestratorErrorFailsTheScenarioWithTheCause(t *testing.T) {
	orig := routingtestRunCycle
	t.Cleanup(func() { routingtestRunCycle = orig })
	routingtestRunCycle = func(*core.Orchestrator, core.CycleRequest) (core.CycleResult, error) {
		return core.CycleResult{}, errors.New("lease held")
	}

	var ran, failed bool
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	realStdout := os.Stdout
	os.Stdout = devNull
	ok := testing.RunTests(func(string, string) (bool, error) { return true, nil }, []testing.InternalTest{{
		Name: "TestInnerRunCycleErrorF4",
		F: func(inner *testing.T) {
			ran = true
			defer func() { failed = inner.Failed() }()
			runCycle(inner, Scenario("x", Cycle()))
			t.Errorf("runCycle must stop at the orchestrator error")
		},
	}})

	os.Stdout = realStdout
	devNull.Close()
	if !ran || ok || !failed {
		t.Fatalf("ran=%v ok=%v failed=%v: an orchestrator error must fail the scenario", ran, ok, failed)
	}
}

func TestSignalSpec_HandoffFilesCarryDeliverableAndGoalKinds(t *testing.T) {
	spec := SignalSpec{
		CycleSize:             "small",
		TriageSize:            "small",
		GoalType:              "research",
		DeliverableKind:       "document",
		TriageDeliverableKind: "code",
	}

	files := spec.HandoffFiles()

	var scout, triage map[string]any
	if err := json.Unmarshal([]byte(files["handoff-scout.json"]), &scout); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(files["handoff-triage.json"]), &triage); err != nil {
		t.Fatal(err)
	}
	if scout["goal_type"] != "research" || scout["deliverable_kind"] != "document" {
		t.Errorf("scout handoff = %v", scout)
	}
	if triage["deliverable_kind"] != "code" || triage["cycle_size"] != "small" {
		t.Errorf("triage handoff = %v", triage)
	}

	bare := SignalSpec{CycleSize: "small", TriageSize: "small"}.HandoffFiles()
	if strings.Contains(bare["handoff-scout.json"], "goal_type") || strings.Contains(bare["handoff-scout.json"], "deliverable_kind") ||
		strings.Contains(bare["handoff-triage.json"], "deliverable_kind") {
		t.Errorf("unset kinds must not be emitted: %v", bare)
	}
}
