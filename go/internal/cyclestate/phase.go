package cyclestate

// Phase is the typed identity of an orchestrator lifecycle stage.
type Phase string

// The lifecycle stages; each string is a wire value in ledger and state JSON.
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
	// PhaseDebugger is the optional recovery phase that diagnoses a structured phase error; never on the mandatory spine.
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
