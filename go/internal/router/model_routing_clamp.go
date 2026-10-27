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
		Proposed: fmt.Sprintf("%s={tier:%q}", e.Phase, e.Tier),
		Forced:   fmt.Sprintf("%s={tier:%q} (%s)", e.Phase, forcedTier, reason),
	}
}

func ClampPlanModelRouting(plan *PhasePlan, profileFor func(phase string) *profiles.Profile) (*PhasePlan, []Clamp) {
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
		if e.Tier == "" {
			continue
		}
		if c, clamped := clampToTierEnvelope(e, profileFor(e.Phase)); clamped {
			clamps = append(clamps, c)
		}
	}
	return out, clamps
}

func clampToTierEnvelope(e *PhasePlanEntry, prof *profiles.Profile) (Clamp, bool) {
	if prof == nil {
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
