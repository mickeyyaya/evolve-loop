package core

import "testing"

// transition_oracle_test.go — the byte-identity ORACLE for the transition kernel
// (ADR-0058). It FREEZES the current behavior of NewStateMachine().Next and
// CanTransition, captured from HEAD (main abf787ff, 2026-06-21), so every
// subsequent slice that makes the kernel config-driven can prove it changed
// NOTHING observable. This is the trust anchor for the phase-agnostic-flow
// refactor: it MUST stay byte-identical green across S0..S7. A diff here is a
// behavior change in the integrity-floor transition logic and must be
// deliberate, reviewed, and re-frozen — never silently.

// oracleCell is one frozen Next() outcome: the successor phase and the exact
// error string ("" means nil error).
type oracleCell struct {
	next    Phase
	errText string
}

// nextGoldenLinear freezes the Next() successor for every phase whose successor
// is verdict-INDEPENDENT (the linear spine + the recovery/terminal sentinels).
var nextGoldenLinear = map[Phase]oracleCell{
	PhaseStart:        {PhaseScout, ""},
	PhaseIntent:       {PhaseScout, ""},
	PhaseScout:        {PhaseTriage, ""},
	PhaseTriage:       {PhaseTDD, ""},
	PhaseTDD:          {PhaseBuildPlanner, ""},
	PhaseBuildPlanner: {PhaseBuild, ""},
	PhaseBuild:        {PhaseAudit, ""},
	PhaseShip:         {PhaseEnd, ""},
	PhaseRetro:        {PhaseEnd, ""}, // sentinel; orchestrator overrides via decideAfterRetro
	PhaseDebugger:     {PhaseEnd, ""}, // sentinel; orchestrator overrides via decideAfterDebugger
	PhaseSwarmPlan:    {"", "core: invalid phase transition: no successor for swarm-plan"},
	PhaseEnd:          {"", "core: invalid phase transition: end is terminal"},
}

// nextGoldenAudit freezes audit's verdict-DEPENDENT successors — the only real
// verdict branch in the kernel today.
var nextGoldenAudit = map[string]oracleCell{
	VerdictPASS: {PhaseShip, ""},
	VerdictWARN: {PhaseShip, ""},
	VerdictFAIL: {PhaseRetro, ""},
	"":          {"", `core: invalid phase transition: audit verdict ""`},
	"garbage":   {"", `core: invalid phase transition: audit verdict "garbage"`},
}

// oracleVerdicts is the verdict cross-product the oracle sweeps for every phase.
var oracleVerdicts = []string{"", VerdictPASS, VerdictWARN, VerdictFAIL, "garbage"}

// allOraclePhases is every VALID phase the legality matrix sweeps.
var allOraclePhases = []Phase{
	PhaseStart, PhaseIntent, PhaseScout, PhaseTriage, PhaseTDD, PhaseBuildPlanner,
	PhaseSwarmPlan, PhaseBuild, PhaseAudit, PhaseShip, PhaseRetro, PhaseDebugger, PhaseEnd,
}

// legalityGolden freezes the legality graph: legalityGolden[from] is the exact
// set of `to` phases for which CanTransition(from,to) is true at HEAD. This
// graph is a config-INDEPENDENT trust anchor (ADR-0058 §1) and must never drift.
var legalityGolden = map[Phase][]Phase{
	PhaseStart:        {PhaseIntent, PhaseScout},
	PhaseIntent:       {PhaseScout},
	PhaseScout:        {PhaseTriage, PhaseTDD, PhaseBuild, PhaseEnd},
	PhaseTriage:       {PhaseTDD, PhaseBuild, PhaseEnd},
	PhaseTDD:          {PhaseBuildPlanner, PhaseBuild},
	PhaseBuildPlanner: {PhaseBuild},
	PhaseBuild:        {PhaseAudit},
	PhaseAudit:        {PhaseShip, PhaseRetro},
	PhaseRetro:        {PhaseShip, PhaseTDD, PhaseEnd},
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

// TestTransitionKernelOracle_Next freezes every Next(phase,verdict) outcome
// (successor phase + verbatim error string) over the full cross product.
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
	// An invalid phase is ErrPhaseInvalid, verdict-independent.
	for _, v := range oracleVerdicts {
		assertNextCell(t, Phase("bogus"), v, oracleCell{"", "core: invalid phase: bogus"})
	}
}

// TestTransitionKernelOracle_Legality freezes CanTransition over the full
// Phase×Phase matrix — proves the legality graph stays byte-untouched.
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
