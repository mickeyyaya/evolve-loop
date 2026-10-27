package cliroute

import (
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

const (
	legacyRulePrefix     = "legacy:"
	RuleLegacyPin        = "legacy:pin"
	ruleLegacyEnv        = "legacy:env"
	ruleLegacyCaller     = "legacy:caller"
	ruleLegacyDefault    = "legacy:default"
	ruleLegacyProfile    = "legacy:profile"
	ruleLegacyRouter     = "legacy:router"
	ruleLegacyClassifier = "legacy:classifier"
)

func legacyDefaultDriver() string { return policy.SeatDefault(policy.SeatNoProfile).CLI }

func (v resolver) resolveLegacy(req Request) (Decision, error) {
	switch req.Launch {
	case LaunchAdvisor:
		return v.legacyAdvisor(req), nil
	case LaunchClassifier:
		return legacyClassifier(), nil
	}
	return v.legacyDispatch(req)
}

func (v resolver) legacyDispatch(req Request) (Decision, error) {
	prof := v.table.legacyProfile(req.Agent)
	pin, err := v.table.legacyPin(req.Phase, prof)
	if err != nil {
		return Decision{}, err
	}
	plan := llmroute.Resolve(req.Agent, req.Phase, req.DefaultModel, req.Env, v.table.overrides.For(req.Agent), prof, req.Expand, pin)
	plan.Candidates = leadWith(req.CallerCLI, plan.Candidates)
	var clamped []string
	if pin == nil && req.Overlay != (llmroute.Overlay{}) {
		var ov llmroute.Overlay
		if ov, clamped, err = clampLadder(req.Overlay, retryladder.Bounds{}.Resolved(), v.table.ladderBase(req.Agent, plan.Model)); err != nil {
			return Decision{}, refused(req.Agent, err)
		}
		plan = llmroute.ApplySoftOverlay(plan, ov, prof)
	}
	if pin == nil || pin.CLI == "" {
		plan = v.legacyUnlocked(req, prof, plan)
	}
	rule := legacyRule(req, plan, pin)
	toolPlan, notes, err := v.requireToolUse(req, rule, plan)
	if err != nil {
		return Decision{}, refused(req.Agent, err)
	}
	return Decision{Plan: toolPlan, Rule: rule, Allowed: prof.AllowedFamilies(), Trace: slices.Concat(clamped, notes)}, nil
}

func (t Table) legacyPin(phase string, prof *profiles.Profile) (*policy.Pin, error) {
	if phase == "" {
		return nil, nil
	}
	pin, ok := t.pol.PinFor(phase)
	if !ok {
		return nil, nil
	}
	if err := policy.ValidatePin(phase, pin, prof); err != nil {
		return nil, err
	}
	return &pin, nil
}

func (v resolver) legacyUnlocked(req Request, prof *profiles.Profile, plan llmroute.Plan) llmroute.Plan {
	probed := llmroute.Probe(plan, v.host.LookPath)
	v.logReorder(req, "capability probe reordered", plan.Candidates, probed.Candidates)
	plan = v.bench(req, probed)
	wf := v.table.pol.WorkflowConfig()
	if !wf.UniversalFallback || v.host.Discover == nil {
		return plan
	}
	discovered := llmroute.ExcludeFamilies(v.host.Discover(), wf.UniversalFallbackExclude)
	tailed := llmroute.ApplyUniversalFallback(plan, llmroute.AllowedDiscovered(discovered, prof))
	v.logReorder(req, "universal fallback appended to", plan.Candidates, tailed.Candidates)
	return tailed
}

func (v resolver) logReorder(req Request, what string, before, after []string) {
	if !slices.Equal(before, after) {
		v.logf("[cliroute] agent=%s phase=%s %s the chain: %v -> %v\n", req.Agent, req.Phase, what, before, after)
	}
}

func legacyRule(req Request, plan llmroute.Plan, pin *policy.Pin) string {
	switch {
	case pin != nil:
		return RuleLegacyPin
	case req.CallerCLI != "":
		return ruleLegacyCaller
	case llmroute.IsOperatorChoice(plan.PrimarySource):
		return ruleLegacyEnv
	case plan.PrimarySource == "default":
		return ruleLegacyDefault
	}
	return ruleLegacyProfile
}

func (v resolver) legacyAdvisor(req Request) Decision {
	prof := v.table.legacyProfile(req.Agent)
	primary, model, rule := legacyDefaultDriver(), req.DefaultModel, ruleLegacyDefault
	if prof != nil && prof.CLI != "" {
		primary, rule = prof.CLI, ruleLegacyProfile
	}
	if prof != nil && prof.ModelTierDefault != "" {
		model = prof.ModelTierDefault
	}
	keys := v.table.pol.RouterConfig()
	if keys.CLI != "" {
		primary, rule = keys.CLI, ruleLegacyRouter
	}
	if keys.Model != "" {
		model = keys.Model
	}
	plan := llmroute.ChainFor(primary, prof)
	plan.Candidates = withoutDuplicates(append(plan.Candidates, legacyDefaultDriver()))
	plan.Model, plan.PrimarySource = model, rule
	return Decision{Plan: plan, Rule: rule, Allowed: prof.AllowedFamilies()}
}

func legacyClassifier() Decision {
	families := policy.ClassifierFamilies()
	chain := make([]string, 0, len(families))
	for _, fam := range families {
		chain = append(chain, llmroute.DefaultDriverForFamily(fam))
	}
	return Decision{Plan: llmroute.Plan{Candidates: chain, PrimarySource: ruleLegacyClassifier}, Rule: ruleLegacyClassifier}
}

func withoutDuplicates(chain []string) []string {
	out := make([]string, 0, len(chain))
	for _, c := range chain {
		if !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	return out
}
