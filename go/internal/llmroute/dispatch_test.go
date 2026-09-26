package llmroute

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// Names DispatchResult for apicover, whose naming check scans identifiers and misses inferred types.
var _ DispatchResult

// scriptedLaunch replays seq by call order, repeating the last entry, and records each cli launched.
type scriptedLaunch struct {
	seq   []scriptedAttempt
	calls []string
}

type scriptedAttempt struct {
	exitCode int
	err      error
}

func (s *scriptedLaunch) launch(cli string) (int, error) {
	s.calls = append(s.calls, cli)
	i := len(s.calls) - 1
	if i >= len(s.seq) {
		i = len(s.seq) - 1
	}
	a := s.seq[i]
	return a.exitCode, a.err
}

func TestDispatch_SuccessOnFirstCandidate(t *testing.T) {
	plan := Plan{Candidates: []string{"claude-tmux"}, Triggers: defaultFallbackOnExit}
	sl := &scriptedLaunch{seq: []scriptedAttempt{{exitCode: 0, err: nil}}}

	got := Dispatch(plan, sl.launch)

	if got.Err != nil {
		t.Fatalf("Dispatch: unexpected error %v", got.Err)
	}
	if got.CLI != "claude-tmux" {
		t.Errorf("Dispatch: CLI=%q, want claude-tmux", got.CLI)
	}
	if !reflect.DeepEqual(sl.calls, []string{"claude-tmux"}) {
		t.Errorf("Dispatch: launched %v, want exactly one call to claude-tmux (byte-identical single-CLI chain)", sl.calls)
	}
	if !reflect.DeepEqual(got.Attempts, []string{"claude-tmux"}) {
		t.Errorf("Dispatch: Attempts=%v, want [claude-tmux]", got.Attempts)
	}
}

// Per-code trigger coverage lives here: the end-to-end TestE2ECLIFallbackChain costs about ten minutes per code.
func TestDispatch_FallsBackOnTriggerExit(t *testing.T) {
	if len(defaultFallbackOnExit) == 0 {
		t.Fatal("defaultFallbackOnExit is empty — the trigger contract would be vacuous")
	}
	for _, code := range defaultFallbackOnExit {
		t.Run(fmt.Sprintf("exit_%d", code), func(t *testing.T) {
			plan := Plan{Candidates: []string{"agy-tmux", "claude-tmux"}, Triggers: defaultFallbackOnExit}
			sl := &scriptedLaunch{seq: []scriptedAttempt{
				{exitCode: code, err: fmt.Errorf("bridge: launch exit=%d", code)},
				{exitCode: 0, err: nil},
			}}

			got := Dispatch(plan, sl.launch)

			if got.Err != nil {
				t.Fatalf("Dispatch: expected fallback success, got err=%v", got.Err)
			}
			if got.CLI != "claude-tmux" {
				t.Errorf("Dispatch: CLI=%q, want claude-tmux (the fallback that succeeded)", got.CLI)
			}
			want := []string{"agy-tmux", "claude-tmux"}
			if !reflect.DeepEqual(sl.calls, want) {
				t.Errorf("Dispatch: launched %v, want %v (both candidates tried in order)", sl.calls, want)
			}
			if !reflect.DeepEqual(got.Attempts, want) {
				t.Errorf("Dispatch: Attempts=%v, want %v", got.Attempts, want)
			}
		})
	}
}

func TestDispatch_StopsOnNonTriggerExit(t *testing.T) {
	plan := Plan{Candidates: []string{"agy-tmux", "claude-tmux"}, Triggers: []int{81}}
	sl := &scriptedLaunch{seq: []scriptedAttempt{
		{exitCode: 1, err: errors.New("real failure, not a trigger")},
	}}

	got := Dispatch(plan, sl.launch)

	if got.Err == nil {
		t.Fatalf("Dispatch: expected the non-trigger error to surface, got nil")
	}
	if len(sl.calls) != 1 {
		t.Fatalf("Dispatch: launched %v, want exactly ONE call — a non-trigger exit must never reroute to the fallback", sl.calls)
	}
	if got.CLI != "agy-tmux" {
		t.Errorf("Dispatch: CLI=%q, want agy-tmux (the sole attempt)", got.CLI)
	}
}

func TestDispatch_AllCandidatesFailReturnsLastError(t *testing.T) {
	plan := Plan{Candidates: []string{"agy-tmux", "claude-tmux"}, Triggers: []int{81}}
	lastErr := errors.New("claude-tmux: launch exit=81 too")
	sl := &scriptedLaunch{seq: []scriptedAttempt{
		{exitCode: 81, err: errors.New("agy-tmux: launch exit=81")},
		{exitCode: 81, err: lastErr},
	}}

	got := Dispatch(plan, sl.launch)

	if got.Err == nil {
		t.Fatalf("Dispatch: expected an error when every candidate is exhausted, got nil")
	}
	if !errors.Is(got.Err, lastErr) && got.Err.Error() != lastErr.Error() {
		t.Errorf("Dispatch: Err=%v, want the LAST candidate's error (%v)", got.Err, lastErr)
	}
	want := []string{"agy-tmux", "claude-tmux"}
	if !reflect.DeepEqual(sl.calls, want) {
		t.Errorf("Dispatch: launched %v, want %v (both candidates exhausted)", sl.calls, want)
	}
}

func TestChainFor_PrimaryPlusDedupedFallback(t *testing.T) {
	prof := &profiles.Profile{
		CLI:               "agy-tmux", // the swapped-away primary, excluded from the chain
		CLIFallback:       []string{"claude-tmux", "agy-tmux"},
		CLIFallbackOnExit: []int{81},
	}
	got := ChainFor("codex-tmux", prof)

	want := []string{"codex-tmux", "claude-tmux"}
	if !reflect.DeepEqual(got.Candidates, want) {
		t.Errorf("ChainFor: Candidates=%v, want %v (explicit primary honored over profile.cli, dedup preserved)", got.Candidates, want)
	}
	if !reflect.DeepEqual(got.Triggers, []int{81}) {
		t.Errorf("ChainFor: Triggers=%v, want [81] (profile.cli_fallback_on_exit)", got.Triggers)
	}
}

func TestChainFor_NilProfileSingleCandidateDefaultTriggers(t *testing.T) {
	got := ChainFor("claude-tmux", nil)

	if !reflect.DeepEqual(got.Candidates, []string{"claude-tmux"}) {
		t.Errorf("ChainFor: Candidates=%v, want [claude-tmux]", got.Candidates)
	}
	if !reflect.DeepEqual(got.Triggers, defaultFallbackOnExit) {
		t.Errorf("ChainFor: Triggers=%v, want the package default %v", got.Triggers, defaultFallbackOnExit)
	}
}
