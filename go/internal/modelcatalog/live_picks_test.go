package modelcatalog_test

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

func entry(source, deep string) modelcatalog.CLIEntry {
	return modelcatalog.CLIEntry{TierModels: map[string]string{"deep": deep}, Source: source}
}

func TestCatalog_LiveCountCountsOnlyTheLiveEntries(t *testing.T) {
	t.Parallel()
	cat := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
		"agy":    entry(modelcatalog.SourceLive, "gemini-3.8-flash"),
		"claude": entry(modelcatalog.SourceDetect, "opus"),
		"codex":  entry(modelcatalog.SourceLive, "gpt-5.6"),
	}}
	if got := cat.LiveCount(); got != 2 {
		t.Fatalf("LiveCount = %d, want 2", got)
	}
	if got := (modelcatalog.Catalog{}).LiveCount(); got != 0 {
		t.Fatalf("LiveCount of an empty catalog = %d, want 0", got)
	}
}

func TestKeepLivePicks_AFamilyWhoseListFailedKeepsItsPriorLivePick(t *testing.T) {
	t.Parallel()
	prior := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
		"agy":    entry(modelcatalog.SourceLive, "gemini-3.8-flash"),
		"claude": entry(modelcatalog.SourceDetect, "opus"),
		"codex":  entry(modelcatalog.SourceLive, "gpt-5.5"),
	}}
	next := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
		"agy":    entry(modelcatalog.SourceDetect, "gemini-3.5-flash"),
		"claude": entry(modelcatalog.SourceDetect, "sonnet"),
		"codex":  entry(modelcatalog.SourceLive, "gpt-5.6"),
	}}

	got, kept := modelcatalog.KeepLivePicks(prior, next)

	if !slices.Equal(kept, []string{"agy"}) {
		t.Fatalf("kept = %v, want [agy]", kept)
	}
	want := map[string]modelcatalog.CLIEntry{
		"agy":    prior.CLIs["agy"],
		"claude": next.CLIs["claude"],
		"codex":  next.CLIs["codex"],
	}
	for cli, w := range want {
		if g := got.CLIs[cli]; g.Source != w.Source || g.TierModels["deep"] != w.TierModels["deep"] {
			t.Errorf("%s = %+v, want %+v", cli, g, w)
		}
	}
	if next.CLIs["agy"].Source != modelcatalog.SourceDetect {
		t.Error("KeepLivePicks changed its input catalog")
	}
}
