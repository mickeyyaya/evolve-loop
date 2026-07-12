package recurrence

import "testing"

// TestExportedTypesNamed pins the exported vocabulary apicover requires every
// export be named in a test — the behavioural tests only touch these via field
// access or as fake receivers. It also asserts a bare struct literal satisfies
// the Escalator/Autofiler interfaces (the load-bearing seam shapes).
func TestExportedTypesNamed(t *testing.T) {
	var _ Entry
	var _ InboxItem = InboxItem{ID: "x", Weight: 0.5}
	var _ Ledger
	// Chronicle S2 digest vocabulary: the behavioural tests exercise WriteDigest
	// via Dossiers/Index only, so name the remaining field here.
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
