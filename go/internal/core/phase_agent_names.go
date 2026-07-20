package core

// phaseAgentName maps a phase's routing name (the string carried on a
// router.PhasePlanEntry.Phase / core.Phase) to the AGENT name whose profile
// JSON governs it: .evolve/profiles/<agent>.json. Each value is exactly the
// phase package's own AgentPromptName() with the "evolve-" prefix stripped —
// the same convention phases/runner/runner.go uses to resolve a profile
// (build → "evolve-builder" → builder.json). core cannot import the phases/*
// packages to read AgentPromptName() directly (they already import core, so it
// would be an import cycle), so this static table is the cycle-safe single
// source for the phase→agent mapping inside core.
//
// Package-private on purpose: this is internal wiring for
// Orchestrator.profileForModelRouting, not an exported surface — so there is no
// new exported symbol to give a caller/naming test. Phases with no LLM agent
// and thus no profile (e.g. the native ship phase) are intentionally ABSENT: a
// miss resolves to a nil profile, the documented ValidatePin nil-profile
// pass-through, rather than fabricating a wrong mapping.
//
// Drift guard: phase_agent_names_test.go asserts every value here has a real
// .evolve/profiles/<agent>.json on disk, so a rename that desyncs this table
// from a phase package's AgentPromptName() fails loudly.
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
