package core

import "testing"

type oracleCell struct {
	next    Phase
	errText string
}

var nextGoldenLinear = map[Phase]oracleCell{
	PhaseStart:        {PhaseScout, ""},
	PhaseIntent:       {PhaseScout, ""},
	PhaseScout:        {PhaseTriage, ""},
	PhaseTriage:       {PhaseTDD, ""},
	PhaseTDD:          {PhaseBuildPlanner, ""},
	PhaseBuildPlanner: {PhaseBuild, ""},
	PhaseBuild:        {PhaseAudit, ""},
	PhaseShip:         {PhaseEnd, ""},
	PhaseRetro:        {PhaseEnd, ""},
	PhaseDebugger:     {PhaseEnd, ""},
	PhaseSwarmPlan:    {"", "core: invalid phase transition: no successor for swarm-plan"},
	PhaseEnd:          {"", "core: invalid phase transition: end is terminal"},
}

var nextGoldenAudit = map[string]oracleCell{
	VerdictPASS: {PhaseShip, ""},
	VerdictWARN: {PhaseShip, ""},
	VerdictFAIL: {PhaseRetro, ""},
	"":          {"", `core: invalid phase transition: audit verdict ""`},
	"garbage":   {"", `core: invalid phase transition: audit verdict "garbage"`},
}

var oracleVerdicts = []string{"", VerdictPASS, VerdictWARN, VerdictFAIL, "garbage"}

var allOraclePhases = []Phase{
	PhaseStart, PhaseIntent, PhaseScout, PhaseTriage, PhaseTDD, PhaseBuildPlanner,
	PhaseSwarmPlan, PhaseBuild, PhaseAudit, PhaseShip, PhaseRetro, PhaseDebugger, PhaseEnd,
}

var legalityGolden = map[Phase][]Phase{
	PhaseStart:        {PhaseIntent, PhaseScout},
	PhaseIntent:       {PhaseScout},
	PhaseScout:        {PhaseTriage, PhaseTDD, PhaseBuild, PhaseEnd},
	PhaseTriage:       {PhaseTDD, PhaseBuild, PhaseEnd},
	PhaseTDD:          {PhaseBuildPlanner, PhaseBuild},
	PhaseBuildPlanner: {PhaseBuild},
	PhaseBuild:        {PhaseAudit},
	PhaseAudit:        {PhaseShip, PhaseRetro, PhaseTDD, PhaseBuild},
	PhaseRetro:        {PhaseShip, PhaseTDD, PhaseEnd, PhaseAudit},
	PhaseShip:         {PhaseEnd, PhaseDebugger, PhaseAudit, PhaseBuild, PhaseTDD, PhaseShip},
	PhaseDebugger:     {PhaseShip, PhaseAudit, PhaseBuild, PhaseTDD, PhaseEnd},
	PhaseEnd:          {},
}

func assertNextCell(t *testing.T, p Phase, v string, want oracleCell) {
	t.Helper()
	got, err := NewStateMachine().Next(p, v)
	if got != want.next {
		t.Errorf("Next(%q,%q) phase = %q, want %q", p, v, got, want.next)
	}
	gotErr := ""
	if err != nil {
		gotErr = err.Error()
	}
	if gotErr != want.errText {
		t.Errorf("Next(%q,%q) err = %q, want %q", p, v, gotErr, want.errText)
	}
}

func TestTransitionKernelOracle_Next(t *testing.T) {
	t.Parallel()
	for p, want := range nextGoldenLinear {
		for _, v := range oracleVerdicts {
			assertNextCell(t, p, v, want)
		}
	}
	for v, want := range nextGoldenAudit {
		assertNextCell(t, PhaseAudit, v, want)
	}
	for _, v := range oracleVerdicts {
		assertNextCell(t, Phase("bogus"), v, oracleCell{"", "core: invalid phase: bogus"})
	}
}

func TestTransitionKernelOracle_Legality(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine()
	for _, from := range allOraclePhases {
		allowed := make(map[Phase]bool, len(legalityGolden[from]))
		for _, to := range legalityGolden[from] {
			allowed[to] = true
		}
		for _, to := range allOraclePhases {
			if got := sm.CanTransition(from, to); got != allowed[to] {
				t.Errorf("CanTransition(%q,%q) = %v, want %v", from, to, got, allowed[to])
			}
		}
	}
}
