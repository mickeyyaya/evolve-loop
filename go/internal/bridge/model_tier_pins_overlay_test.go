package bridge

import (
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

func loadClaudeOverAnOpusCatalog(t *testing.T, available ...string) Manifest {
	t.Helper()
	dir := t.TempDir()
	prev := modelCatalogDirFn
	modelCatalogDirFn = func() string { return dir }
	t.Cleanup(func() { modelCatalogDirFn = prev })
	opus := map[string]string{"fast": "haiku", "balanced": "sonnet", "deep": "opus", "top": "opus"}
	cat := modelcatalog.BuildFromSnapshots([]modelcatalog.CLISnapshot{{
		CLI: "claude", Ready: true, TierModels: opus,
		Available: available, Source: modelcatalog.SourceLive, CandidatesHash: "sha256:before-the-pin",
	}}, time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC))
	if err := modelcatalog.Write(dir, cat); err != nil {
		t.Fatalf("Write: %v", err)
	}
	m, err := LoadManifest("claude-tmux")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	return m
}

func TestLoadManifest_ACatalogFromBeforeThePinDoesNotChangeThePin(t *testing.T) {
	m := loadClaudeOverAnOpusCatalog(t, "opus", "sonnet", "haiku")

	if m.ModelTierMap["deep"] != "opus" {
		t.Fatalf("deep = %q, want the catalog's opus: the CLI does not offer the pinned model", m.ModelTierMap["deep"])
	}
	want := map[string]string{"deep": "fable", "top": "fable"}
	if got := m.TierPins(); !reflect.DeepEqual(got, want) {
		t.Errorf("TierPins() = %v, want %v: a pin is the declared model, not the model of the last catalog", got, want)
	}
	pins := m.TierPins()
	pins["deep"] = "changed"
	if got := m.TierPins(); got["deep"] != "fable" {
		t.Errorf("a caller changed the pins of the manifest: %v", got)
	}
}

func TestLoadManifest_AnOlderLiveCatalogThatOffersThePinnedModelLaunchesThePin(t *testing.T) {
	m := loadClaudeOverAnOpusCatalog(t, "fable", "opus", "sonnet", "haiku")

	want := map[string]string{"fast": "haiku", "balanced": "sonnet", "deep": "fable", "top": "fable"}
	for tier, model := range want {
		if m.ModelTierMap[tier] != model {
			t.Errorf("%s = %q, want %q: an offered pin decides the launch model and moves no other tier", tier, m.ModelTierMap[tier], model)
		}
	}
}
