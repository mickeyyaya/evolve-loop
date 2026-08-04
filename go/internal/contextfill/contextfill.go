// Package contextfill derives how full a phase's model context window got, from
// the token counts the loop ALREADY persists. Every phase records a
// cyclestate.TokenUsage (input / output / cache_read / cache_write) into
// <workspace>/phase-timing.json, but nothing today turns those absolute counts
// into the one number that says whether the phase was running out of room:
// occupancy over the model's context-window size.
//
// This package is the pure derivation only — a stdlib+cyclestate leaf with no
// wiring. Persisting the ratio (a phasetiming.Entry field), the off/advisory/
// enforce Stage dial, and the advisory prompt hint are deliberately DEFERRED to
// follow-up cycles; nothing imports this package yet.
package contextfill

import (
	"errors"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// HotThreshold is the fill ratio at or above which a phase is classified "hot":
// it consumed 85% or more of its context window and its output is at risk of
// truncation. Inclusive — see IsHot.
const HotThreshold = 0.85

// ErrInvalidWindow reports a non-positive context-window size. It is returned
// rather than tolerated so an unknown window (WindowSizeForTier's 0 for an
// unrecognised tier) surfaces as "unknown", never as a fabricated fill number or
// a silent divide-by-zero NaN/Inf.
var ErrInvalidWindow = errors.New("contextfill: context window size must be positive")

// FillRatio reports what fraction of a windowSize-token context window the given
// token usage occupied. Every TokenUsage field counts: uncached input, cache
// reads, cache writes and generated output all consume window space.
//
// The ratio is NOT clamped at 1.0 — a phase that overran its window by half must
// stay distinguishable from one that just fit, which is the whole diagnostic
// point. A non-positive windowSize returns (0, ErrInvalidWindow).
func FillRatio(tokens cyclestate.TokenUsage, windowSize int) (float64, error) {
	if windowSize <= 0 {
		return 0, ErrInvalidWindow
	}
	occupied := tokens.Input + tokens.Output + tokens.CacheRead + tokens.CacheWrite
	return float64(occupied) / float64(windowSize), nil
}

// IsHot reports whether a fill ratio has reached HotThreshold. The boundary is
// inclusive: a ratio exactly at the threshold is hot.
func IsHot(ratio float64) bool { return ratio >= HotThreshold }

// WindowSizeForTier returns the context-window size in tokens for an abstract
// model tier ("fast", "balanced", "deep", "top" — modelcatalog.CanonicalTiers).
// An unknown or empty tier returns 0, which FillRatio rejects as
// ErrInvalidWindow; no default is invented for a tier we cannot identify.
//
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
