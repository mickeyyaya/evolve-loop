package router

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// universalTierFloor is the compiled-default model-tier envelope applied when a
// profile declares no explicit model_tier_envelope (cycle-480,
// universal-envelope-floor). 72/91 profiles omit an envelope; without a default
// those phases skipped the clamp-up gate entirely and a below-floor advisor tier
// proposal fell through to policy.ValidatePin as a mere PREFERENCE (B2), never
// clamped. Single-sourcing the floor HERE — rather than editing every profile
// JSON — guarantees a below-floor tier is clamped UP to "balanced" for EVERY
// phase, while any profile that DOES declare an envelope still wins (its explicit
// Min is used verbatim). Min is the only field the clamp-up gate consults; Max
// documents the intended ceiling for parity with declared envelopes (there is no
// clamp-DOWN today).
var universalTierFloor = &profiles.ModelTierEnvelope{Min: "balanced", Max: "deep"}

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
// catalogLookup resolves (cli,tier)→(model,ok); it is INJECTED (dependency
// inversion) so router stays a leaf and never imports modelcatalog — the
// caller passes modelcatalog.Catalog.Lookup. A nil catalogLookup skips the
// catalog-resolvability gate (guardrail validation still applies).
// PURE: returns a NEW plan (input unmutated) plus the clamps applied, so it
// composes with ClampPlanToFloorWith exactly like every other router clamp.
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
			continue // nothing proposed — nothing to clamp
		}
		prof := profileFor(e.Phase)

		// Operator low-model floor (cycle-463 T4; universalized cycle-480): a tier
		// proposal BELOW the phase's envelope minimum clamps UP to the floor rather
		// than emptying the whole proposal — the CLI is left untouched since only
		// the tier violated a bound. When the profile declares no envelope, the
		// compiled universalTierFloor is substituted so the floor is UNIVERSAL
		// across every phase (72/91 profiles omit an envelope). TierRank returns 0
		// for an unclassifiable string, so this only fires when both ranks are real.
		if prof != nil && e.Tier != "" {
			env := prof.ModelTierEnvelope
			if env == nil {
				env = universalTierFloor
			}
			tierRank := policy.TierRank(e.Tier)
			minRank := policy.TierRank(env.Min)
			if tierRank > 0 && minRank > 0 && tierRank < minRank {
				clamps = append(clamps, Clamp{
					Phase:    e.Phase,
					Rule:     "model-routing-guardrail",
					Proposed: fmt.Sprintf("%s={cli:%q,tier:%q}", e.Phase, e.CLI, e.Tier),
					Forced:   fmt.Sprintf("%s={cli:%q,tier:%q} (clamped up to envelope floor)", e.Phase, e.CLI, env.Min),
				})
				e.Tier = env.Min
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

// RejectionsFromClamps converts router Clamps (from ClampPlanModelRouting or
// the integrity-floor clamp) into the PlanRejection shape the
// advisor-rejections.json artifact already uses, so a model-routing clamp is
// visible in the SAME rejection artifact operators already read — naming the
// phase and the rule that fired. Empty input yields nil (no artifact noise
// for the common no-clamp cycle).
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
