package core

var phaseAgentName = map[string]string{
	string(PhaseIntent):       "intent",
	string(PhaseScout):        "scout",
	string(PhaseTriage):       "triage",
	string(PhaseTDD):          "tdd-engineer",
	string(PhaseBuildPlanner): "build-planner",
	string(PhaseBuild):        "builder",
	string(PhaseAudit):        "auditor",
	string(PhaseRetro):        "retrospective",
	string(PhaseDebugger):     "debugger",
	string(PhaseSwarmPlan):    "swarm-planner",
}
