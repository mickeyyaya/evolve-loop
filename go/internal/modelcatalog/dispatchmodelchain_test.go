package modelcatalog

import (
	"reflect"
	"testing"
)

func TestDispatchModelChain_IsThePrimaryThenTheFallbacksDedupedForALiveEntryOnly(t *testing.T) {
	t.Parallel()
	cat := Catalog{CLIs: map[string]CLIEntry{
		"claude": {Source: SourceLive, TierModels: map[string]string{"deep": "opus"}, TierFallbacks: map[string][]string{"deep": {"opus", "", "sonnet", "sonnet"}}},
		"codex":  {Source: SourceDetect, TierModels: map[string]string{"deep": "gpt-5.6-sol"}},
		"agy":    {Source: SourceLive, TierFallbacks: map[string][]string{"deep": {"Gemini 3.1 Pro (High)"}}},
	}}
	cases := []struct {
		cli, tier string
		want      []string
	}{
		{"claude", "deep", []string{"opus", "sonnet"}},
		{"claude", "fast", nil},
		{"codex", "deep", nil},
		{"agy", "deep", []string{"Gemini 3.1 Pro (High)"}},
		{"ollama", "deep", nil},
	}
	for _, c := range cases {
		if got := cat.DispatchModelChain(c.cli, c.tier); !reflect.DeepEqual(got, c.want) {
			t.Errorf("DispatchModelChain(%s, %s) = %v, want %v", c.cli, c.tier, got, c.want)
		}
	}
}
