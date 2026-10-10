package bridgechain

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/launchoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

type WallKeeper struct {
	rungs        []string
	res          core.BridgeResponse
	err          error
	unproven     core.BridgeResponse
	unprovenErr  error
	unprovenRung string
}

func (k *WallKeeper) Observe(rung string, res core.BridgeResponse, err error) {
	k.rungs = append(k.rungs, fmt.Sprintf("%s=%d", rung, res.ExitCode))
	switch {
	case err == nil:
	case res.ExitCode == exitQuota:
		k.res, k.err = res, err
	case neverStartedWork(res.ExitCode):
	case !res.UsageExhausted && k.unprovenErr == nil:
		k.unproven, k.unprovenErr, k.unprovenRung = res, err, rung
	}
}

func (k WallKeeper) Surfaces(walk llmroute.TieredDispatchResult, res core.BridgeResponse) bool {
	if !walk.Walled || k.err == nil {
		return false
	}
	return k.unprovenErr != nil || res.ExitCode != exitQuota
}

func (k WallKeeper) Surface(walk llmroute.TieredDispatchResult, res core.BridgeResponse, err error) (core.BridgeResponse, error) {
	if !k.Surfaces(walk, res) {
		return res, err
	}
	rungs := strings.Join(k.rungs, " -> ")
	if k.unprovenErr != nil {
		return k.unproven, fmt.Errorf("%w; the dispatch walk %s met a quota wall, but rung %s failed with no evidence of exhaustion, so the walk is not a quota pause", k.unprovenErr, rungs, k.unprovenRung)
	}
	return k.res, fmt.Errorf("%w; the dispatch walk %s met a quota wall before its last rung failed", k.err, rungs)
}

func neverStartedWork(exitCode int) bool {
	switch exitCode {
	case launchoutcome.ExitREPLBootTimeout, launchoutcome.ExitModelMismatch, launchoutcome.ExitMissingBinary:
		return true
	}
	return false
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
