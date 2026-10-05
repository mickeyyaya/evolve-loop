package cliroute

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

type resolution struct {
	req        Request
	prof       *profiles.Profile
	allowed    []string
	base       llmroute.Plan
	rule       string
	modelFixed bool
	plan       llmroute.Plan
	trace      []string
}

type stage func(r *Router, res resolution) (resolution, error)

var declaredStages = []stage{
	selectRule, fixAgentModel, guardEnv, leadWithCaller, overlayCLI, overlayTier,
	probeChain, benchChain, appendAfterChain, applyTierCeiling,
}

func (r *Router) resolveDeclared(req Request) (Decision, error) {
	prof, err := r.table.profile(req.Agent)
	if err != nil {
		return Decision{}, fmt.Errorf("cliroute: agent %s: %w", req.Agent, err)
	}
	res := resolution{
		req: req, prof: prof, allowed: r.table.allowed(req.Agent, prof),
		base: llmroute.Resolve(req.Agent, req.Phase, req.DefaultModel, req.Env, prof, req.Expand, nil),
	}
	for _, next := range declaredStages {
		out, err := next(r, res)
		if err != nil {
			return Decision{}, fmt.Errorf("cliroute: agent %s: %w", req.Agent, err)
		}
		res = out
	}
	r.logf("[cliroute] agent=%s phase=%s rule=%s chain=%v tiers=%v allowed=%v trace=%q\n",
		req.Agent, req.Phase, res.rule, res.plan.Candidates, res.plan.Tiers, res.allowed, res.trace)
	return Decision{Plan: res.plan, Rule: res.rule, Allowed: res.allowed, Trace: res.trace}, nil
}

func (res resolution) noted(format string, args ...any) resolution {
	res.trace = append(slices.Clone(res.trace), fmt.Sprintf(format, args...))
	return res
}

func selectRule(r *Router, res resolution) (resolution, error) {
	picked, ruleErr := r.table.selectRule(res.req.Agent, r.table.roleFor(res.req), res.prof, res.allowed)
	if ruleErr != nil {
		return res, ruleErr
	}
	res.rule, res.plan = picked.rule, res.base
	res.plan.Candidates, res.plan.PrimarySource = picked.chain, picked.rule
	if len(picked.dropped) > 0 {
		return res.noted("%s: %v filtered out, outside the allowed set %v", picked.rule, picked.dropped, res.allowed), nil
	}
	return res, nil
}

func fixAgentModel(r *Router, res resolution) (resolution, error) {
	model, set := r.table.models[res.req.Agent]
	if !set || res.req.Env[envchain.PhaseEnvKey(res.req.Agent, "MODEL")] != "" {
		return res, nil
	}
	res.plan = llmroute.ApplySoftOverlay(res.plan, llmroute.Overlay{Tier: model}, res.prof)
	res.modelFixed = true
	return res, nil
}

func guardEnv(_ *Router, res resolution) (resolution, error) {
	source := res.base.PrimarySource
	if !strings.HasPrefix(source, "env(") {
		return res, nil
	}
	primary := res.base.Candidates[0]
	if !slices.Contains(res.allowed, familyOf(primary)) {
		return res, fmt.Errorf("%s primary %s is outside the allowed set %v", source, primary, res.allowed)
	}
	res.plan.Candidates = leadWith(primary, res.plan.Candidates)
	res.plan.PrimarySource = source
	return res, nil
}

func leadWithCaller(_ *Router, res resolution) (resolution, error) {
	caller := res.req.CallerCLI
	if caller == "" || res.rule != ruleProfile || strings.HasPrefix(res.plan.PrimarySource, "env(") {
		return res, nil
	}
	if !slices.Contains(res.allowed, familyOf(caller)) {
		return res.noted("caller cli %s ignored: outside the allowed set %v", caller, res.allowed), nil
	}
	res.plan.Candidates = leadWith(caller, res.plan.Candidates)
	return res, nil
}

func overlayCLI(_ *Router, res resolution) (resolution, error) {
	cli := res.req.Overlay.CLI
	switch {
	case cli == "":
		return res, nil
	case res.rule != ruleProfile:
		return res.noted("advisor cli %s kept off: rule %s fixes the chain", cli, res.rule), nil
	case !slices.Contains(res.allowed, familyOf(cli)):
		return res.noted("advisor cli %s ignored: outside the allowed set %v", cli, res.allowed), nil
	}
	res.plan = llmroute.ApplySoftOverlay(res.plan, llmroute.Overlay{CLI: cli}, res.prof)
	return res, nil
}

func overlayTier(_ *Router, res resolution) (resolution, error) {
	tier := res.req.Overlay.Tier
	if tier == "" {
		return res, nil
	}
	if res.modelFixed {
		return res.noted("advisor tier %s kept off: agents.%s.model fixes the tier", tier, res.req.Agent), nil
	}
	res.plan = llmroute.ApplySoftOverlay(res.plan, llmroute.Overlay{Tier: tier}, res.prof)
	return res, nil
}

func probeChain(r *Router, res resolution) (resolution, error) {
	res.plan = llmroute.Probe(res.plan, r.host.LookPath)
	return res, nil
}

func benchChain(r *Router, res resolution) (resolution, error) {
	res.plan = r.bench(res.req, res.plan)
	return res, nil
}

func appendAfterChain(r *Router, res resolution) (resolution, error) {
	res.plan = r.table.withAfterChain(res.plan, res.allowed)
	return res, nil
}

func applyTierCeiling(r *Router, res resolution) (resolution, error) {
	if len(r.table.tiers) == 0 {
		return res, nil
	}
	res.plan.TierCeiling = cloneCeiling(r.table.tiers)
	tiers := res.plan.Tiers
	if len(tiers) == 0 {
		tiers = []string{res.plan.Model}
	}
	if ceilingErr := r.table.ceilingRefusal(res.rule, res.plan.Candidates, tiers); ceilingErr != nil {
		return res, ceilingErr
	}
	return res, nil
}
