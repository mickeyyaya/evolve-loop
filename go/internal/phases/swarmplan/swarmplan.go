// Package swarmplan implements the optional swarm-plan phase: an LLM planner
// that partitions one phase's task into N independent worker assignments for
// the swarm harness.
package swarmplan

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

type Config struct {
	Bridge  core.Bridge
	Prompts *prompts.Loader
}

type Phase struct {
	bridge  core.Bridge
	prompts *prompts.Loader
}

func New(cfg Config) *Phase { return &Phase{bridge: cfg.Bridge, prompts: cfg.Prompts} }

func (p *Phase) BaseRunner() *runner.BaseRunner {
	return runner.New(runner.Options{Hooks: p, Bridge: p.bridge, Prompts: p.prompts, Optional: true})
}

func (p *Phase) ShouldSkip(req core.PhaseRequest) (bool, string, string, []core.Diagnostic) {
	if router.PolicyForProject(req.ProjectRoot).ShouldRunPhase(string(core.PhaseSwarmPlan)) {
		return false, "", "", nil
	}
	return true, core.VerdictSKIPPED, string(core.PhaseBuild), nil
}

func (p *Phase) PhaseName() string { return string(core.PhaseSwarmPlan) }

func (p *Phase) AgentPromptName() string { return "evolve-swarm-planner" }

func (p *Phase) ArtifactFilename(_ core.PhaseRequest) string { return "swarm-plan.md" }

func (p *Phase) DefaultModel() string { return "deep" }

func (p *Phase) ComposePrompt(body string, req core.PhaseRequest) string {
	prompt := runner.BaseCycleContext(body, req)
	if req.Input.Active() {
		prompt += runner.UpstreamDigestBlock(req.Input.Upstream().DigestFor(string(core.PhaseSwarmPlan)))
	}
	return prompt
}

func (p *Phase) Classify(artifact string, _ core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	next := string(core.PhaseBuild)
	if strings.TrimSpace(artifact) == "" {
		return core.VerdictFAIL, []core.Diagnostic{{Severity: "error", Message: "swarm-plan.md is empty"}}, next
	}
	return core.VerdictPASS, nil, next
}
