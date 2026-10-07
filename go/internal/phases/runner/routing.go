package runner

import (
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
	"github.com/mickeyyaya/evolve-loop/go/internal/systemprompt"
)

var DefaultRouter *cliroute.Router

type phaseDispatchPlan struct {
	plan              llmroute.Plan
	overlayPolicy     policy.Policy
	modelSource       string
	permissionMode    string
	interactivePolicy string
	systemPrompt      string
}

func (b *BaseRunner) resolveDispatchPlan(req core.PhaseRequest, prep phasePreparation) (phaseDispatchPlan, *core.PhaseResponse, error) {
	router, overlayPolicy, err := b.routerFor(req, prep)
	if err != nil {
		return phaseDispatchPlan{}, routingFailure(req, prep, err), fmt.Errorf("%s: %w", prep.phase, err)
	}
	d, err := router.Resolve(b.routeRequest(req, prep, router))
	if err != nil {
		return phaseDispatchPlan{}, routingFailure(req, prep, err), fmt.Errorf("%s: %w", prep.phase, err)
	}
	resolved := phaseDispatchPlan{plan: d.Plan, overlayPolicy: overlayPolicy, modelSource: modelSourceOf(req, d)}
	b.logDecision(req, prep, d, overlayPolicy)
	resolved.permissionMode = req.Env[envchain.PhaseEnvKey(prep.profileName, "PERMISSION_MODE")]
	if resolved.permissionMode == "" && prep.profile != nil {
		resolved.permissionMode = prep.profile.PermissionMode
	}
	if prep.profile != nil {
		resolved.interactivePolicy = prep.profile.InteractivePolicy
	}
	resolved.systemPrompt = systemprompt.Resolve(prep.profileName, prep.profileDir, req.Env)
	return resolved, nil, nil
}

func routingFailure(req core.PhaseRequest, prep phasePreparation, err error) *core.PhaseResponse {
	return &core.PhaseResponse{
		Phase: prep.phase, Verdict: core.VerdictFAIL, ArtifactsDir: req.Workspace,
		Diagnostics: []core.Diagnostic{{Severity: "error", Message: err.Error()}},
	}
}

func (b *BaseRunner) routerFor(req core.PhaseRequest, prep phasePreparation) (*cliroute.Router, policy.Policy, error) {
	router := b.router
	if router == nil {
		router = DefaultRouter
	}
	if router == nil {
		return b.launchRouter(req, prep)
	}
	if req.BypassPolicy {
		return router, policy.Policy{}, nil
	}
	return router, router.Policy(), nil
}

func (b *BaseRunner) launchRouter(req core.PhaseRequest, prep phasePreparation) (*cliroute.Router, policy.Policy, error) {
	loaded := policy.Policy{}
	if !req.BypassPolicy {
		var err error
		if loaded, err = policy.Load(filepath.Join(req.ProjectRoot, ".evolve", "policy.json")); err != nil {
			return nil, policy.Policy{}, err
		}
		if err := cliroute.RefuseLaunchRouter(loaded); err != nil {
			return nil, policy.Policy{}, err
		}
	}
	router, err := cliroute.NewSingleProfileRouter(loaded, cliroute.SingleProfile{Agent: prep.profileName, Profile: prep.profile}, cliroute.Host{Bench: b.bench})
	return router, loaded, err
}

func (b *BaseRunner) routeRequest(req core.PhaseRequest, prep phasePreparation, router *cliroute.Router) cliroute.Request {
	resolve := b.resolveLLM
	if resolve == nil {
		resolve = router.ResolveRole
	}
	return cliroute.Request{
		Agent: prep.profileName, Phase: prep.phase, ProjectRoot: req.ProjectRoot, DefaultModel: b.hooks.DefaultModel(),
		Env: req.Env, Overlay: llmroute.Overlay{CLI: req.ModelRoutingCLI, Tier: req.ModelRoutingTier},
		Expand: autoExpander(resolve), BypassPolicy: req.BypassPolicy,
	}
}

func autoExpander(resolve func(string, resolvellm.Options) (resolvellm.Result, error)) llmroute.AutoModel {
	return func(role string) (string, bool) {
		result, err := resolve(role, resolvellm.Options{})
		if err != nil || result.ModelTier == "" {
			return "", false
		}
		return result.ModelTier, true
	}
}

func modelSourceOf(req core.PhaseRequest, d cliroute.Decision) string {
	switch {
	case d.Rule == cliroute.RuleLegacyPin:
		return "pin"
	case req.ModelRoutingCLI != "" || req.ModelRoutingTier != "":
		return "advisor"
	}
	return "profile"
}

func (b *BaseRunner) logDecision(req core.PhaseRequest, prep phasePreparation, d cliroute.Decision, overlayPolicy policy.Policy) {
	switch pin, pinned := overlayPolicy.PinFor(prep.phase); {
	case d.Rule == cliroute.RuleLegacyPin && pinned:
		log.Diag().Infof("[runner] phase=%s policy pin: cli=%q model=%q\n", prep.phase, pin.CLI, pin.Model)
	case req.ModelRoutingCLI != "" || req.ModelRoutingTier != "":
		b.diag.Infof("[runner] phase=%s advisor overlay cli=%s tier=%s\n", prep.phase, req.ModelRoutingCLI, req.ModelRoutingTier)
	default:
		b.diag.Infof("[runner] phase=%s no advisor overlay (profile default)\n", prep.phase)
	}
	for _, line := range d.Trace {
		log.Diag().Infof("[runner] phase=%s routing: %s\n", prep.phase, line)
	}
	plan := d.Plan
	if len(plan.Candidates) > 1 {
		log.Diag().Infof("[runner] phase=%s agent=%s cli=%s (source=%s) profile=%s fallback=%v triggers=%v\n",
			prep.phase, prep.profileName, plan.Candidates[0], plan.PrimarySource, prep.profilePath, plan.Candidates[1:], plan.Triggers)
		return
	}
	log.Diag().Infof("[runner] phase=%s agent=%s cli=%s (source=%s) profile=%s\n",
		prep.phase, prep.profileName, plan.Candidates[0], plan.PrimarySource, prep.profilePath)
}
