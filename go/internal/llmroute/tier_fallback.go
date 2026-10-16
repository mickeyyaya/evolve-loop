package llmroute

import (
	"errors"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const exitQuotaExhausted = 85

const universalTierFloorMin = "balanced"

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

type TieredDispatchResult struct {
	CLI      string
	Tier     string
	Attempts []string
	Err      error
	Walled   bool
}

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
