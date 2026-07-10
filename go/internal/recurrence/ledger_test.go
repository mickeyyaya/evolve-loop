package recurrence

import (
	"path/filepath"
	"testing"
)

// fakeEscalator records Bump calls and answers OpenItemForPattern from a map.
type fakeEscalator struct {
	open  map[string]InboxItem
	bumps []bumpCall
}

type bumpCall struct {
	id     string
	weight float64
}

func (f *fakeEscalator) OpenItemForPattern(pattern string) (InboxItem, bool) {
	it, ok := f.open[pattern]
	return it, ok
}

func (f *fakeEscalator) Bump(id string, w float64) error {
	f.bumps = append(f.bumps, bumpCall{id, w})
	return nil
}

// fakeAutofiler counts Autofile calls.
type fakeAutofiler struct {
	calls []string
}

func (f *fakeAutofiler) Autofile(pattern string, count int) error {
	f.calls = append(f.calls, pattern)
	return nil
}

// TestLedger_ThreeSamePatternCountsThreeAndBumpsOnce (AC1): three same-pattern
// retro closeouts yield ledger count=3 and exactly ONE deterministic weight
// bump on the linked open inbox item — idempotent across a re-run of the same
// cycle's closeout.
func TestLedger_ThreeSamePatternCountsThreeAndBumpsOnce(t *testing.T) {
	pat := "self-sha-tamper"
	// Seed prior history (cycles 100, 200) so this cycle's close reaches count 3.
	led := &Ledger{Entries: map[string]*Entry{
		pat: {Pattern: pat, Cycles: []int{100, 200}, Count: 2, LastSeen: 200},
	}}
	esc := &fakeEscalator{open: map[string]InboxItem{
		pat: {ID: "self-sha-fix", Weight: 0.90},
	}}
	pol := DefaultEscalationPolicy()

	// Cycle 300's closeout runs three times (idempotent re-invocation).
	for i := 0; i < 3; i++ {
		if err := led.RecordClosure(pat, 300, esc, nil, pol); err != nil {
			t.Fatalf("RecordClosure: %v", err)
		}
	}

	if got := led.Count(pat); got != 3 {
		t.Fatalf("count = %d, want 3 (cycles 100,200,300)", got)
	}
	if len(esc.bumps) != 1 {
		t.Fatalf("bumps = %d, want exactly 1 (idempotent per cycle)", len(esc.bumps))
	}
	// Target = min(0.99, 0.90 + 0.03*(3-1)) = 0.96.
	if got := esc.bumps[0]; got.id != "self-sha-fix" || got.weight < 0.9599 || got.weight > 0.9601 {
		t.Fatalf("bump = %+v, want {self-sha-fix 0.96}", got)
	}
}

// TestLedger_CountGE2NoOpenItemAutofilesOnce (AC2): a pattern that reaches
// count>=2 with NO open inbox item is handed to the autofile seam EXACTLY ONCE
// while open — never zero, never twice.
func TestLedger_CountGE2NoOpenItemAutofilesOnce(t *testing.T) {
	pat := "orphan-recurrence"
	led := NewLedger()
	esc := &fakeEscalator{open: map[string]InboxItem{}} // no open item for pat
	af := &fakeAutofiler{}
	pol := DefaultEscalationPolicy()

	// count=1: below threshold, no autofile.
	if err := led.RecordClosure(pat, 100, esc, af, pol); err != nil {
		t.Fatal(err)
	}
	if len(af.calls) != 0 {
		t.Fatalf("autofile at count=1: got %d, want 0", len(af.calls))
	}
	// count=2: threshold, autofile #1.
	if err := led.RecordClosure(pat, 200, esc, af, pol); err != nil {
		t.Fatal(err)
	}
	// count=3: still no open item, must NOT autofile again.
	if err := led.RecordClosure(pat, 300, esc, af, pol); err != nil {
		t.Fatal(err)
	}
	if len(af.calls) != 1 {
		t.Fatalf("autofile calls = %d, want exactly 1 (dedup guard)", len(af.calls))
	}
	if led.Count(pat) != 3 {
		t.Fatalf("count = %d, want 3", led.Count(pat))
	}
}

// TestEscalationPolicy_TargetCapsAtCap pins the pure formula, including the cap.
func TestEscalationPolicy_TargetCapsAtCap(t *testing.T) {
	pol := DefaultEscalationPolicy()
	if got := pol.Target(0.90, 3); got < 0.9599 || got > 0.9601 {
		t.Fatalf("Target(0.90,3) = %v, want 0.96", got)
	}
	if got := pol.Target(0.98, 10); got != 0.99 {
		t.Fatalf("Target(0.98,10) = %v, want cap 0.99", got)
	}
	if got := pol.Target(0.5, 0); got != 0.5 {
		t.Fatalf("Target(0.5,0) = %v, want 0.5 (count clamped to 1)", got)
	}
}

// TestLedger_LoadSaveRoundTrip exercises the persistence exports (Load/Save)
// and Patterns' descending-count ordering.
func TestLedger_LoadSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recurrence-ledger.json")

	// Load of a missing file is an empty ledger, not an error.
	empty, err := Load(path)
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if empty.Count("x") != 0 {
		t.Fatalf("missing-file ledger not empty")
	}

	led := NewLedger()
	pol := DefaultEscalationPolicy()
	_ = led.RecordClosure("low", 1, nil, nil, pol)
	_ = led.RecordClosure("high", 1, nil, nil, pol)
	_ = led.RecordClosure("high", 2, nil, nil, pol)
	if err := led.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	pats := got.Patterns()
	if len(pats) != 2 || pats[0].Pattern != "high" || pats[0].Count != 2 {
		t.Fatalf("Patterns not sorted by count desc: %+v", pats)
	}
}
