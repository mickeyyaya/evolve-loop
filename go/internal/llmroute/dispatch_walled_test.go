package llmroute

import (
	"errors"
	"testing"
)

func TestDispatchTiered_AnExhaustedWalkThatMetAWallIsWalled(t *testing.T) {
	plan := Plan{Candidates: []string{"codex-tmux", "claude-tmux"}, Triggers: []int{80, 81, 85, 124, 127}, Model: "balanced", Tiers: []string{"balanced"}}
	for name, tc := range map[string]struct {
		exits  map[string]int
		walled bool
	}{
		"a wall then a stall":       {map[string]int{"codex-tmux": 85, "claude-tmux": 81}, true},
		"a stall then a wall":       {map[string]int{"codex-tmux": 81, "claude-tmux": 85}, true},
		"two stalls and no wall":    {map[string]int{"codex-tmux": 81, "claude-tmux": 81}, false},
		"a wall then a non-trigger": {map[string]int{"codex-tmux": 85, "claude-tmux": 3}, false},
		"a wall then a success":     {map[string]int{"codex-tmux": 85, "claude-tmux": 0}, false},
	} {
		res := DispatchTiered(plan, func(cli, _ string) (int, error) {
			if code := tc.exits[cli]; code != 0 {
				return code, errors.New("launch failed")
			}
			return 0, nil
		}, nil)
		if res.Walled != tc.walled {
			t.Errorf("%s: Walled = %v, want %v (attempts %v)", name, res.Walled, tc.walled, res.Attempts)
		}
	}
}

func TestDispatchTiered_AWallOnAHigherTierStaysWalledAfterTheStepDown(t *testing.T) {
	plan := Plan{Candidates: []string{"codex-tmux", "claude-tmux"}, Triggers: []int{80, 81, 85, 124, 127}, Model: "deep", Tiers: []string{"deep", "balanced"}}
	exits := map[string]int{"codex-tmux@deep": 85, "claude-tmux@deep": 85, "codex-tmux@balanced": 81, "claude-tmux@balanced": 81}

	res := DispatchTiered(plan, func(cli, tier string) (int, error) { return exits[cli+"@"+tier], errors.New("launch failed") }, nil)

	if !res.Walled || len(res.Attempts) != 4 {
		t.Fatalf("a wall met on the tier above keeps the walk walled: %+v", res)
	}
}
