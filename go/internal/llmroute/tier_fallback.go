package llmroute

import (
	"errors"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// exitQuotaExhausted is the only exit that steps a tier down; the other triggers are CLI problems, not tier availability.
const exitQuotaExhausted = 85

// universalTierFloorMin mirrors the router's universalTierFloor Min.
const universalTierFloorMin = "balanced"

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
		chain = append(chain, policy.TierName(r))
	}
	return chain
}

// TieredDispatchResult is the outcome of walking a Plan's tier×CLI grid.
type TieredDispatchResult struct {
	CLI      string   // the CLI that produced the terminal result
	Tier     string   // the tier the terminal result ran at
	Attempts []string // every launch as "cli@tier", in order
	Err      error    // nil on success; the terminal attempt's error otherwise
	Walled   bool
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
	sawWall := false
	for i, t := range tiers {
		allQuota := true
		for _, candidate := range plan.Candidates {
			if !plan.Permits(candidate, t) {
				continue
			}
			cli, tier = candidate, t
			var exitCode int
			exitCode, err = launch(cli, tier)
			attempts = append(attempts, cli+"@"+tier)
			if err == nil || !plan.TriggersFallback(exitCode) {
				return TieredDispatchResult{CLI: cli, Tier: tier, Attempts: attempts, Err: err}
			}
			if exitCode != exitQuotaExhausted {
				allQuota = false
			} else {
				sawWall = true
			}
		}
		if !allQuota || i == len(tiers)-1 {
			break
		}
		if onStepDown != nil {
			onStepDown(t, tiers[i+1])
		}
	}
	if len(attempts) == 0 {
		return TieredDispatchResult{Err: ErrNoPermittedAttempt}
	}
	return TieredDispatchResult{CLI: cli, Tier: tier, Attempts: attempts, Err: err, Walled: sawWall}
}

var ErrNoPermittedAttempt = errors.New("llmroute: the tier ceiling permits no attempt on this chain")
