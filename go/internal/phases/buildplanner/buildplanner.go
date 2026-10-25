// Package buildplanner implements the optional build-planner phase.
// See docs/architecture/packages/internal-phases-buildplanner.md.
package buildplanner

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

type Config struct {
	Bridge           core.Bridge
	Prompts          *prompts.Loader
	ContractVerifier func() runner.ContractVerifier
	HostEffects      func() core.HostEffects
}

type Phase struct {
	bridge           core.Bridge
	prompts          *prompts.Loader
	contractVerifier func() runner.ContractVerifier
	hostEffects      func() core.HostEffects
}

func New(cfg Config) *Phase {
	return &Phase{bridge: cfg.Bridge, prompts: cfg.Prompts, contractVerifier: cfg.ContractVerifier, hostEffects: cfg.HostEffects}
}

func (p *Phase) BaseRunner() *runner.BaseRunner {
	return runner.New(runner.Options{Hooks: p, Bridge: p.bridge, Prompts: p.prompts, Optional: true, ContractVerifier: p.contractVerifier, HostEffects: p.hostEffects})
}

func (p *Phase) ShouldSkip(req core.PhaseRequest) (bool, string, string, []core.Diagnostic) {
	signals, err := router.Digest(req.Workspace, []string{string(core.PhaseTriage)})
	var diags []core.Diagnostic
	if err != nil {
		diags = append(diags, core.Diagnostic{Severity: "warning", Message: "build-planner routing digest failed: " + err.Error()})
	}
	if len(signals.DigestDegraded) > 0 {
		diags = append(diags, core.Diagnostic{Severity: "warning", Message: "build-planner routing digest degraded: " + strings.Join(signals.DigestDegraded, "; ")})
	}
	if router.PolicyForProject(req.ProjectRoot, req.Env).Enabled(string(core.PhaseBuildPlanner), signals) {
		return false, "", "", diags
	}
	return true, core.VerdictSKIPPED, string(core.PhaseBuild), diags
}

func (p *Phase) PhaseName() string { return string(core.PhaseBuildPlanner) }

func (p *Phase) AgentPromptName() string { return "evolve-build-planner" }

func (p *Phase) ArtifactFilename(_ core.PhaseRequest) string { return "build-plan.md" }

func (p *Phase) DefaultModel() string { return "deep" }

func (p *Phase) ComposePrompt(body string, req core.PhaseRequest) string {
	prompt := runner.BaseCycleContext(body, req)
	if req.Input.Active() {
		prompt += runner.UpstreamDigestBlock(req.Input.Upstream().DigestFor(string(core.PhaseBuildPlanner)))
	}
	return prompt
}

func (p *Phase) Classify(artifact string, _ core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	next := string(core.PhaseBuild)
	if strings.TrimSpace(artifact) == "" {
		return core.VerdictFAIL, []core.Diagnostic{{Severity: "error", Message: "build-plan.md is empty"}}, next
	}
	return core.VerdictPASS, nil, next
}
