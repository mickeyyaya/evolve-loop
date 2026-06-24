// Package swarmplan implements the optional swarm-plan phase: an LLM planner
// that partitions one phase's task into N independent worker assignments for the
// swarm harness (ADR-0032). It emits swarm-plan.md (a fenced JSON block parsed
// by internal/swarm.ParsePlan).
//
// Rollout (mirrors buildplanner's shadow→advisory→enforce ladder):
//   - v1 (this): SHADOW — EVOLVE_SWARM_STAGE default "shadow"/"off"; the phase
//     is wired and (when enabled) produces swarm-plan.md for inspection, but the
//     orchestrator does NOT yet dispatch a swarm — build/scout still run N=1.
//   - v3: ADVISORY — the dispatcher fans out workers; integration is computed
//     but compared, not authoritative.
//   - v4: ENFORCE — the merge-train integration result is the phase output.
//
// Shadow-mode invariant: ShouldSkip returns SKIPPED whenever the swarm stage is
// off, so Bridge/Prompts may be nil in Config.
package swarmplan

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// Config is the swarm-plan phase configuration. Bridge and Prompts may be nil in
// shadow mode because ShouldSkip returns before either is consulted.
type Config struct {
	Bridge  core.Bridge
	Prompts *prompts.Loader
}

// Phase implements runner.Hooks and runner.Skipper for the swarm-plan phase.
type Phase struct {
	bridge  core.Bridge
	prompts *prompts.Loader
}

// New returns a Phase ready to be wired into the orchestrator.
func New(cfg Config) *Phase { return &Phase{bridge: cfg.Bridge, prompts: cfg.Prompts} }

// BaseRunner wraps the phase so it satisfies core.PhaseRunner. Optional: a
// missing artifact degrades to a single-writer fallback rather than aborting.
func (p *Phase) BaseRunner() *runner.BaseRunner {
	return runner.New(runner.Options{Hooks: p, Bridge: p.bridge, Prompts: p.prompts, Optional: true})
}

// ShouldSkip implements runner.Skipper. The swarm planner runs only when the
// swarm stage enables it (central PhasePolicy reads EVOLVE_SWARM_STAGE); in
// shadow/off it is skipped and the next phase runs single-writer.
func (p *Phase) ShouldSkip(req core.PhaseRequest) (bool, string, string, []core.Diagnostic) {
	if router.PolicyForProject(req.ProjectRoot, req.Env).ShouldRunPhase(string(core.PhaseSwarmPlan)) {
		return false, "", "", nil
	}
	return true, core.VerdictSKIPPED, string(core.PhaseBuild), nil
}

// PhaseName implements runner.Hooks.
func (p *Phase) PhaseName() string { return string(core.PhaseSwarmPlan) }

// AgentPromptName implements runner.Hooks.
func (p *Phase) AgentPromptName() string { return "evolve-swarm-planner" }

// ArtifactFilename implements runner.Hooks.
func (p *Phase) ArtifactFilename(_ core.PhaseRequest) string { return "swarm-plan.md" }

// DefaultModel implements runner.Hooks. Opus: partition design is high-leverage
// reasoning, and an independent session from the workers' models.
func (p *Phase) DefaultModel() string { return "opus" }

// ComposePrompt implements runner.Hooks. Delegates to runner.BaseCycleContext;
// swarm-plan has no phase-specific extras.
func (p *Phase) ComposePrompt(body string, req core.PhaseRequest) string {
	return runner.BaseCycleContext(body, req)
}

// Classify implements runner.Hooks. v1 is non-blocking: an empty artifact is a
// FAIL (the planner produced nothing), but any non-empty plan PASSes — the
// swarm validator (internal/swarm.Validate) is the real gate at dispatch time,
// and a non-partitionable plan is a legitimate "fall back to N=1" outcome, not a
// phase failure.
func (p *Phase) Classify(artifact string, _ core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	next := string(core.PhaseBuild)
	if strings.TrimSpace(artifact) == "" {
		return core.VerdictFAIL, []core.Diagnostic{{Severity: "error", Message: "swarm-plan.md is empty"}}, next
	}
	return core.VerdictPASS, nil, next
}
