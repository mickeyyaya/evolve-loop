package usageprobe

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

const fullWindowPct = 100

func WeekUsed(evolveDir string, now time.Time) (map[string]float64, error) {
	all, err := LoadObservations(evolveDir)
	if err != nil {
		return nil, err
	}
	used := map[string]float64{}
	for _, obs := range all {
		if obs.Error != "" {
			continue
		}
		for _, w := range obs.Windows {
			if pct, live := familyWeekPct(w, now); live && pct >= used[w.Family] {
				used[w.Family] = pct
			}
		}
	}
	return used, nil
}

func familyWeekPct(w quotastate.UsageWindow, now time.Time) (float64, bool) {
	if w.Kind != quotastate.KindWeek || w.Family == "" || w.Model != "" {
		return 0, false
	}
	if w.ResetsAt != nil && !now.Before(*w.ResetsAt) {
		return 0, false
	}
	if w.ExhaustsFamily() {
		return fullWindowPct, true
	}
	return w.PercentUsed, true
}
