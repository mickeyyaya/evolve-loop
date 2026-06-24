// Package buildplanner implements the optional build-planner phase (Opt C).
// The phase externalises Builder's Step 3 (chain-of-thought design) into a
// separate Opus session so the Builder can spend its 25-turn budget on
// implementation rather than planning.
//
// Rollout (3 cycles):
//   - Cycle 1 (this): shadow mode — EVOLVE_BUILD_PLANNER=0 default; phase wired
//     but always skipped; Go constant, state-machine edges, and profile exist.
//   - Cycle 2: advisory — flip default to 1; build-plan.md produced and read
//     by Builder's Step 2.8.
//   - Cycle 3: enforce — Builder's internal Step 3 removed; Auditor checks
//     build-plan.md adherence.
//
// Shadow-mode invariant: ShouldSkip returns (true, SKIPPED, "build", nil)
// whenever EVOLVE_BUILD_PLANNER != "1", so the bridge is never called and
// Bridge / Prompts may be nil in Config.
package buildplanner

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// Config is the build-planner phase configuration.
// Bridge and Prompts may be nil in shadow mode (EVOLVE_BUILD_PLANNER=0)
// because ShouldSkip returns before either is consulted.
type Config struct {
	Bridge  core.Bridge
	Prompts *prompts.Loader
}

// Phase implements runner.Hooks and runner.Skipper for the build-planner
// phase. Construct via New; obtain a core.PhaseRunner via BaseRunner.
type Phase struct {
	bridge  core.Bridge
	prompts *prompts.Loader
}

// New returns a Phase ready to be wired into the orchestrator.
func New(cfg Config) *Phase {
	return &Phase{bridge: cfg.Bridge, prompts: cfg.Prompts}
}

// BaseRunner wraps the phase in a runner.BaseRunner so it satisfies
// core.PhaseRunner. Call this once at startup and register the result
// in the orchestrator's runners map.
func (p *Phase) BaseRunner() *runner.BaseRunner {
	// Optional: build-planner is advisory (opt-in via EVOLVE_BUILD_PLANNER). If
	// its artifact never appears (ExitArtifactTimeout — e.g. cycle-120 quota
	// exhaustion), it degrades to WARN and the cycle advances to build instead
	// of aborting. See Workstream D.
	return runner.New(runner.Options{Hooks: p, Bridge: p.bridge, Prompts: p.prompts, Optional: true})
}

// ShouldSkip implements runner.Skipper. Delegates to the central PhasePolicy
// (config.Load is the sole reader of EVOLVE_BUILD_PLANNER). Legacy posture
// preserved: build-planner is opt-in — skipped unless the flag enables it.
func (p *Phase) ShouldSkip(req core.PhaseRequest) (bool, string, string, []core.Diagnostic) {
	if router.PolicyForProject(req.ProjectRoot, req.Env).ShouldRunPhase(string(core.PhaseBuildPlanner)) {
		return false, "", "", nil
	}
	return true, core.VerdictSKIPPED, string(core.PhaseBuild), nil
}

// PhaseName implements runner.Hooks.
func (p *Phase) PhaseName() string { return string(core.PhaseBuildPlanner) }

// AgentPromptName implements runner.Hooks.
func (p *Phase) AgentPromptName() string { return "evolve-build-planner" }

// ArtifactFilename implements runner.Hooks.
func (p *Phase) ArtifactFilename(_ core.PhaseRequest) string { return "build-plan.md" }

// DefaultModel implements runner.Hooks. Opus: independent LLM session
// from Builder's Sonnet to preserve anti-cooperative-bias invariant.
func (p *Phase) DefaultModel() string { return "opus" }

// ComposePrompt implements runner.Hooks. Delegates to runner.BaseCycleContext;
// build-planner has no phase-specific extras.
func (p *Phase) ComposePrompt(body string, req core.PhaseRequest) string {
	return runner.BaseCycleContext(body, req)
}

// Classify implements runner.Hooks. Verifies the required section headings
// in build-plan.md are present. Cycle 1 (advisory): a missing heading is
// WARN rather than FAIL; enforcement lands in Cycle 3.
func (p *Phase) Classify(artifact string, _ core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	next := string(core.PhaseBuild)
	if strings.TrimSpace(artifact) == "" {
		return core.VerdictFAIL, []core.Diagnostic{{Severity: "error", Message: "build-plan.md is empty"}}, next
	}
	return core.VerdictPASS, nil, next
}
