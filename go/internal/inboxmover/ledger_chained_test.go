package inboxmover

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestLedgerAppender_SatisfiedByFileLedger(t *testing.T) {
	var _ LedgerAppender = ledger.New(t.TempDir())
}

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
