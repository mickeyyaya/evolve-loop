package dossier

import "sort"

// SpineFailOpenRollup is a batch's spine fail-open summary: the total, the
// count per phase that proceeded, and the cycles whose own count exceeded the
// threshold, ascending.
type SpineFailOpenRollup struct {
	Total               int            `json:"total"`
	ByPhase             map[string]int `json:"by_phase"`
	OverThresholdCycles []int          `json:"over_threshold_cycles,omitempty"`
}

// RollupSpineFailOpens folds a batch's dossiers into one summary. A cycle
// escalates only on its own count; threshold <= 0 disables escalation, and nil
// dossiers are skipped.
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
