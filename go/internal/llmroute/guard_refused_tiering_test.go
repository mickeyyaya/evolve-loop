package llmroute

import (
	"errors"
	"slices"
	"testing"
)

func TestTieringExit_AGuardRefusedStallIsTerminal(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		exit  int
		cause string
		want  int
	}{
		{81, CauseGuardRefused, ExitGuardRefused},
		{81, "artifact_timeout", 81},
		{81, "pane_lost", 81},
		{81, "", 81},
		{85, CauseGuardRefused, 85},
		{80, CauseGuardRefused, 80},
	} {
		if got := TieringExit(tc.exit, tc.cause); got != tc.want {
			t.Errorf("TieringExit(%d, %q) = %d, want %d", tc.exit, tc.cause, got, tc.want)
		}
	}
}

func TestPlanTriggersFallback_NoTriggerListHoldsTheGuardRefusal(t *testing.T) {
	t.Parallel()
	if (Plan{Triggers: DefaultTriggers()}).TriggersFallback(ExitGuardRefused) {
		t.Error("the default triggers must not walk past a guard refusal")
	}
	if (Plan{Triggers: []int{81, 85}}).TriggersFallback(ExitGuardRefused) {
		t.Error("a plan that falls back on exit 81 must still stop on a guard refusal")
	}
}

func TestDispatchTiered_AGuardRefusalRunsOneAttempt(t *testing.T) {
	t.Parallel()
	plan := Plan{Candidates: []string{"codex-tmux", "claude-tmux", "agy-tmux"}, Triggers: DefaultTriggers(), Tiers: []string{"deep", "balanced"}}
	stepped := false

	res := DispatchTiered(plan, func(string, string) (int, error) {
		return TieringExit(81, CauseGuardRefused), errors.New("bridge: launch exit=81")
	}, func(string, string) { stepped = true })

	if !slices.Equal(res.Attempts, []string{"codex-tmux@deep"}) || stepped || res.Walled || res.Err == nil {
		t.Fatalf("attempts %v stepped %v walled %v err %v: one attempt, no step down, no wall, the error kept", res.Attempts, stepped, res.Walled, res.Err)
	}
}

func TestDispatchTiered_AStallWithoutADenialKeepsTheChain(t *testing.T) {
	t.Parallel()
	plan := Plan{Candidates: []string{"codex-tmux", "claude-tmux", "agy-tmux"}, Triggers: DefaultTriggers()}

	res := DispatchTiered(plan, func(string, string) (int, error) {
		return TieringExit(81, "artifact_timeout"), errors.New("bridge: launch exit=81")
	}, nil)

	if len(res.Attempts) != 3 {
		t.Fatalf("attempts %v: a stall with no denial walks every family", res.Attempts)
	}
}
