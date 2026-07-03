// Package budgethistory rolls the last-N cycles' durable timing + cost
// telemetry into a Throughput — the DURATION-based pace estimate the fleet
// budget layer (internal/fleetbudget) sizes a wave against.
//
// Why duration, not dollars: per-cycle cost is $0 on subscription CLIs (the
// exact reason the old --budget-usd cap was removed as unreliable), so cost
// cannot be a sizing input. It is carried here for display only. The honest
// budget proxy is how fast the pipeline turns cycles — the median cycle
// wall-clock, from which a per-lane cycles/hour falls out.
//
// Read-only + degrade-gracefully: Collect walks the same .evolve/runs/cycle-<n>
// workspaces soakreport does; a missing or unparsable cycle is absent evidence
// (skipped), never an error. With no measurable evidence it returns the
// zero-value Throughput — the caller must read that as "pace unknown" and fall
// back to the policy floor, never as "zero throughput". Leaf package: depends
// only on the two telemetry readers (phasetiming, cyclecost) + stdlib.
package budgethistory

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclecost"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

// msPerHour is the millisecond→hour divisor for the cycles/hour rate.
const msPerHour = 3_600_000.0

// Throughput is the measured pipeline pace over a batch of recent cycles. It is
// the budget INPUT: MedianCycleDurationMS and the per-lane CyclesPerHour derived
// from it size the wave. MedianCostUSD is display-only (subscription CLIs report
// $0) and never a sizing field. SampleCount is how many cycles actually yielded
// usable timing evidence — a small sample is a weak estimate the caller may weigh.
type Throughput struct {
	SampleCount           int
	MedianCycleDurationMS int64
	CyclesPerHour         float64 // per lane: msPerHour / MedianCycleDurationMS (0 when unknown)
	// MedianCostUSD is display-only; may be 0. CostSampleCount is how many of the
	// SampleCount cycles actually carried an event log — it can be smaller (a cycle
	// with timing but no *-events.ndjson). A caller MUST read MedianCostUSD as "no
	// cost data" when CostSampleCount == 0, not as a genuine $0, and may show the
	// median as "over N of M cycles".
	MedianCostUSD   float64
	CostSampleCount int
}

// Collect walks the run workspaces for the given cycles under projectRoot and
// rolls their phase-timing.json (wall-clock) + event logs (cost) into a
// Throughput. Cost is best-effort and display-only. Missing/unparsable cycles —
// and cycles with no measurable duration — are skipped as absent evidence; an
// empty sample returns the zero value.
func Collect(projectRoot string, cycles []int) Throughput {
	var durations []int64
	var costs []float64
	for _, c := range cycles {
		ws := filepath.Join(projectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", c))
		entries, err := phasetiming.Read(ws)
		if err != nil {
			continue // absent/unreadable timing → absent evidence
		}
		total := phasetiming.Rollup(entries).TotalMS
		if total <= 0 {
			continue // no measurable pace — can't derive a rate from 0
		}
		durations = append(durations, total)
		// Cost is display-only and strictly best-effort: a cycle can have timing
		// but no event log (ErrNoLogs), which just shrinks the cost sample.
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
	return tp
}

// number bounds the generic median to the two numeric kinds budgethistory
// aggregates: int64 durations and float64 costs.
type number interface{ ~int64 | ~float64 }

// median returns the middle value of xs (averaging the two middles on an even
// count), or the zero value for an empty slice. It sorts a copy, leaving the
// caller's slice untouched.
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
	// Midpoint form (lo + (hi-lo)/2), not (lo+hi)/2: the sum form can overflow
	// int64 on two large adjacent values; this cannot. Identical result for both
	// types. Same trick the stdlib sort uses.
	lo, hi := s[n/2-1], s[n/2]
	return lo + (hi-lo)/2
}
