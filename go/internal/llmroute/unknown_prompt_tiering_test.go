package llmroute

import (
	"errors"
	"slices"
	"testing"
)

func TestTieringExit_OnlyAnUnknownPromptLeavesTheQuotaClass(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		exit  int
		cause string
		want  int
	}{
		{85, "unknown_prompt", ExitUnknownPromptNotQuota},
		{85, "rate_limit", 85},
		{85, "model_unsupported", 85},
		{85, "", 85},
		{81, "unknown_prompt", 81},
		{0, "", 0},
	} {
		if got := TieringExit(tc.exit, tc.cause); got != tc.want {
			t.Errorf("TieringExit(%d, %q) = %d, want %d", tc.exit, tc.cause, got, tc.want)
		}
	}
}

func TestDispatchTiered_AnUnknownPromptAtEveryCLIIsNoQuotaWall(t *testing.T) {
	t.Parallel()
	plan := Plan{Candidates: []string{"codex-tmux", "claude-tmux"}, Triggers: DefaultTriggers(), Tiers: []string{"deep", "balanced"}}
	stepped := false

	res := DispatchTiered(plan, func(string, string) (int, error) {
		return TieringExit(85, "unknown_prompt"), errors.New("bridge: exit 85")
	}, func(string, string) { stepped = true })

	if !slices.Equal(res.Attempts, []string{"codex-tmux@deep", "claude-tmux@deep"}) || stepped || res.Walled {
		t.Fatalf("attempts %v stepped %v walled %v: an undelivered prompt still falls back to the next CLI, but it is no quota wall, so the tier holds", res.Attempts, stepped, res.Walled)
	}
}

func TestPlanTriggersFallback_AnUnknownPromptFollowsTheTriggerOfExit85(t *testing.T) {
	t.Parallel()
	if !(Plan{Triggers: []int{85}}).TriggersFallback(ExitUnknownPromptNotQuota) {
		t.Error("a plan that falls back on exit 85 must fall back on an undelivered prompt")
	}
	if (Plan{Triggers: []int{81}}).TriggersFallback(ExitUnknownPromptNotQuota) {
		t.Error("a plan without exit 85 must not fall back on an undelivered prompt")
	}
}
