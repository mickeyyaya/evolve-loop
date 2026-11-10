package usageevidence

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

func claudeWeekPane(modelWord string) string {
	return strings.Join([]string{
		"Current session",
		"████                                               7% used",
		"Resets 4h",
		"Current week (all models)",
		"████████████████████████████████                   64% used",
		"Resets 90h",
		"Current week (" + modelWord + ")",
		"██████████████████████████████████████████████████ 100% used",
		"Resets 5h",
		"",
		"Esc to cancel",
	}, "\n")
}

func explainClaudeThroughNew(t *testing.T, pane string) (root, log string) {
	t.Helper()
	root = t.TempDir()
	var buf bytes.Buffer
	explain := New(Options{
		ProjectRoot: root, EvolveDir: filepath.Join(root, ".evolve"), Act: true, Log: &buf,
		Probe: func(context.Context, string) (string, error) { return pane, nil },
	})

	explain(context.Background(), "claude-tmux", time.Time{})

	return root, buf.String()
}

func TestNew_AnExhaustedOpusWeekBenchesNoModelThroughTheEvidencePath(t *testing.T) {
	root, log := explainClaudeThroughNew(t, claudeWeekPane("Opus"))

	store := clihealth.NewStore(root, nil)
	if models := store.ActiveModels(); len(models) != 0 {
		t.Errorf("ActiveModels() = %v; no tier launches opus, so the evidence path benches nothing\n%s", models, log)
	}
	if active := store.Active(); len(active) != 0 {
		t.Errorf("Active() = %v; an unmapped model word never benches the family", active)
	}
	if !strings.Contains(log, "WARN") || !strings.Contains(log, "Opus") {
		t.Errorf("the unmapped drained window lost its WARN:\n%s", log)
	}
	if strings.Contains(log, "benched claude/Opus") {
		t.Errorf("the log claims a bench that was never written:\n%s", log)
	}
}

func TestNew_AnExhaustedFableWeekStillBenchesClaudeFableThroughTheEvidencePath(t *testing.T) {
	root, log := explainClaudeThroughNew(t, claudeWeekPane("Fable"))

	store := clihealth.NewStore(root, nil)
	if _, ok := store.ActiveModels()[clihealth.Entry{Family: "claude", Model: "Fable"}.Key()]; !ok {
		t.Errorf("ActiveModels() = %v; the mapped Fable week must bench claude/Fable\n%s", store.ActiveModels(), log)
	}
	if len(store.Active()) != 0 {
		t.Errorf("Active() = %v; the family stays unbenched", store.Active())
	}
}

func bareProberOverTheSharedReader(t *testing.T, pane string) (root, log string) {
	t.Helper()
	root = t.TempDir()
	var buf bytes.Buffer
	prober := &usageprobe.Prober{
		Families: []string{"claude"},
		Probe:    func(context.Context, string) (string, error) { return pane, nil },
		Classify: func(string, string) bool { return false },
		Windows:  ReadWindowsNow,
		Store:    clihealth.NewStore(root, nil),
		Log:      &buf,
	}

	prober.Run(context.Background())

	return root, buf.String()
}

func TestReadWindowsNow_AnUnmappedScopeWordBenchesNoModelWithoutAnyOptionalSeam(t *testing.T) {
	root, log := bareProberOverTheSharedReader(t, claudeWeekPane("Opus"))

	if models := clihealth.NewStore(root, nil).ActiveModels(); len(models) != 0 {
		t.Errorf("ActiveModels() = %v; a Prober that only reads windows through the shared reader must not bench an unmapped word\n%s", models, log)
	}
	if !strings.Contains(log, "WARN") || !strings.Contains(log, "Opus") {
		t.Errorf("the unmapped drained window lost its WARN:\n%s", log)
	}
}

func TestReadWindowsNow_AMappedScopeWordStillBenchesItsModelWithoutAnyOptionalSeam(t *testing.T) {
	root, log := bareProberOverTheSharedReader(t, claudeWeekPane("Fable"))

	if _, ok := clihealth.NewStore(root, nil).ActiveModels()[clihealth.Entry{Family: "claude", Model: "Fable"}.Key()]; !ok {
		t.Errorf("the mapped Fable week must bench claude/Fable through the shared reader alone\n%s", log)
	}
}

func TestReadWindowsNow_ReadsTheSameModelMapTheCanaryAndTheRouterUse(t *testing.T) {
	for _, word := range []string{"Fable", "Opus", "Sonnet", "Haiku"} {
		_, mapped := bridge.UsageModelFor("claude-tmux", word)
		windows := ReadWindowsNow("claude", claudeWeekPane(word))
		var exhausted int
		for _, w := range windows {
			if w.Exhausted && w.Model == word {
				exhausted++
			}
		}
		if exhausted != 1 {
			t.Fatalf("%s: the reader returned %d exhausted windows for the word, want 1: %+v", word, exhausted, windows)
		}
		root := t.TempDir()
		var buf bytes.Buffer
		(&usageprobe.Prober{
			Families: []string{"claude"},
			Probe:    func(context.Context, string) (string, error) { return claudeWeekPane(word), nil },
			Classify: func(string, string) bool { return false },
			Windows:  ReadWindowsNow,
			Store:    clihealth.NewStore(root, nil),
			Log:      &buf,
		}).Run(context.Background())
		benched := len(clihealth.NewStore(root, nil).ActiveModels()) == 1
		if benched != mapped {
			t.Errorf("%s: benched=%v but the manifest maps it=%v; the reader and the manifest model_match must agree\n%s", word, benched, mapped, buf.String())
		}
	}
}
