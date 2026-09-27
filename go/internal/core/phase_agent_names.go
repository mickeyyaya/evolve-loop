package core

// phaseAgentName maps a phase's routing name to the AGENT name whose profile
// JSON governs it (.evolve/profiles/<agent>.json). Package-private: internal
// wiring for Orchestrator.profileForModelRouting, not an exported surface.
var phaseAgentName = map[string]string{
	string(PhaseIntent):       "intent",        // evolve-intent
	string(PhaseScout):        "scout",         // evolve-scout
	string(PhaseTriage):       "triage",        // evolve-triage
	string(PhaseTDD):          "tdd-engineer",  // evolve-tdd-engineer
	string(PhaseBuildPlanner): "build-planner", // evolve-build-planner
	string(PhaseBuild):        "builder",       // evolve-builder
	string(PhaseAudit):        "auditor",       // evolve-auditor
	string(PhaseRetro):        "retrospective", // evolve-retrospective
	string(PhaseDebugger):     "debugger",      // evolve-debugger
	string(PhaseSwarmPlan):    "swarm-planner", // evolve-swarm-planner
}
