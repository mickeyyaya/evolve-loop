package llmroute

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestResolveTriggers_EveryPlanOwnsItsDefaultTriggers(t *testing.T) {
	want := []int{80, 81, 85, 124, 127}
	cases := map[string]*profiles.Profile{
		"nil profile":        nil,
		"nil trigger list":   {CLI: "codex-tmux"},
		"empty trigger list": {CLIFallbackOnExit: []int{}},
		"model-tier-only":    {ModelTierDefault: "balanced"},
	}
	for label, prof := range cases {
		t.Run(label, func(t *testing.T) {
			builders := map[string]func() Plan{
				"Resolve":  func() Plan { return Resolve("scout", "scout", "balanced", nil, prof, nil, nil) },
				"ChainFor": func() Plan { return ChainFor("claude-tmux", prof) },
			}
			for name, build := range builders {
				edited := build()
				edited.Triggers[0] = 999
				_ = append(edited.Triggers[:2], 4242)
				if got := build().Triggers; !slices.Equal(got, want) {
					t.Errorf("%s: a later Plan's Triggers=%v after editing an earlier Plan's, want %v", name, got, want)
				}
				if got := DefaultTriggers(); !slices.Equal(got, want) {
					t.Errorf("%s: DefaultTriggers()=%v after editing a Plan's Triggers, want %v", name, got, want)
				}
			}
		})
	}
}

func TestResolveTriggers_AProfileListWinsAndIsNeverAliased(t *testing.T) {
	own := []int{7, 8}
	plan := Resolve("scout", "scout", "balanced", nil, &profiles.Profile{CLIFallbackOnExit: own}, nil, nil)
	if !slices.Equal(plan.Triggers, []int{7, 8}) {
		t.Fatalf("Triggers=%v, want the profile's own [7 8]", plan.Triggers)
	}
	plan.Triggers[0] = 999
	if own[0] != 7 {
		t.Errorf("editing the Plan rewrote the profile's cli_fallback_on_exit to %v", own)
	}
}
