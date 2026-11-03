package fleet

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/prefixqueue"
)

type (
	RiskTier      = prefixqueue.RiskTier
	LaneCandidate = prefixqueue.LaneCandidate
	PrefixQueue   = prefixqueue.PrefixQueue
)

const (
	TierRollup = prefixqueue.TierRollup
	TierMaybe  = prefixqueue.TierMaybe
	TierIffy   = prefixqueue.TierIffy
)

var NewPrefixQueue = prefixqueue.NewPrefixQueue

type LandingMode string

const (
	LandingPerLane     LandingMode = "per-lane"
	LandingPrefixQueue LandingMode = "prefix-queue"
)

func DefaultLandingMode() LandingMode {
	return LandingPerLane
}

func ParseLandingMode(s string) (LandingMode, error) {
	switch LandingMode(s) {
	case LandingPerLane, LandingPrefixQueue:
		return LandingMode(s), nil
	default:
		return "", fmt.Errorf("invalid landing mode %q: want %q or %q", s, LandingPerLane, LandingPrefixQueue)
	}
}
