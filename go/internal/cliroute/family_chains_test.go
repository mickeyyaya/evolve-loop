package cliroute_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestFamilyChains_ListsEveryDeclaredChainAsFamilies(t *testing.T) {
	t.Parallel()
	block := &policy.CLIRouting{
		CLIs:    []string{"agy", "agy-claude", "claude"},
		Default: []string{"agy", " agy-claude-tmux", "claude"},
		Work:    map[string][]string{"plan": {"claude-tmux", "claude-p"}},
		Agents: map[string]policy.AgentRule{
			"builder": {CLI: []string{"codex"}},
			"scout":   {Effort: "low"},
		},
		Tiers: map[string]policy.TierRule{
			"top":  {CLIs: []string{"agy-claude", "claude"}},
			"deep": {CLIs: []string{"agy-claude", "claude"}},
			"fast": {Effort: "low"},
		},
	}

	got := cliroute.FamilyChains(block)

	want := [][]string{
		{"agy", "agy-claude", "claude"},
		{"claude"},
		{"codex"},
		{"agy-claude", "claude"},
		{"agy-claude", "claude"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FamilyChains = %v, want %v", got, want)
	}
}

func TestFamilyChains_NoTableIsNil(t *testing.T) {
	t.Parallel()
	if got := cliroute.FamilyChains(nil); got != nil {
		t.Errorf("FamilyChains(nil) = %v, want nil: no table means the chains are unknown", got)
	}
}

func TestFamilyChains_SkipsBlankEntriesAndDropsEmptyChains(t *testing.T) {
	t.Parallel()
	block := &policy.CLIRouting{
		Default: []string{"", "  ", "claude"},
		Work:    map[string][]string{"plan": {" ", ""}},
	}

	got := cliroute.FamilyChains(block)

	want := [][]string{{"claude"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FamilyChains = %v, want %v: a blank entry is no family and a chain of only blanks is no chain", got, want)
	}
}
