package dossier

import "sort"

type SpineFailOpenRollup struct {
	Total               int            `json:"total"`
	ByPhase             map[string]int `json:"by_phase"`
	OverThresholdCycles []int          `json:"over_threshold_cycles,omitempty"`
}

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
