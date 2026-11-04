package llmroute

import (
	"errors"
	"slices"
	"testing"
)

func TestTieringExit_AGuardRefusalLeavesEveryFallbackClass(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		exit  int
		cause string
		want  int
	}{
		{81, CauseGuardRefused, ExitGuardRefusedTerminal},
		{81, "review_pause", 81},
		{85, CauseGuardRefused, 85},
	} {
		if got := TieringExit(tc.exit, tc.cause); got != tc.want {
			t.Errorf("TieringExit(%d, %q) = %d, want %d", tc.exit, tc.cause, got, tc.want)
		}
	}
	if (Plan{Triggers: DefaultTriggers()}).TriggersFallback(ExitGuardRefusedTerminal) {
		t.Fatal("no fallback trigger may name the terminal guard refusal")
	}
}

func TestDispatchTiered_AGuardRefusalStopsTheChainAtTheFirstFamily(t *testing.T) {
	t.Parallel()
	plan := Plan{Candidates: []string{"agy-tmux", "claude-tmux", "codex-tmux"}, Triggers: DefaultTriggers(), Tiers: []string{"deep", "balanced"}}

	res := DispatchTiered(plan, func(string, string) (int, error) {
		return TieringExit(81, CauseGuardRefused), errors.New("bridge: launch exit=81")
	}, nil)

	if !slices.Equal(res.Attempts, []string{"agy-tmux@deep"}) || res.Err == nil {
		t.Fatalf("attempts %v err %v: a guard denial cannot depend on the family, so the chain must stop after one attempt", res.Attempts, res.Err)
	}
}

func TestDispatchTiered_AStallWithNoDenialKeepsTheFallbackChain(t *testing.T) {
	t.Parallel()
	plan := Plan{Candidates: []string{"agy-tmux", "claude-tmux"}, Triggers: DefaultTriggers(), Tiers: []string{"deep"}}

	res := DispatchTiered(plan, func(string, string) (int, error) {
		return TieringExit(81, "review_pause"), errors.New("bridge: launch exit=81")
	}, nil)

	if !slices.Equal(res.Attempts, []string{"agy-tmux@deep", "claude-tmux@deep"}) {
		t.Fatalf("attempts %v: a stall with no denial must still walk each family", res.Attempts)
	}
}
