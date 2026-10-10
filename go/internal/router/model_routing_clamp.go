package router

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

var universalTierFloor = &profiles.ModelTierEnvelope{Min: "balanced", Max: "top"}

func envelopeClamp(e *PhasePlanEntry, forcedTier, reason string) Clamp {
	return Clamp{
		Phase:    e.Phase,
		Rule:     "model-routing-guardrail",
		Proposed: fmt.Sprintf("%s={cli:%q,tier:%q}", e.Phase, e.CLI, e.Tier),
		Forced:   fmt.Sprintf("%s={cli:%q,tier:%q} (%s)", e.Phase, e.CLI, forcedTier, reason),
	}
}

func ClampPlanModelRouting(plan *PhasePlan, profileFor func(phase string) *profiles.Profile, catalogLookup func(cli, tier string) (string, bool)) (*PhasePlan, []Clamp) {
	if plan == nil {
		return nil, nil
	}
	out := &PhasePlan{
		Entries:    append([]PhasePlanEntry(nil), plan.Entries...),
		MintPhases: plan.MintPhases,
	}

	var clamps []Clamp
	for i := range out.Entries {
		e := &out.Entries[i]
		if e.CLI == "" && e.Tier == "" {
			continue
		}
		prof := profileFor(e.Phase)

		if c, clamped := clampToTierEnvelope(e, prof); clamped {
			clamps = append(clamps, c)
			continue
		}
		if c, rejected := clampToProfilePin(e, prof); rejected {
			clamps = append(clamps, c)
			continue
		}
		if c, missed := clampToCatalog(e, catalogLookup); missed {
			clamps = append(clamps, c)
		}
	}
	return out, clamps
}

func clampToTierEnvelope(e *PhasePlanEntry, prof *profiles.Profile) (Clamp, bool) {
	if prof == nil || e.Tier == "" {
		return Clamp{}, false
	}
	env := prof.ModelTierEnvelope
	if env == nil {
		env = universalTierFloor
	}
	tierRank := policy.TierRank(e.Tier)
	if minRank := policy.TierRank(env.Min); tierRank > 0 && minRank > 0 && tierRank < minRank {
		c := envelopeClamp(e, env.Min, "clamped up to envelope floor")
		e.Tier = env.Min
		return c, true
	}
	if maxRank := policy.TierRank(env.Max); tierRank > 0 && maxRank > 0 && tierRank > maxRank {
		c := envelopeClamp(e, env.Max, "clamped down to envelope ceiling")
		e.Tier = env.Max
		return c, true
	}
	return Clamp{}, false
}

func clampToProfilePin(e *PhasePlanEntry, prof *profiles.Profile) (Clamp, bool) {
	pin := policy.Pin{CLI: e.CLI, Model: e.Tier}
	if err := policy.ValidatePin(e.Phase, pin, prof); err == nil {
		return Clamp{}, false
	}
	c := Clamp{
		Phase:    e.Phase,
		Rule:     "model-routing-guardrail",
		Proposed: fmt.Sprintf("%s={cli:%q,tier:%q}", e.Phase, e.CLI, e.Tier),
		Forced:   e.Phase + "={cli:,tier:} (profile default)",
	}
	e.CLI, e.Tier = "", ""
	return c, true
}

func clampToCatalog(e *PhasePlanEntry, catalogLookup func(cli, tier string) (string, bool)) (Clamp, bool) {
	if catalogLookup == nil || e.CLI == "" || e.Tier == "" {
		return Clamp{}, false
	}
	if _, ok := catalogLookup(policy.BaseCLI(e.CLI), e.Tier); ok {
		return Clamp{}, false
	}
	c := Clamp{
		Phase:    e.Phase,
		Rule:     "model-routing-catalog-miss",
		Proposed: fmt.Sprintf("%s={cli:%q,tier:%q}", e.Phase, e.CLI, e.Tier),
		Forced:   e.Phase + "={cli:,tier:} (profile default)",
	}
	e.CLI, e.Tier = "", ""
	return c, true
}

func RejectionsFromClamps(clamps []Clamp) []PlanRejection {
	if len(clamps) == 0 {
		return nil
	}
	out := make([]PlanRejection, len(clamps))
	for i, c := range clamps {
		out[i] = PlanRejection{
			Phase:  c.Phase,
			Reason: c.Rule,
			Detail: fmt.Sprintf("proposed %s, forced %s", c.Proposed, c.Forced),
		}
	}
	return out
}
