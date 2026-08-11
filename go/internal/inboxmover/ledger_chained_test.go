package inboxmover

// ledger_chained_test.go — the fleet-concurrency chain-break generator
// (console-plane forensics 2026-08-11, item ledger-fleet-concurrency-chain):
// writeLedger raw-O_APPEND'ed unchained NDJSON (no prev_hash, no flock, no
// tip update) into the hash-chained ledger.jsonl on essentially every cycle's
// ship.postship — each such line breaks the walk at that point, and it also
// defeated the Rebaseline seal (which must bind the physical predecessor).
// Every inbox-lifecycle record now goes through the chained append path: same
// flock, same prev_hash/entry_seq, same atomic tip replace.

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestLedgerWrites_AreChainValid is the keystone: after a lifecycle event on
// a ledger that already has chained history, the WHOLE chain still verifies —
// the inbox record is a chain participant, not a foreign interleaved line.
func TestLedgerWrites_AreChainValid(t *testing.T) {
	repo := makeRepo(t)
	evolveDir := repo + "/.evolve"
	l := ledger.New(evolveDir)
	ctx := context.Background()
	if err := l.Append(ctx, core.LedgerEntry{TS: "2026-08-11T00:00:00Z", Role: "orchestrator", Kind: "test"}); err != nil {
		t.Fatal(err)
	}

	dropInboxFile(t, repo, "task-1.json", "task-1")
	if _, err := Claim(Options{
		ProjectRoot: repo,
		Now:         func() time.Time { return time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC) },
	}, "task-1", "7"); err != nil {
		t.Fatalf("claim: %v", err)
	}

	if err := l.Verify(ctx); err != nil {
		t.Fatalf("chain broken by an inbox-lifecycle write (the fleet-concurrency break generator): %v", err)
	}

	// The record itself must still be findable by its lifecycle identity.
	it, err := l.Iter(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = it.Close() }()
	found := false
	for {
		e, ok, err := it.Next()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		if e.Kind == "inbox-lifecycle" && e.TaskID == "task-1" && e.Action == "claim" {
			found = true
			if e.PrevHash == "" {
				t.Error("inbox-lifecycle entry has no prev_hash — written outside the chained path")
			}
		}
	}
	if !found {
		t.Error("no chained inbox-lifecycle entry with task_id=task-1 action=claim found")
	}
}
