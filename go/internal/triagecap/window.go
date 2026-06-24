package triagecap

import (
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// seedK is the throughput assumed before any observation: the cycle-281 PASS
// baseline (~5 floors per builder turn, the only coverage-campaign cycle that
// shipped at full scope).
const seedK = 5

// windowSize bounds the rolling window to the last 5 floor-bearing PASS cycles.
const windowSize = 5

// K is the observed builder throughput estimate: the rounded (half-up) mean
// of floors passed per cycle over the window. An empty window yields the
// cycle-281 seed; a degenerate mean is clamped to 1 so the cap never
// collapses to reject-everything.
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

// Cap is the per-cycle committed-floor ceiling: ceil(1.25·K), with K clamped
// to ≥1 (Cap is never below 2, so a healthy single-floor cycle always fits).
func Cap(k int) int {
	if k < 1 {
		k = 1
	}
	return (5*k + 3) / 4 // ceil(5k/4)
}

// Record appends one observed PASS cycle to the window, keeping only the
// most recent windowSize entries. floors <= 0 is a no-op: zero-floor cycles
// carry no throughput signal and must not drag K toward zero. The input
// slice is never mutated.
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
