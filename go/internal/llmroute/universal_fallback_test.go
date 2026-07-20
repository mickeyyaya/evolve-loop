package llmroute

import (
	"errors"
	"testing"
)

// lookPathStub reports the given set as present-on-PATH, everything else missing.
func lookPathStub(present ...string) func(string) (string, error) {
	set := map[string]struct{}{}
	for _, p := range present {
		set[p] = struct{}{}
	}
	return func(bin string) (string, error) {
		if _, ok := set[bin]; ok {
			return "/usr/local/bin/" + bin, nil
		}
		return "", errors.New("not found")
	}
}

// TestApplyUniversalFallback_AllStaticMissing_AppendsDiscovered — the headline
// case: the whole configured chain's binaries are absent (e.g. an agy-only host
// where the profile still names claude/codex), so the discovered installed CLIs
// are appended to the tail and the loop can dispatch instead of halting.
func TestApplyUniversalFallback_AllStaticMissing_AppendsDiscovered(t *testing.T) {
	p := Plan{Candidates: []string{"claude-tmux", "codex-tmux"}} // both binaries absent
	got := ApplyUniversalFallback(p, []string{"agy-tmux"}, lookPathStub("agy"))
	want := []string{"claude-tmux", "codex-tmux", "agy-tmux"}
	if len(got.Candidates) != len(want) {
		t.Fatalf("candidates = %v, want %v", got.Candidates, want)
	}
	for i := range want {
		if got.Candidates[i] != want[i] {
			t.Fatalf("candidates = %v, want %v (discovered appended AFTER the configured chain)", got.Candidates, want)
		}
	}
}

// TestApplyUniversalFallback_AConfiguredCLIAvailable_NoOp — the configured chain
// wins: if ANY static candidate's binary is present, discovery must not touch
// the plan (operator config is authoritative; universal fallback is last-resort).
func TestApplyUniversalFallback_AConfiguredCLIAvailable_NoOp(t *testing.T) {
	p := Plan{Candidates: []string{"claude-tmux", "codex-tmux"}}
	got := ApplyUniversalFallback(p, []string{"agy-tmux"}, lookPathStub("codex")) // codex present
	if len(got.Candidates) != 2 {
		t.Fatalf("a present configured CLI must suppress universal fallback; got %v", got.Candidates)
	}
}

// TestApplyUniversalFallback_NoDiscovered_FailLoudPreserved — nothing discovered
// (or discovery disabled) → the plan is untouched, so the classifier still sees
// a real ExitMissingBinary on the absent configured chain (never silently green).
func TestApplyUniversalFallback_NoDiscovered_FailLoudPreserved(t *testing.T) {
	p := Plan{Candidates: []string{"claude-tmux"}}
	got := ApplyUniversalFallback(p, nil, lookPathStub())
	if len(got.Candidates) != 1 || got.Candidates[0] != "claude-tmux" {
		t.Fatalf("no discovered CLIs must leave the plan untouched (fail-loud); got %v", got.Candidates)
	}
}

// TestApplyUniversalFallback_DedupesAndPreservesOtherFields — a discovered CLI
// already in the chain is not duplicated, and non-Candidates Plan fields survive.
func TestApplyUniversalFallback_DedupesAndPreservesOtherFields(t *testing.T) {
	p := Plan{
		Candidates: []string{"claude-tmux", "agy-tmux"}, // agy already present in chain
		Triggers:   []int{80, 85},
		Model:      "auto",
		Tiers:      []string{"deep"},
	}
	got := ApplyUniversalFallback(p, []string{"agy-tmux", "codex-tmux"}, lookPathStub()) // all static missing
	// agy-tmux already in chain (not re-appended); codex-tmux discovered+new → appended.
	want := []string{"claude-tmux", "agy-tmux", "codex-tmux"}
	if len(got.Candidates) != len(want) {
		t.Fatalf("candidates = %v, want %v (dedup agy)", got.Candidates, want)
	}
	for i := range want {
		if got.Candidates[i] != want[i] {
			t.Fatalf("candidates = %v, want %v", got.Candidates, want)
		}
	}
	if len(got.Triggers) != 2 || got.Model != "auto" || len(got.Tiers) != 1 {
		t.Errorf("non-Candidates fields were dropped: %+v", got)
	}
}

// TestApplyUniversalFallback_UnknownCandidateName_ConfiguredWins — an unknown
// candidate name (not in cliBinaryFor) is treated as available (matches Probe's
// "unknown name keeps position"), so discovery does not override it.
func TestApplyUniversalFallback_UnknownCandidateName_ConfiguredWins(t *testing.T) {
	p := Plan{Candidates: []string{"some-future-cli"}}
	got := ApplyUniversalFallback(p, []string{"agy-tmux"}, lookPathStub())
	if len(got.Candidates) != 1 {
		t.Fatalf("an unknown (assumed-available) candidate must suppress fallback; got %v", got.Candidates)
	}
}
