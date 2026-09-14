package llmroute

import (
	"errors"
	"strings"
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

// TestApplyUniversalFallback_AConfiguredCLIAvailable_AppendsTheRestAsLastResort
// — operator policy (2026-09-14, wave 2): a phase must try EVERY available CLI
// before it gives up, because one CLI's quota wall must never fail a cycle at
// its last phase. The configured chain keeps precedence (it runs first, in
// order); the discovered CLIs the profile allows are appended after it even
// when a configured CLI is present — that tail is what the walk reaches when
// the configured chain is present but walled.
func TestApplyUniversalFallback_AConfiguredCLIAvailable_AppendsTheRestAsLastResort(t *testing.T) {
	p := Plan{Candidates: []string{"claude-tmux", "codex-tmux"}}
	got := ApplyUniversalFallback(p, []string{"agy-tmux"}, lookPathStub("codex")) // codex present
	want := []string{"claude-tmux", "codex-tmux", "agy-tmux"}
	if strings.Join(got.Candidates, " ") != strings.Join(want, " ") {
		t.Fatalf("a present configured CLI keeps precedence and the discovered rest is appended; got %v", got.Candidates)
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

// TestApplyUniversalFallback_UnknownCandidateName_ConfiguredStaysFirst — an
// unknown candidate name (not in cliBinaryFor) keeps its position (matches
// Probe's "unknown name keeps position"); discovery is appended after it.
func TestApplyUniversalFallback_UnknownCandidateName_ConfiguredStaysFirst(t *testing.T) {
	p := Plan{Candidates: []string{"some-future-cli"}}
	got := ApplyUniversalFallback(p, []string{"agy-tmux"}, lookPathStub())
	if strings.Join(got.Candidates, " ") != "some-future-cli agy-tmux" {
		t.Fatalf("configured first, discovered appended; got %v", got.Candidates)
	}
}
