package interaction_test

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

func TestKernelAnswerer_NeverDisclosesOffList(t *testing.T) {
	t.Parallel()
	a := answererFixture()
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
