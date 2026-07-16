package inboxbatch

// apicover_named_test.go — per-symbol coverage naming (the apicover two-signal
// convention: every exported symbol is named by a test with a REAL assertion).
// Edge and Rule are the Strategy seam: this test proves Config.Rules is
// honored by injecting a custom rule that binds two otherwise-unrelated items
// — the extension point future signals (title similarity, size buckets) plug
// into without touching the classifier.

import "testing"

// pairRule is a minimal custom Rule: it always edges item 0 to item 1.
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
	// Default rules: no shared signal → two singletons.
	if got := len(Classify(items, Config{})); got != 2 {
		t.Fatalf("default rules grouped unrelated items: %d batches, want 2", got)
	}
	// Injected custom Rule (the Strategy seam): one batch, its Edge reason
	// surfaced.
	batches := Classify(items, Config{Rules: []Rule{pairRule{}}})
	if len(batches) != 1 {
		t.Fatalf("custom rule ignored: %d batches, want 1", len(batches))
	}
	if len(batches[0].Reasons) != 1 || batches[0].Reasons[0] != "pair rule" {
		t.Errorf("custom rule's Edge reason must surface; got %v", batches[0].Reasons)
	}
}
