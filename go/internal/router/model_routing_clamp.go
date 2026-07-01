package router

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// ClampPlanModelRouting is the cycle-436 MR2 guardrail: it re-validates every
// plan entry's advisor-proposed {CLI,Tier} against the phase's OWN profile
// guardrails (allowed_clis + model_tier_envelope, via the EXISTING
// policy.ValidatePin — reused, not forked) and the live model catalog
// (modelcatalog.Catalog.Lookup), clamping any out-of-bounds or
// catalog-unresolvable proposal back to the safe static default ({cli:"",
// tier:""}, which the resolver already treats as "use the profile's pinned
// default") rather than ever letting an illegal or unresolvable pair reach
// dispatch ("model proposes, kernel disposes"). profileFor resolves a
// phase's profile lazily (nil ⇒ nothing to validate ⇒ honored, matching
// ValidatePin's own nil-profile contract); a NIL profile is distinct from a
// profile with an empty AllowedCLIs (B2: no restriction configured is a
// PREFERENCE, not a violation — ValidatePin already encodes this). An entry
// that proposes neither CLI nor Tier is left untouched (nothing to clamp).
// PURE: returns a NEW plan (input unmutated) plus the clamps applied, so it
// composes with ClampPlanToFloorWith exactly like every other router clamp.
func ClampPlanModelRouting(plan *PhasePlan, profileFor func(phase string) *profiles.Profile, catalog modelcatalog.Catalog) (*PhasePlan, []Clamp) {
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
			continue // nothing proposed — nothing to clamp
		}
		prof := profileFor(e.Phase)
		pin := policy.Pin{CLI: e.CLI, Model: e.Tier}
		if err := policy.ValidatePin(e.Phase, pin, prof); err != nil {
			clamps = append(clamps, Clamp{
				Rule:     "model-routing-guardrail",
				Proposed: fmt.Sprintf("%s={cli:%q,tier:%q}", e.Phase, e.CLI, e.Tier),
				Forced:   e.Phase + "={cli:,tier:} (profile default)",
			})
			e.CLI, e.Tier = "", ""
			continue
		}
		if e.CLI != "" && e.Tier != "" {
			if _, ok := catalog.Lookup(e.CLI, e.Tier); !ok {
				clamps = append(clamps, Clamp{
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
