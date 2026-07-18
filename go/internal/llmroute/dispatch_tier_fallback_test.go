package llmroute

import (
	"errors"
	"fmt"
	"testing"
)

// tierTestPlan is the shared fixture: two-CLI chain, default triggers, a
// deep→balanced tier chain.
func tierTestPlan() Plan {
	return Plan{
		Candidates: []string{"claude-tmux", "codex-tmux"},
		Triggers:   []int{80, 81, 85, 124, 127},
		Model:      "deep",
		Tiers:      []string{"deep", "balanced"},
	}
}

func TestTierChain(t *testing.T) {
	cases := []struct {
		name          string
		resolved, min string
		want          []string
	}{
		{"one step down", "deep", "balanced", []string{"deep", "balanced"}},
		{"two steps down", "top", "balanced", []string{"top", "deep", "balanced"}},
		{"already at floor", "balanced", "balanced", []string{"balanced"}},
		{"floor above balanced", "top", "deep", []string{"top", "deep"}},
		{"unclassifiable resolved", "weird-model-xyz", "balanced", []string{"weird-model-xyz"}},
		{"empty min uses universal balanced floor", "deep", "", []string{"deep", "balanced"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := TierChain(c.resolved, c.min); fmt.Sprint(got) != fmt.Sprint(c.want) {
				t.Errorf("TierChain(%q, %q) = %v, want %v", c.resolved, c.min, got, c.want)
			}
		})
	}
}

// TestDispatchTiered_StepsDownAfterQuotaExhaustedChain is the scout
// verifiableBy regression test: given a Plan whose CLI chain all return exit
// 85 at tier "deep", DispatchTiered retries the same CLI chain at
// "balanced" before declaring quota-exhausted, and never steps below the
// chain's floor.
func TestDispatchTiered_StepsDownAfterQuotaExhaustedChain(t *testing.T) {
	var attempts []string
	var steps []string
	res := DispatchTiered(tierTestPlan(), func(cli, tier string) (int, error) {
		attempts = append(attempts, cli+"@"+tier)
		if tier == "deep" {
			return 85, errors.New("quota exhausted")
		}
		return 0, nil
	}, func(from, to string) { steps = append(steps, from+"->"+to) })

	if res.Err != nil {
		t.Fatalf("expected success at balanced, got err: %v", res.Err)
	}
	if res.CLI != "claude-tmux" || res.Tier != "balanced" {
		t.Errorf("terminal = cli %q tier %q, want claude-tmux@balanced", res.CLI, res.Tier)
	}
	wantAttempts := []string{"claude-tmux@deep", "codex-tmux@deep", "claude-tmux@balanced"}
	if fmt.Sprint(attempts) != fmt.Sprint(wantAttempts) {
		t.Errorf("attempt order = %v, want %v", attempts, wantAttempts)
	}
	if fmt.Sprint(steps) != fmt.Sprint([]string{"deep->balanced"}) {
		t.Errorf("surfaced step-downs = %v, want exactly [deep->balanced]", steps)
	}
}

func TestDispatchTiered_TerminalOnlyAfterLowestTierExhausted(t *testing.T) {
	var attempts []string
	res := DispatchTiered(tierTestPlan(), func(cli, tier string) (int, error) {
		attempts = append(attempts, cli+"@"+tier)
		return 85, errors.New("quota exhausted")
	}, nil)
	if res.Err == nil {
		t.Fatalf("expected terminal quota error after the lowest tier is exhausted")
	}
	want := []string{
		"claude-tmux@deep", "codex-tmux@deep",
		"claude-tmux@balanced", "codex-tmux@balanced",
	}
	if fmt.Sprint(attempts) != fmt.Sprint(want) {
		t.Errorf("attempt order = %v, want %v", attempts, want)
	}
}

func TestDispatchTiered_NonQuotaExitsNeverStepTier(t *testing.T) {
	// Chain exhausted on 81 (CLI-level trigger): terminal error, no tier
	// step-down. A mixed 81/85 chain likewise must not step down — the
	// step-down predicate is "EVERY attempt at the tier exited 85".
	for _, mixed := range []bool{false, true} {
		var attempts []string
		stepDowns := 0
		res := DispatchTiered(tierTestPlan(), func(cli, tier string) (int, error) {
			attempts = append(attempts, cli+"@"+tier)
			if mixed && cli == "codex-tmux" {
				return 85, errors.New("quota exhausted")
			}
			return 81, errors.New("artifact timeout")
		}, func(from, to string) { stepDowns++ })
		if res.Err == nil {
			t.Errorf("mixed=%v: expected terminal error", mixed)
		}
		if len(attempts) != 2 || stepDowns != 0 {
			t.Errorf("mixed=%v: attempts %v stepDowns %d, want 2 deep attempts and 0 step-downs", mixed, attempts, stepDowns)
		}
	}
}

func TestDispatchTiered_LegitimateFailStopsImmediately(t *testing.T) {
	var attempts []string
	res := DispatchTiered(tierTestPlan(), func(cli, tier string) (int, error) {
		attempts = append(attempts, cli+"@"+tier)
		return 1, errors.New("legitimate FAIL")
	}, nil)
	if res.Err == nil {
		t.Fatalf("expected the legitimate FAIL to surface")
	}
	if len(attempts) != 1 || attempts[0] != "claude-tmux@deep" {
		t.Errorf("attempts = %v, want exactly [claude-tmux@deep]", attempts)
	}
}

func TestDispatchTiered_EmptyTiersDegradesToSingleTierModel(t *testing.T) {
	plan := Plan{
		Candidates: []string{"claude-tmux"},
		Triggers:   []int{85},
		Model:      "deep",
	}
	var attempts []string
	res := DispatchTiered(plan, func(cli, tier string) (int, error) {
		attempts = append(attempts, cli+"@"+tier)
		return 85, errors.New("quota exhausted")
	}, nil) // nil onStepDown must not panic
	if res.Err == nil {
		t.Errorf("expected terminal error, got success")
	}
	if fmt.Sprint(attempts) != fmt.Sprint([]string{"claude-tmux@deep"}) {
		t.Errorf("attempts = %v, want [claude-tmux@deep]", attempts)
	}
}
