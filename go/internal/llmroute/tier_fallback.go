package llmroute

import (
	"errors"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// exitQuotaExhausted is the bridge exit for a provider quota wall (see the
// defaultFallbackOnExit doc: 85 = ExitUnknownPrompt incl. rate-limit
// escalations). It is the ONLY exit that promotes a tier step-down: other
// trigger exits (80/81/124/127) are CLI-level problems, not tier-availability
// problems.
const exitQuotaExhausted = 85

// universalTierFloorMin is the lowest tier DispatchTiered will ever step down
// to when the phase's ModelTierEnvelope.Min is empty or unclassifiable —
// mirror of the router's cycle-480 universalTierFloor{Min:"balanced"}.
const universalTierFloorMin = "balanced"

// tierNameByRank inverts policy.TierRank onto the canonical tier vocabulary
// (fast < balanced < deep < top) for stepping down one rank at a time.
var tierNameByRank = map[int]string{1: "fast", 2: "balanced", 3: "deep", 4: "top"}

// TierChain builds the ordered tier fallback list for Plan.Tiers: the resolved
// tier first, then one policy.TierRank rank down at a time, never below
// envelopeMin. An empty or unclassifiable envelopeMin means the universal
// "balanced" floor. An unclassifiable resolved tier (rank 0, e.g. an exact
// model id) yields a single-element chain — no bogus step-down.
func TierChain(resolved, envelopeMin string) []string {
	chain := []string{resolved}
	rank := policy.TierRank(resolved)
	if rank == 0 {
		return chain
	}
	floor := policy.TierRank(envelopeMin)
	if floor == 0 {
		floor = policy.TierRank(universalTierFloorMin)
	}
	for r := rank - 1; r >= floor; r-- {
		chain = append(chain, tierNameByRank[r])
	}
	return chain
}

// TieredDispatchResult is the outcome of walking a Plan's tier×CLI grid: the
// CLI and tier that produced the terminal result, every launch in order
// ("cli@tier"), and the terminal error (nil on success).
type TieredDispatchResult struct {
	CLI      string   // the CLI that produced the terminal result
	Tier     string   // the tier the terminal result ran at
	Attempts []string // every launch as "cli@tier", in order
	Err      error    // nil on success; the terminal attempt's error otherwise
}

// DispatchTiered walks plan.Tiers outer, plan.Candidates inner. Within a tier
// it behaves exactly like Dispatch: a nil error stops on success, a trigger
// exit advances the CLI chain, and a non-trigger exit (a legitimate FAIL)
// stops the walk immediately. It steps down to the next tier ONLY when EVERY
// attempt at the current tier exited 85 (quota) — the full CLI chain is
// drained at that tier, so a lower tier is the only remaining capacity. Each
// step-down invokes onStepDown(from, to) (nil-safe) so the downgrade is never
// silent. An empty plan.Tiers degrades to a single-tier walk at plan.Model.
//
// The terminal quota error is therefore only reachable after the LOWEST tier
// in the chain is also exhausted — the precondition for the DEFERRED /
// all-families-exhausted classification (core/quota_exhaustion.go).
func DispatchTiered(plan Plan, launch func(cli, tier string) (exitCode int, err error), onStepDown func(from, to string)) TieredDispatchResult {
	if len(plan.Candidates) == 0 {
		return TieredDispatchResult{Err: errors.New("llmroute: DispatchTiered called with no candidates")}
	}
	tiers := plan.Tiers
	if len(tiers) == 0 {
		tiers = []string{plan.Model}
	}
	var attempts []string
	var cli, tier string
	var err error
	for i, t := range tiers {
		tier = t
		allQuota := true
		for _, cli = range plan.Candidates {
			var exitCode int
			exitCode, err = launch(cli, tier)
			attempts = append(attempts, cli+"@"+tier)
			if err == nil || !plan.TriggersFallback(exitCode) {
				return TieredDispatchResult{CLI: cli, Tier: tier, Attempts: attempts, Err: err}
			}
			if exitCode != exitQuotaExhausted {
				allQuota = false
			}
		}
		if !allQuota || i == len(tiers)-1 {
			break
		}
		if onStepDown != nil {
			onStepDown(tier, tiers[i+1])
		}
	}
	return TieredDispatchResult{CLI: cli, Tier: tier, Attempts: attempts, Err: err}
}
