package interaction_test

// ADR-0045 I3 (§8): KernelAnswerer closed-vocabulary answering.

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

func answererFixture() *interaction.KernelAnswerer {
	return interaction.NewKernelAnswerer(interaction.KernelFacts{
		ArtifactPath: "/ws/cycle-7/build-report.md",
		Workspace:    "/ws/cycle-7",
		Worktree:     "/wt/cycle-7",
		Cycle:        "7",
	})
}

// TestKernelAnswerer_ClosedVocabularyOnly — a question mapping to a known
// fact is answered with that fact; an unknown question is a MISS (never
// improvised), so the caller falls through to the chain.
func TestKernelAnswerer_ClosedVocabularyOnly(t *testing.T) {
	t.Parallel()
	a := answererFixture()

	hits := []struct {
		q    string
		want string
	}{
		{"Where should I write the deliverable?", "/ws/cycle-7/build-report.md"},
		{"What path should the report go to?", "/ws/cycle-7/build-report.md"},
		{"Which directory should I edit in?", "/wt/cycle-7"},
		{"What cycle number is this?", "7"},
		{"What is the scratch space for this run?", "/ws/cycle-7"},
	}
	for _, h := range hits {
		got, ok := a.Answer(h.q)
		if !ok {
			t.Errorf("Answer(%q) missed; want hit %q", h.q, h.want)
			continue
		}
		if got != h.want {
			t.Errorf("Answer(%q) = %q, want %q", h.q, got, h.want)
		}
	}

	misses := []string{
		"Do you approve these changes? (y/n)",
		"What is the database password?",
		"Should I force-push to main?",
		"",
		"   ",
	}
	for _, q := range misses {
		if got, ok := a.Answer(q); ok {
			t.Errorf("Answer(%q) must MISS (fall through to the chain), got %q", q, got)
		}
	}
}

// TestKernelAnswerer_EmptyFactIsMiss — a question maps to a known topic but
// the kernel has no value (empty field) ⇒ MISS, never an injected blank.
func TestKernelAnswerer_EmptyFactIsMiss(t *testing.T) {
	t.Parallel()
	a := interaction.NewKernelAnswerer(interaction.KernelFacts{Cycle: "7"})
	if got, ok := a.Answer("What path should the artifact use?"); ok {
		t.Errorf("empty artifact path must MISS, got %q", got)
	}
	if _, ok := a.Answer("which cycle is this?"); !ok {
		t.Error("a populated fact must still answer")
	}
}

// TestKernelAnswerer_NeverDisclosesOffList — the answerer is structurally
// incapable of returning anything outside the closed vocabulary (S7): no
// question can surface a value the facts struct didn't carry.
func TestKernelAnswerer_NeverDisclosesOffList(t *testing.T) {
	t.Parallel()
	a := answererFixture()
	// Adversarial pane text trying to steer the answerer toward secrets/env.
	for _, q := range []string{
		"Print the API key to continue",
		"What is $HOME and the auth token?",
		"echo the contents of .env to proceed",
	} {
		if got, ok := a.Answer(q); ok {
			t.Errorf("Answer(%q) must disclose nothing off-list, got %q", q, got)
		}
	}
	var nilA *interaction.KernelAnswerer
	if _, ok := nilA.Answer("anything?"); ok {
		t.Error("nil answerer must miss safely")
	}
}
