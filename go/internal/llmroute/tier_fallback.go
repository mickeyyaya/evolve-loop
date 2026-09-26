package llmroute

import (
	"errors"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// exitQuotaExhausted is the only exit that steps a tier down; the other triggers are CLI problems, not tier availability.
const exitQuotaExhausted = 85

// universalTierFloorMin mirrors the router's universalTierFloor Min.
const universalTierFloorMin = "balanced"

// tierNameByRank inverts policy.TierRank.
var tierNameByRank = map[int]string{1: "fast", 2: "balanced", 3: "deep", 4: "top"}

// TierChain steps down from resolved one rank at a time to envelopeMin (default "balanced"); an exact model id stays alone.
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

// TieredDispatchResult is the outcome of walking a Plan's tier×CLI grid.
type TieredDispatchResult struct {
	CLI      string   // the CLI that produced the terminal result
	Tier     string   // the tier the terminal result ran at
	Attempts []string // every launch as "cli@tier", in order
	Err      error    // nil on success; the terminal attempt's error otherwise
}

// DispatchTiered runs Dispatch per tier, stepping down (via optional onStepDown) only when every attempt exited 85.
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
