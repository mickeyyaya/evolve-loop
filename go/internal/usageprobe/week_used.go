package usageprobe

import (
	"strings"
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

func TierWeek(evolveDir, family, model string, now time.Time) (quotastate.UsageWindow, bool, error) {
	all, err := LoadObservations(evolveDir)
	if err != nil {
		return quotastate.UsageWindow{}, false, err
	}
	var familyWindow, modelWindow *quotastate.UsageWindow
	for _, obs := range all {
		if obs.Error != "" {
			continue
		}
		for i := range obs.Windows {
			w := obs.Windows[i]
			if w.Kind != quotastate.KindWeek || w.Family != family || (w.ResetsAt != nil && !now.Before(*w.ResetsAt)) {
				continue
			}
			switch {
			case w.Model == "":
				familyWindow = closerWindow(familyWindow, w)
			case model == w.Model || strings.HasPrefix(model, w.Model+" "):
				modelWindow = closerWindow(modelWindow, w)
			}
		}
	}
	if modelWindow != nil {
		return *modelWindow, true, nil
	}
	if familyWindow != nil {
		return *familyWindow, true, nil
	}
	return quotastate.UsageWindow{}, false, nil
}

func closerWindow(kept *quotastate.UsageWindow, w quotastate.UsageWindow) *quotastate.UsageWindow {
	switch {
	case kept == nil, len(w.Model) > len(kept.Model):
		return &w
	case len(w.Model) == len(kept.Model) && w.PercentUsed > kept.PercentUsed:
		return &w
	}
	return kept
}
