package cyclestate

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
	PhaseDebugger     Phase = "debugger"
	PhaseEnd          Phase = "end"
)

func (p Phase) String() string { return string(p) }

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
