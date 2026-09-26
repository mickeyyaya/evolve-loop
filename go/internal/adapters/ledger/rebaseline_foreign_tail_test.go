package ledger

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
	// A foreign raw line past the tip: no prev_hash, no entry_seq, tip not moved.
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
	if err := l.VerifyDeep(ctx); err == nil {
		t.Fatal("fixture not broken — the foreign line should break the walk")
	}

	if err := l.Rebaseline(ctx, "test operator note: sealing a foreign-tail fixture"); err != nil {
		t.Fatalf("Rebaseline on a foreign-tail chain failed (the live console-plane shape): %v", err)
	}
	if err := l.VerifyDeep(ctx); err != nil {
		t.Fatalf("chain not green after rebaseline: %v", err)
	}
	if err := l.Append(ctx, core.LedgerEntry{TS: "2026-08-11T00:00:02Z", Role: "orchestrator", Kind: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := l.VerifyDeep(ctx); err != nil {
		t.Fatalf("chain broken by the first append AFTER the seal (tip not moved to the seal): %v", err)
	}
}
