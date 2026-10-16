// Package contextfill derives how full a phase's model context window got,
// from the token counts the loop ALREADY persists.
package contextfill

import (
	"errors"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

const HotThreshold = 0.85

var ErrInvalidWindow = errors.New("contextfill: context window size must be positive")

func FillRatio(tokens cyclestate.TokenUsage, windowSize int) (float64, error) {
	if windowSize <= 0 {
		return 0, ErrInvalidWindow
	}
	occupied := tokens.Input + tokens.Output + tokens.CacheRead + tokens.CacheWrite
	return float64(occupied) / float64(windowSize), nil
}

func IsHot(ratio float64) bool { return ratio >= HotThreshold }

// minimal: this is a flat per-TIER stub, not a per-model registry. Every Claude
// tier the loop routes to today shares the same 200k-token window, so a tier map
// is the smallest thing that is true. Upgrade path when that stops holding: key
// a registry off the resolved model id (mirroring internal/modelcatalog's
// per-model tables) and keep this function as the tier-level fallback.
func WindowSizeForTier(tier string) int {
	switch tier {
	case "fast", "balanced", "deep", "top":
		return 200_000
	default:
		return 0
	}
}
