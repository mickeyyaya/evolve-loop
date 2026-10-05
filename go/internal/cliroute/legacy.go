package cliroute

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func (r *Router) resolveLegacy(req Request) (Decision, error) {
	prof := r.table.legacyProfile(req.Agent)
	pin, err := r.table.legacyPin(req.Phase, prof)
	if err != nil {
		return Decision{}, err
	}
	plan := llmroute.Resolve(req.Agent, req.Phase, req.DefaultModel, req.Env, prof, req.Expand, pin)
	plan.Candidates = leadWith(req.CallerCLI, plan.Candidates)
	if pin == nil && (req.Overlay.CLI != "" || req.Overlay.Tier != "") {
		plan = llmroute.ApplySoftOverlay(plan, req.Overlay, prof)
	}
	if pin == nil || pin.CLI == "" {
		plan = r.legacyUnlocked(req, prof, plan)
	}
	return Decision{Plan: plan, Rule: legacyRule(req, plan), Allowed: prof.AllowedFamilies()}, nil
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

func (r *Router) legacyUnlocked(req Request, prof *profiles.Profile, plan llmroute.Plan) llmroute.Plan {
	plan = llmroute.Probe(plan, r.host.LookPath)
	plan = r.bench(req, plan)
	wf := r.table.pol.WorkflowConfig()
	if !wf.UniversalFallback || r.host.Discover == nil {
		return plan
	}
	discovered := llmroute.ExcludeFamilies(r.host.Discover(), wf.UniversalFallbackExclude)
	return llmroute.ApplyUniversalFallback(plan, llmroute.AllowedDiscovered(discovered, prof), nil)
}

func legacyRule(req Request, plan llmroute.Plan) string {
	switch {
	case plan.PrimarySource == "policy.pin":
		return "legacy:pin"
	case req.CallerCLI != "":
		return "legacy:caller"
	case strings.HasPrefix(plan.PrimarySource, "env("):
		return "legacy:env"
	case plan.PrimarySource == "default":
		return "legacy:default"
	}
	return "legacy:profile"
}
