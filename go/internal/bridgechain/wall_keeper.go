package bridgechain

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

type WallKeeper struct {
	rungs []string
	res   core.BridgeResponse
	err   error
}

func (k *WallKeeper) Observe(rung string, res core.BridgeResponse, err error) {
	k.rungs = append(k.rungs, fmt.Sprintf("%s=%d", rung, res.ExitCode))
	if err != nil && res.ExitCode == exitQuota {
		k.res, k.err = res, err
	}
}

func (k WallKeeper) Surfaces(walk llmroute.TieredDispatchResult, res core.BridgeResponse) bool {
	return walk.Walled && k.err != nil && res.ExitCode != exitQuota
}

func (k WallKeeper) Surface(walk llmroute.TieredDispatchResult, res core.BridgeResponse, err error) (core.BridgeResponse, error) {
	if !k.Surfaces(walk, res) {
		return res, err
	}
	return k.res, fmt.Errorf("%w; the dispatch walk %s met a quota wall before its last rung failed", k.err, strings.Join(k.rungs, " -> "))
}

var ErrUnlaunched = errors.New("the dispatch walk made no attempt")

func Unlaunched(walk llmroute.TieredDispatchResult, plan llmroute.Plan) error {
	if len(walk.Attempts) > 0 || walk.Walled {
		return nil
	}
	cause := walk.Err
	if cause == nil {
		cause = llmroute.ErrNoPermittedAttempt
	}
	return fmt.Errorf("%w (rule %s, chain %v, tiers %v, tier ceiling %v): %w",
		ErrUnlaunched, plan.PrimarySource, plan.Candidates, plan.Tiers, plan.TierCeiling, cause)
}
