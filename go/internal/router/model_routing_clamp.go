package router

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// universalTierFloor is the envelope for a profile that declares none. Max is "top", the highest
// policy.TierRank, so only an explicitly declared lower Max ever clamps a tier down.
var universalTierFloor = &profiles.ModelTierEnvelope{Min: "balanced", Max: "top"}

// envelopeClamp records a floor or ceiling clamp. reason is echoed into Forced because
// RejectionsFromClamps' consumers grep for "floor" and "ceiling".
func envelopeClamp(e *PhasePlanEntry, forcedTier, reason string) Clamp {
	return Clamp{
		Phase:    e.Phase,
		Rule:     "model-routing-guardrail",
		Proposed: fmt.Sprintf("%s={cli:%q,tier:%q}", e.Phase, e.CLI, e.Tier),
		Forced:   fmt.Sprintf("%s={cli:%q,tier:%q} (%s)", e.Phase, e.CLI, forcedTier, reason),
	}
}

// ClampPlanModelRouting re-validates each entry's proposed {CLI, Tier} against its profile envelope,
// policy.ValidatePin and catalogLookup (modelcatalog.Catalog.Lookup, injected; nil skips the catalog
// check). A tier outside the envelope is clamped to its bound; any other violation empties the pair to
// the profile default. It returns a new plan and the clamps applied.
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

		// A tier outside the envelope clamps to the bound and keeps the CLI. TierRank
		// is 0 for an unknown tier, so only real ranks clamp.
		if prof != nil && e.Tier != "" {
			env := prof.ModelTierEnvelope
			if env == nil {
				env = universalTierFloor
			}
			tierRank := policy.TierRank(e.Tier)
			if minRank := policy.TierRank(env.Min); tierRank > 0 && minRank > 0 && tierRank < minRank {
				clamps = append(clamps, envelopeClamp(e, env.Min, "clamped up to envelope floor"))
				e.Tier = env.Min
				continue
			}
			if maxRank := policy.TierRank(env.Max); tierRank > 0 && maxRank > 0 && tierRank > maxRank {
				clamps = append(clamps, envelopeClamp(e, env.Max, "clamped down to envelope ceiling"))
				e.Tier = env.Max
				continue
			}
		}

		pin := policy.Pin{CLI: e.CLI, Model: e.Tier}
		if err := policy.ValidatePin(e.Phase, pin, prof); err != nil {
			clamps = append(clamps, Clamp{
				Phase:    e.Phase,
				Rule:     "model-routing-guardrail",
				Proposed: fmt.Sprintf("%s={cli:%q,tier:%q}", e.Phase, e.CLI, e.Tier),
				Forced:   e.Phase + "={cli:,tier:} (profile default)",
			})
			e.CLI, e.Tier = "", ""
			continue
		}
		if catalogLookup != nil && e.CLI != "" && e.Tier != "" {
			if _, ok := catalogLookup(policy.BaseCLI(e.CLI), e.Tier); !ok {
				clamps = append(clamps, Clamp{
					Phase:    e.Phase,
					Rule:     "model-routing-catalog-miss",
					Proposed: fmt.Sprintf("%s={cli:%q,tier:%q}", e.Phase, e.CLI, e.Tier),
					Forced:   e.Phase + "={cli:,tier:} (profile default)",
				})
				e.CLI, e.Tier = "", ""
			}
		}
	}
	return out, clamps
}

// RejectionsFromClamps converts clamps into advisor-rejections.json records; no clamps yields nil.
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
