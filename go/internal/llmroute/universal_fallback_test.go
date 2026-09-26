package llmroute

import (
	"errors"
	"strings"
	"testing"
)

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

func TestApplyUniversalFallback_AConfiguredCLIAvailable_AppendsTheRestAsLastResort(t *testing.T) {
	p := Plan{Candidates: []string{"claude-tmux", "codex-tmux"}}
	got := ApplyUniversalFallback(p, []string{"agy-tmux"}, lookPathStub("codex")) // codex present
	want := []string{"claude-tmux", "codex-tmux", "agy-tmux"}
	if strings.Join(got.Candidates, " ") != strings.Join(want, " ") {
		t.Fatalf("a present configured CLI keeps precedence and the discovered rest is appended; got %v", got.Candidates)
	}
}

func TestApplyUniversalFallback_NoDiscovered_FailLoudPreserved(t *testing.T) {
	p := Plan{Candidates: []string{"claude-tmux"}}
	got := ApplyUniversalFallback(p, nil, lookPathStub())
	if len(got.Candidates) != 1 || got.Candidates[0] != "claude-tmux" {
		t.Fatalf("no discovered CLIs must leave the plan untouched (fail-loud); got %v", got.Candidates)
	}
}

func TestApplyUniversalFallback_DedupesAndPreservesOtherFields(t *testing.T) {
	p := Plan{
		Candidates: []string{"claude-tmux", "agy-tmux"}, // agy already present in chain
		Triggers:   []int{80, 85},
		Model:      "auto",
		Tiers:      []string{"deep"},
	}
	got := ApplyUniversalFallback(p, []string{"agy-tmux", "codex-tmux"}, lookPathStub()) // all static missing
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

func TestApplyUniversalFallback_UnknownCandidateName_ConfiguredStaysFirst(t *testing.T) {
	p := Plan{Candidates: []string{"some-future-cli"}}
	got := ApplyUniversalFallback(p, []string{"agy-tmux"}, lookPathStub())
	if strings.Join(got.Candidates, " ") != "some-future-cli agy-tmux" {
		t.Fatalf("configured first, discovered appended; got %v", got.Candidates)
	}
}
