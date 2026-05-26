package ledger

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestIter_RealLedger_NoStringCycleError confirms that the live reader
// can iterate the project's real .evolve/ledger.jsonl end-to-end
// without the "json: cannot unmarshal string into Go struct field
// LedgerEntry.cycle of type int" failure that the dispatcher logged at
// line 1740 during cycle-107 (2026-05-26).
//
// The test is skipped when the file isn't reachable (CI sandboxes,
// fresh checkouts), so this acts as a project-local smoke without
// constraining external test environments.
func TestIter_RealLedger_NoStringCycleError(t *testing.T) {
	candidates := []string{
		filepath.Join("..", "..", "..", "..", ".evolve"),
		filepath.Join("..", "..", "..", "..", "..", ".evolve"),
	}
	var evolveDir string
	for _, c := range candidates {
		abs, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(abs, "ledger.jsonl")); err == nil {
			evolveDir = abs
			break
		}
	}
	if evolveDir == "" {
		t.Skip("real .evolve/ledger.jsonl not reachable from this test cwd")
	}
	l := New(evolveDir)
	it, err := l.Iter(context.Background())
	if err != nil {
		t.Fatalf("Iter open: %v", err)
	}
	defer func() { _ = it.Close() }()
	var read, withLabel int
	for {
		e, ok, err := it.Next()
		if err != nil {
			t.Fatalf("Iter.Next at entry #%d: %v (this is the cycle-107 bug — defensive unmarshal must absorb string cycles)", read, err)
		}
		if !ok {
			break
		}
		if e.CycleLabel != "" {
			withLabel++
			t.Logf("found labeled entry seq=%d cycle=%d label=%q role=%s", e.EntrySeq, e.Cycle, e.CycleLabel, e.Role)
		}
		read++
	}
	t.Logf("read %d entries; %d carried CycleLabel (legacy string-cycle absorbed)", read, withLabel)
	if read == 0 {
		t.Fatal("expected at least one entry from real ledger")
	}
	// Sanity: we know the v10.16.0 manual entry exists in this project's
	// ledger. If the project ledger is loaded, we expect at least one
	// CycleLabel-carrying entry (the legacy bad line at seq=1740).
	if withLabel == 0 {
		t.Logf("note: zero labeled entries — either the project ledger predates the v10.16.0 manual entry, or it was rewritten")
	}
}
