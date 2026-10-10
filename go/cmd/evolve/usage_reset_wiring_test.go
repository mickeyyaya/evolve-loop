package main

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageevidence"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

func screens(by map[string]usageprobe.Evidence) func(context.Context, string, time.Time) usageprobe.Evidence {
	return func(_ context.Context, driver string, _ time.Time) usageprobe.Evidence { return by[driver] }
}

func resetIn(now time.Time, d time.Duration, exhausted bool) quotastate.UsageWindow {
	at := now.Add(d)
	return quotastate.UsageWindow{Kind: "week", ResetsAt: &at, Exhausted: exhausted}
}

func TestUsageResetFrom_TheFirstWalledFamilyBackSetsTheReset(t *testing.T) {
	now := time.Date(2026, 10, 9, 18, 42, 13, 0, time.UTC)
	explain := screens(map[string]usageprobe.Evidence{
		"claude-tmux":     {Verdict: usageprobe.VerdictExhausted, Windows: []quotastate.UsageWindow{resetIn(now, 3*time.Hour, true), resetIn(now, 9*time.Hour, true), resetIn(now, time.Hour, false)}},
		"agy-claude-tmux": {Verdict: usageprobe.VerdictExhausted, Windows: []quotastate.UsageWindow{resetIn(now, 89*time.Hour, true)}},
	})

	got, ok := usageResetFrom(explain)([]string{"claude", "agy-claude"}, now)

	if !ok || !got.Equal(now.Add(9*time.Hour)) {
		t.Errorf("reset=%v ok=%v, want %v: a family is back when its last exhausted window resets, and the first family back wins", got, ok, now.Add(9*time.Hour))
	}
}

func TestUsageResetFrom_AScreenThatProvesNoExhaustionGivesNoReset(t *testing.T) {
	now := time.Date(2026, 10, 9, 18, 42, 13, 0, time.UTC)
	explain := screens(map[string]usageprobe.Evidence{
		"claude-tmux": {Verdict: usageprobe.VerdictHealthy, Windows: []quotastate.UsageWindow{resetIn(now, time.Hour, false)}},
		"agy-tmux":    {Verdict: usageprobe.VerdictExhausted, Windows: []quotastate.UsageWindow{{Kind: "week", Exhausted: true}}},
	})

	if got, ok := usageResetFrom(explain)([]string{"claude", "agy"}, now); ok {
		t.Errorf("reset=%v, want none: a healthy screen and an exhausted window with no reset time prove no wake time", got)
	}
	if usageResetFrom(nil) != nil {
		t.Error("usageResetFrom(nil) must be nil, so the checkpoint skips the query when no explainer is wired")
	}
}

func TestCheckpointUsageReset_IsWiredToTheProductionUsageQuery(t *testing.T) {
	now := time.Date(2026, 10, 9, 18, 42, 13, 0, time.UTC)
	saved := usageEvidenceFn
	t.Cleanup(func() { usageEvidenceFn = saved })
	var root string
	usageEvidenceFn = func(projectRoot, _ string, _ io.Writer) usageevidence.Explain {
		root = projectRoot
		return screens(map[string]usageprobe.Evidence{"claude-tmux": {Verdict: usageprobe.VerdictExhausted, Windows: []quotastate.UsageWindow{resetIn(now, 2*time.Hour, true)}}})
	}

	got, ok := checkpoint.UsageReset("/proj", []string{"claude"}, now)

	if !ok || !got.Equal(now.Add(2*time.Hour)) || root != "/proj" {
		t.Errorf("reset=%v ok=%v root=%q, want the queried reset for /proj: the quota checkpoint asks the CLI before it gives up", got, ok, root)
	}
	usageEvidenceFn = func(string, string, io.Writer) usageevidence.Explain { return nil }
	if _, ok := checkpoint.UsageReset("/proj", []string{"claude"}, now); ok {
		t.Error("an explainer factory that builds nothing must report no reset")
	}
	usageEvidenceFn = nil
	if _, ok := checkpoint.UsageReset("/proj", []string{"claude"}, now); ok {
		t.Error("with no usage explainer wired, the hook must report no reset")
	}
}
