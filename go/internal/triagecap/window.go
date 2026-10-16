package triagecap

import (
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const seedK = 5

const windowSize = 5

func K(window []core.TriageThroughputEntry) int {
	if len(window) == 0 {
		return seedK
	}
	sum := 0
	for _, e := range window {
		sum += e.Floors
	}
	k := halfUpMean(sum, len(window))
	if k < 1 {
		return 1
	}
	return k
}

func Cap(k int) int {
	if k < 1 {
		k = 1
	}
	return ceilDiv(5*k, 4)
}

func halfUpMean(sum, count int) int {
	return (sum*2 + count) / (2 * count)
}

func ceilDiv(numerator, denominator int) int {
	return (numerator + denominator - 1) / denominator
}

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
