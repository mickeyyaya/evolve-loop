package cyclestate

// Phase is the typed identity of an orchestrator lifecycle stage.
// Stringly-backed for JSON portability.
type Phase string

const (
	PhaseStart        Phase = "start"
	PhaseIntent       Phase = "intent"
	PhaseScout        Phase = "scout"
	PhaseTriage       Phase = "triage"
	PhaseTDD          Phase = "tdd"
	PhaseBuildPlanner Phase = "build-planner"
	PhaseSwarmPlan    Phase = "swarm-plan"
	PhaseBuild        Phase = "build"
	PhaseAudit        Phase = "audit"
	PhaseShip         Phase = "ship"
	PhaseRetro        Phase = "retro"
	// PhaseDebugger is the recovery phase the advisor can recommend when a
	// phase (typically ship) returns a structured error/blocker. It receives
	// the ShipError on its input, diagnoses the root cause, and emits a
	// debug-decision (RESHIP / RERUN_PHASE / BLOCK) the orchestrator executes.
	// OPTIONAL — never on the mandatory spine.
	PhaseDebugger Phase = "debugger"
	PhaseEnd      Phase = "end"
)

// String implements fmt.Stringer.
func (p Phase) String() string { return string(p) }

// IsValid reports whether p is one of the known phase constants.
func (p Phase) IsValid() bool {
	switch p {
	case PhaseStart, PhaseIntent, PhaseScout, PhaseTriage,
		PhaseTDD, PhaseBuildPlanner, PhaseSwarmPlan,
		PhaseBuild, PhaseAudit, PhaseShip,
		PhaseRetro, PhaseDebugger, PhaseEnd:
		return true
	}
	return false
}
