package ledger

// rebaseline_foreign_tail_test.go — the console-plane live failure 2026-08-11:
// Rebaseline appended its seal through the TIP-chained path, but the physical
// last line of the file was a FOREIGN record (inboxmover's raw O_APPEND write:
// no prev_hash, no tip update), so the seal's prev_hash pointed at the last
// CHAINED line, sealChainsFromPrev rejected it against the physical
// predecessor, the anchor never moved, and the command failed with "seal
// appended but the chain still does not verify forward" — on exactly the
// damage class (out-of-band interleaved appends) the command was built for.
// A seal must bind the file AS IT PHYSICALLY EXISTS, not as the tip remembers
// it.

import (
	"context"
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRebaseline_ForeignUnchainedTailLine_SealsAndVerifies(t *testing.T) {
	dir := t.TempDir()
	l := New(dir)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := l.Append(ctx, core.LedgerEntry{TS: "2026-08-11T00:00:00Z", Role: "orchestrator", Kind: "test"}); err != nil {
			t.Fatal(err)
		}
	}
	// The rogue writer: a raw unchained line appended straight to the file —
	// no prev_hash, no entry_seq, tip left behind (the inboxmover shape).
	f, err := os.OpenFile(l.ledgerPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"ts":"2026-08-11T00:00:01Z","class":"inbox-lifecycle","action":"promote","task_id":"x","cycle":null,"git_sha":null,"reason":"r"}` + "\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	// Sanity: the chain is broken exactly as the live plane's was.
	if err := l.VerifyDeep(ctx); err == nil {
		t.Fatal("fixture not broken — the foreign line should break the walk")
	}

	if err := l.Rebaseline(ctx, "test operator note: sealing a foreign-tail fixture"); err != nil {
		t.Fatalf("Rebaseline on a foreign-tail chain failed (the live console-plane shape): %v", err)
	}
	if err := l.VerifyDeep(ctx); err != nil {
		t.Fatalf("chain not green after rebaseline: %v", err)
	}
	// Appending after the seal must keep the chain green: the tip has to have
	// moved to the seal line, or the NEXT chained append re-breaks the file.
	if err := l.Append(ctx, core.LedgerEntry{TS: "2026-08-11T00:00:02Z", Role: "orchestrator", Kind: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := l.VerifyDeep(ctx); err != nil {
		t.Fatalf("chain broken by the first append AFTER the seal (tip not moved to the seal): %v", err)
	}
}
