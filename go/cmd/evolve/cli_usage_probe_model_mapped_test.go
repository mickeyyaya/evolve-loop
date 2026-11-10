package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

func opusWeekExhaustedPane() string {
	return strings.Join([]string{
		"Current session",
		"████                                               7% used",
		"Resets 4h",
		"Current week (all models)",
		"████████████████████████████████                   64% used",
		"Resets 90h",
		"Current week (Opus)",
		"██████████████████████████████████████████████████ 100% used",
		"Resets 5h",
		"",
		"Esc to cancel",
	}, "\n")
}

func runClaudeUsageProber(t *testing.T, pane string) (*clihealth.Store, string) {
	t.Helper()
	root := t.TempDir()
	var log strings.Builder
	p := newUsageProber(usageProbeDirs{projectRoot: root, evolveDir: filepath.Join(root, ".evolve")}, []string{"claude"}, nil, &log)
	p.Probe = func(context.Context, string) (string, error) { return pane, nil }

	p.Run(context.Background())

	return clihealth.NewStore(root, nil), log.String()
}

func TestNewUsageProber_AnExhaustedFableWeekBenchesClaudeFableThroughTheProductionProber(t *testing.T) {
	t.Parallel()
	store, log := runClaudeUsageProber(t, usageFixture(t, "claude_usage_fable_exhausted.txt"))

	if _, ok := store.ActiveModels()[clihealth.Entry{Family: "claude", Model: "Fable"}.Key()]; !ok {
		t.Errorf("ActiveModels() = %v; the Fable week serves the deep and top tiers, so it must bench claude/Fable\n%s", store.ActiveModels(), log)
	}
	if len(store.Active()) != 0 {
		t.Errorf("Active() = %v; the family stays unbenched", store.Active())
	}
}

func TestNewUsageProber_AnExhaustedOpusWeekBenchesNoModelThroughTheProductionProber(t *testing.T) {
	t.Parallel()
	store, log := runClaudeUsageProber(t, opusWeekExhaustedPane())

	if models := store.ActiveModels(); len(models) != 0 {
		t.Errorf("ActiveModels() = %v; no tier launches opus, so its drained week benches nothing\n%s", models, log)
	}
	if !strings.Contains(log, "WARN") || !strings.Contains(log, "Opus") {
		t.Errorf("the drained window that no tier uses lost its WARN:\n%s", log)
	}
}
