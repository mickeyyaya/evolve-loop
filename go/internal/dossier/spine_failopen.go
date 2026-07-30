package dossier

// spine_failopen.go — the batch-level roll-up of spine-gate fail-opens
// (cycle-1166, spine-failopen-telemetry).
//
// Per-cycle records make one cycle's fail-opens visible; they do NOT make an
// epidemic visible. A width-3 batch on 2026-07-13 took 76 fail-opens spread
// across its cycles and nothing summed them. RollupSpineFailOpens is that sum,
// plus the two breakdowns that turn a number into a diagnosis: which phase kept
// entering without its predecessor's handoff, and which individual cycles were
// noisy enough to escalate.

import "sort"

// SpineFailOpenRollup is a batch's spine fail-open summary. Total is the sum
// across the batch's cycles; ByPhase breaks it down by the phase that proceeded
// (nil-safe to read — always allocated); OverThresholdCycles lists the cycles
// whose OWN count exceeded the threshold, ascending. A clean batch rolls up to
// the zero-ish value: Total 0, empty ByPhase, no escalation.
//
// The json tags match the snake_case vocabulary of the surfaces that carry it
// (the loop summary's spine_fail_opens block, the dossier's own fields) — a
// rollup that serializes as Total/ByPhase inside an otherwise snake_case
// document reads as a different document's field.
type SpineFailOpenRollup struct {
	Total               int            `json:"total"`
	ByPhase             map[string]int `json:"by_phase"`
	OverThresholdCycles []int          `json:"over_threshold_cycles,omitempty"`
}

// RollupSpineFailOpens folds a batch's dossiers into one summary. A cycle
// escalates only on its OWN count — the batch total must never drag a quiet
// cycle over the line, or the escalation stops identifying anything. threshold
// <= 0 disables escalation (no cycle can exceed a non-positive bound without
// every cycle qualifying, which is noise, not a signal). Nil entries and a nil
// slice are tolerated: a rollup that panics on a partially-built batch is worse
// than no rollup.
func RollupSpineFailOpens(ds []*Dossier, threshold int) SpineFailOpenRollup {
	out := SpineFailOpenRollup{ByPhase: make(map[string]int)}
	for _, d := range ds {
		if d == nil {
			continue
		}
		for _, ev := range d.SpineFailOpens {
			out.Total++
			out.ByPhase[ev.Phase]++
		}
		if threshold > 0 && len(d.SpineFailOpens) > threshold {
			out.OverThresholdCycles = append(out.OverThresholdCycles, d.Cycle)
		}
	}
	sort.Ints(out.OverThresholdCycles)
	return out
}
