package main

import (
	"context"
	"io"
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

func TestNewUsageProber_AnExhaustedFableWeekBenchesNoModelThroughTheProductionProber(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pane := usageFixture(t, "claude_usage_fable_exhausted.txt")
	var log strings.Builder
	p := newUsageProber(usageProbeDirs{projectRoot: root, evolveDir: filepath.Join(root, ".evolve")}, []string{"claude"}, nil, &log)
	p.Probe = func(context.Context, string) (string, error) { return pane, nil }

	p.Run(context.Background())

	store := clihealth.NewStore(root, nil)
	if models := store.ActiveModels(); len(models) != 0 {
		t.Errorf("ActiveModels() = %v; the Fable word is not in model_match, so nothing is benched\n%s", models, log.String())
	}
	if !strings.Contains(log.String(), "WARN") || !strings.Contains(log.String(), "Fable") {
		t.Errorf("the unmapped drained window lost its WARN:\n%s", log.String())
	}
}

func TestNewUsageProber_AnExhaustedOpusWeekBenchesClaudeOpusThroughTheProductionProber(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pane := opusWeekExhaustedPane()
	p := newUsageProber(usageProbeDirs{projectRoot: root, evolveDir: filepath.Join(root, ".evolve")}, []string{"claude"}, nil, io.Discard)
	p.Probe = func(context.Context, string) (string, error) { return pane, nil }

	p.Run(context.Background())

	store := clihealth.NewStore(root, nil)
	if _, ok := store.ActiveModels()[clihealth.Entry{Family: "claude", Model: "Opus"}.Key()]; !ok {
		t.Errorf("ActiveModels() = %v; the mapped Opus week must bench claude/Opus", store.ActiveModels())
	}
	if len(store.Active()) != 0 {
		t.Errorf("Active() = %v; the family stays unbenched", store.Active())
	}
}
