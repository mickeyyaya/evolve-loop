package inboxbatch

import "testing"

type pairRule struct{}

func (pairRule) Edges(items []Item) []Edge {
	if len(items) < 2 {
		return nil
	}
	return []Edge{{A: 0, B: 1, Reason: "pair rule"}}
}

func TestConfigRules_CustomRuleInjection(t *testing.T) {
	items := []Item{
		{ID: "first", Weight: 0.5},
		{ID: "second", Weight: 0.5},
	}
	if got := len(Classify(items, Config{})); got != 2 {
		t.Fatalf("default rules grouped unrelated items: %d batches, want 2", got)
	}
	batches := Classify(items, Config{Rules: []Rule{pairRule{}}})
	if len(batches) != 1 {
		t.Fatalf("custom rule ignored: %d batches, want 1", len(batches))
	}
	if len(batches[0].Reasons) != 1 || batches[0].Reasons[0] != "pair rule" {
		t.Errorf("custom rule's Edge reason must surface; got %v", batches[0].Reasons)
	}
}

func TestUnifiedCommitment_ValidatesAndSizes(t *testing.T) {
	items := []Item{
		{ID: "first", Files: []string{"go/internal/first/first.go"}},
		{ID: "second", Files: []string{"go/internal/second/second.go"}},
	}
	commitment := UnifiedCommitment{
		RootCauseHypothesis: "both callers duplicate retry policy",
		SharedSeam:          "go/internal/retry",
		DesignRequirements:  []string{"one policy"},
		Members: []UnifiedMember{
			{ID: "first", Evidence: "first duplicates retry policy"},
			{ID: "second", Evidence: "second duplicates retry policy"},
		},
	}
	if err := commitment.Validate(items); err != nil {
		t.Fatalf("UnifiedCommitment.Validate: %v", err)
	}
	if got := commitment.Size(); got != "small" {
		t.Fatalf("UnifiedCommitment.Size() = %q, want small", got)
	}
}

func TestUnifiedCommitment_RejectsMixedCampaignMembership(t *testing.T) {
	items := []Item{
		{ID: "unscoped"},
		{ID: "scoped", Campaign: "campaign-a"},
	}
	commitment := UnifiedCommitment{
		RootCauseHypothesis: "both items share a root cause",
		SharedSeam:          "go/internal/shared",
		DesignRequirements:  []string{"one shared fix"},
		Members: []UnifiedMember{
			{ID: "unscoped", Evidence: "unscoped evidence"},
			{ID: "scoped", Evidence: "scoped evidence"},
		},
	}
	if err := commitment.Validate(items); err == nil {
		t.Fatal("UnifiedCommitment.Validate accepted mixed unscoped and campaign-scoped members")
	}
}
