package usageprobe

import (
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

type Hold struct {
	Held       bool
	Unreadable bool
	Percent    float64
	Limit      float64
	Resets     string
	Detail     string
}

func (h Hold) Message(evolveDir string) string {
	if h.Unreadable {
		return fmt.Sprintf("usage hold: the usage reading is unreadable (%s: %s), so it is not under the limit of %g%%", WindowsPath(evolveDir), h.Detail, h.Limit)
	}
	return fmt.Sprintf("usage hold: the usage reading %g%% reached the limit of %g%%; the window resets %s", h.Percent, h.Limit, h.Resets)
}

func CheckHold(evolveDir string, limit float64, now time.Time) Hold {
	h := Hold{Limit: limit}
	if limit <= 0 {
		return h
	}
	all, err := LoadObservations(evolveDir)
	if err != nil {
		h.Held, h.Unreadable, h.Detail = true, true, err.Error()
		return h
	}
	live, failed := false, ""
	for _, obs := range all {
		if obs.Error != "" {
			failed = fmt.Sprintf("%s probe: %s", obs.CLI, obs.Error)
			continue
		}
		for _, w := range obs.Windows {
			pct, ok := familyWeekPct(w, now)
			if !ok {
				continue
			}
			live = true
			if pct >= h.Percent {
				h.Percent, h.Resets = pct, resetOf(w)
			}
		}
	}
	if !live && failed != "" {
		h.Held, h.Unreadable, h.Detail = true, true, failed
		return h
	}
	h.Held = live && h.Percent >= limit
	return h
}

func resetOf(w quotastate.UsageWindow) string {
	switch {
	case w.ResetsAt != nil && w.ResetsText != "":
		return fmt.Sprintf("%s (%s)", w.ResetsText, w.ResetsAt.UTC().Format(time.RFC3339))
	case w.ResetsAt != nil:
		return w.ResetsAt.UTC().Format(time.RFC3339)
	case w.ResetsText != "":
		return w.ResetsText
	}
	return "at an unknown time"
}
