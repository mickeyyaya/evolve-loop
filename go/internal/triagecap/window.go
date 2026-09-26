package triagecap

import (
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// seedK is the throughput assumed before any observation: the one coverage cycle that shipped at full scope.
const seedK = 5

const windowSize = 5

// K is the half-up mean of floors per window entry, seedK when empty, and never below 1.
func K(window []core.TriageThroughputEntry) int {
	if len(window) == 0 {
		return seedK
	}
	sum := 0
	for _, e := range window {
		sum += e.Floors
	}
	k := (sum*2 + len(window)) / (2 * len(window)) // integer mean, half-up
	if k < 1 {
		return 1
	}
	return k
}

// Cap is the committed-floor ceiling ceil(1.25·K) with K clamped to 1, so it is never below 2.
func Cap(k int) int {
	if k < 1 {
		k = 1
	}
	return (5*k + 3) / 4 // ceil(5k/4)
}

// Record returns a new window with the cycle appended, trimmed to windowSize.
// floors <= 0 records nothing, so non-coverage cycles cannot drag K toward zero.
func Record(window []core.TriageThroughputEntry, cycle, floors int) []core.TriageThroughputEntry {
	if floors <= 0 {
		return window
	}
	out := append(slices.Clone(window), core.TriageThroughputEntry{Cycle: cycle, Floors: floors})
	if len(out) > windowSize {
		out = out[len(out)-windowSize:]
	}
	return out
}
