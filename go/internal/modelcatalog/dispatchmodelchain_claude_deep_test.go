package modelcatalog

import (
	"reflect"
	"testing"
)

func liveClaude(primary string, fallbacks []string) Catalog {
	return Catalog{CLIs: map[string]CLIEntry{
		"claude": {Source: SourceLive, TierModels: map[string]string{"deep": primary}, TierFallbacks: map[string][]string{"deep": fallbacks}},
	}}
}

func TestDispatchModelChain_ClaudeDeepWalksFableThenOpusThenSonnet(t *testing.T) {
	t.Parallel()
	want := []string{"fable", "opus", "sonnet"}
	cases := map[string]Catalog{
		"fallbacks repeat the primary": liveClaude("fable", []string{"fable", "opus", "sonnet"}),
		"fallbacks omit the primary":   liveClaude("fable", []string{"opus", "sonnet"}),
		"duplicates and blanks":        liveClaude("fable", []string{"opus", "", "opus", "sonnet", "fable"}),
	}
	for name, cat := range cases {
		if got := cat.DispatchModelChain("claude", "deep"); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: DispatchModelChain(claude, deep) = %v, want %v", name, got, want)
		}
	}
}

func TestDispatchModelChain_TheTierModelLeadsEvenWhenTheFallbackListOrdersItLater(t *testing.T) {
	t.Parallel()
	got := liveClaude("opus", []string{"fable", "opus", "sonnet"}).DispatchModelChain("claude", "deep")
	if want := []string{"opus", "fable", "sonnet"}; !reflect.DeepEqual(got, want) {
		t.Errorf("DispatchModelChain = %v, want the tier model first then the fallbacks in order: %v", got, want)
	}
}

func TestDispatchModelChain_ANonLiveClaudeEntryFallsBackToTheManifest(t *testing.T) {
	t.Parallel()
	for _, source := range []string{SourceDetect, "", "static"} {
		cat := liveClaude("fable", []string{"opus", "sonnet"})
		entry := cat.CLIs["claude"]
		entry.Source = source
		cat.CLIs["claude"] = entry
		if got := cat.DispatchModelChain("claude", "deep"); got != nil {
			t.Errorf("source %q: DispatchModelChain = %v, want nil so the manifest default applies", source, got)
		}
	}
}
