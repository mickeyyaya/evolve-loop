package recurrence

import "testing"

func TestExportedTypesNamed(t *testing.T) {
	var _ Entry
	var _ InboxItem = InboxItem{ID: "x", Weight: 0.5}
	var _ Ledger
	var _ = DigestInput{FailedApproaches: nil}

	var esc Escalator = &fakeEscalator{open: map[string]InboxItem{}}
	if _, ok := esc.OpenItemForPattern("nope"); ok {
		t.Fatal("empty escalator reported an open item")
	}
	var af Autofiler = &fakeAutofiler{}
	if err := af.Autofile("p", 2); err != nil {
		t.Fatalf("Autofiler literal: %v", err)
	}

	var pol EscalationPolicy = DefaultEscalationPolicy()
	if pol.Threshold != 2 {
		t.Fatalf("DefaultEscalationPolicy threshold = %d, want 2", pol.Threshold)
	}
}
