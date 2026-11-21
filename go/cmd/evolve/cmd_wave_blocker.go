package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cycleblocker"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

type waveBlocker struct {
	Cycle   int     `json:"cycle"`
	Blocker string  `json:"blocker"`
	Minutes float64 `json:"minutes"`
}

func waveBlockers(evolveDir string, cycles []wave.Cycle, warnings []string) ([]waveBlocker, map[string]int, []string) {
	pct, warn := blockerWaitSharePct(evolveDir)
	if warn != "" {
		warnings = append(warnings, warn)
	}
	blockers, counts := []waveBlocker{}, map[string]int{}
	for _, c := range cycles {
		b, ok, warn := cycleBlocker(evolveDir, c, pct)
		if warn != "" {
			warnings = append(warnings, warn)
		}
		if ok {
			blockers = append(blockers, b)
			if !c.Shipped || b.Blocker != cycleblocker.None {
				counts[blockerKind(b.Blocker)]++
			}
		}
	}
	return blockers, counts, warnings
}

func blockerWaitSharePct(evolveDir string) (int, string) {
	pol, err := policy.Load(filepath.Join(evolveDir, paths.PolicyFile))
	if err != nil {
		return policy.DefaultBlockerWaitSharePct, fmt.Sprintf("blocker attribution: policy unreadable (%v): the default wait share applies", err)
	}
	return pol.WorkflowConfig().BlockerAttribution.WaitSharePct, ""
}

func cycleBlocker(evolveDir string, c wave.Cycle, pct int) (waveBlocker, bool, string) {
	switch {
	case c.Shipped:
		a := cycleblocker.Attribute(cycleblocker.Input{Shipped: true, Verdict: c.LLMVerdict, WallMinutes: wallMinutes(c), Paused: c.QuotaPauses > 0}, pct)
		return waveBlocker{Cycle: c.ID, Blocker: a.Blocker, Minutes: a.Minutes}, true, ""
	case c.Verdict == cyclestate.VerdictFAIL:
		rec, warn := failRecord(evolveDir, c.ID)
		return waveBlocker{Cycle: c.ID, Blocker: rec.Blocker, Minutes: rec.Minutes}, true, warn
	}
	return waveBlocker{}, false, ""
}

func wallMinutes(c wave.Cycle) float64 {
	if c.StartedAt.IsZero() || c.EndedAt.IsZero() {
		return 0
	}
	return c.EndedAt.Sub(c.StartedAt).Minutes()
}

func failRecord(evolveDir string, id int) (cycleblocker.Record, string) {
	path := paths.BlockerRecordPath(evolveDir, id)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cycleblocker.Record{Blocker: cycleblocker.None}, ""
	}
	if err == nil {
		var rec cycleblocker.Record
		if rec, err = cycleblocker.DecodeRecord(raw); err == nil {
			return rec, ""
		}
	}
	return cycleblocker.Record{Blocker: cycleblocker.None}, fmt.Sprintf("blocker record of cycle %d: %v", id, err)
}

func blockerKind(blocker string) string {
	switch {
	case strings.HasPrefix(blocker, cycleblocker.CheckPrefix):
		return "check"
	case strings.HasPrefix(blocker, cycleblocker.ClassPrefix):
		return "class"
	}
	return blocker
}

func blockerText(blockers []waveBlocker, id int) string {
	for _, b := range blockers {
		if b.Cycle == id && b.Blocker != cycleblocker.None {
			return fmt.Sprintf(", blocker %s (%.1f min)", b.Blocker, b.Minutes)
		}
	}
	return ""
}
