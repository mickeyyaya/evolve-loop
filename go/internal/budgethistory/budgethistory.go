// Package budgethistory rolls the last-N cycles' durable timing + cost
// telemetry into a Throughput — the DURATION-based pace estimate the fleet
// budget layer (internal/fleetbudget) sizes a wave against.
package budgethistory

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclecost"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

const msPerHour = 3_600_000.0

type Throughput struct {
	SampleCount           int
	MedianCycleDurationMS int64
	CyclesPerHour         float64
	MedianCostUSD         float64
	CostSampleCount       int
	MedianTokensPerCycle  int64
}

func Collect(projectRoot string, cycles []int) Throughput {
	var durations []int64
	var costs []float64
	var tokens []int64
	for _, c := range cycles {
		ws := filepath.Join(projectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", c))
		entries, err := phasetiming.Read(ws)
		if err != nil {
			continue
		}
		sum := phasetiming.Rollup(entries)
		if sum.TotalMS <= 0 {
			continue
		}
		durations = append(durations, sum.TotalMS)
		if gross := grossTokens(sum.TotalTokens); gross > 0 {
			tokens = append(tokens, gross)
		}
		if sum, err := cyclecost.SummarizeCycle(ws, c); err == nil {
			costs = append(costs, sum.Total.CostUSD)
		}
	}

	tp := Throughput{SampleCount: len(durations)}
	if len(durations) == 0 {
		return tp
	}
	tp.MedianCycleDurationMS = median(durations)
	if tp.MedianCycleDurationMS > 0 {
		tp.CyclesPerHour = msPerHour / float64(tp.MedianCycleDurationMS)
	}
	tp.MedianCostUSD = median(costs)
	tp.CostSampleCount = len(costs)
	tp.MedianTokensPerCycle = median(tokens)
	return tp
}

func grossTokens(t cyclestate.TokenUsage) int64 {
	return int64(t.Input) + int64(t.Output) + int64(t.CacheRead) + int64(t.CacheWrite)
}

type number interface{ ~int64 | ~float64 }

func median[T number](xs []T) T {
	if len(xs) == 0 {
		return 0
	}
	s := append([]T(nil), xs...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	lo, hi := s[n/2-1], s[n/2]
	return lo + (hi-lo)/2
}
