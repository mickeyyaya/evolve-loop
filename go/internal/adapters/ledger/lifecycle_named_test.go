package ledger

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestFileLedger_AppendLifecycle_ChainsAndMapsRecord names
// ledger.AppendLifecycle and ledger.LifecycleRecord (apicover) and pins the
// full mapping contract: the record lands as a CHAINED entry (prev_hash from
// the predecessor, tip moved) with Kind "inbox-lifecycle", operator-side Role
// "orchestrator", and every LifecycleRecord field routed to its LedgerEntry
// home — the whole point of the seam is that inboxmover's lifecycle telemetry
// became a chain participant (item ledger-fleet-concurrency-chain).
func TestFileLedger_AppendLifecycle_ChainsAndMapsRecord(t *testing.T) {
	dir := t.TempDir()
	l := New(dir)
	ctx := context.Background()
	if err := l.Append(ctx, core.LedgerEntry{TS: "2026-08-11T00:00:00Z", Role: "orchestrator", Kind: "test"}); err != nil {
		t.Fatal(err)
	}
	rec := LifecycleRecord{
		TS:      "2026-08-11T00:00:01Z",
		Action:  "claim",
		TaskID:  "task-9",
		Message: "inbox → processing: triage-claim",
		GitHead: "abc123",
		Cycle:   9,
	}
	if err := l.AppendLifecycle(ctx, rec); err != nil {
		t.Fatalf("AppendLifecycle: %v", err)
	}
	if err := l.Verify(ctx); err != nil {
		t.Fatalf("lifecycle entry broke the chain: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := splitLines(raw)
	var e core.LedgerEntry
	if err := json.Unmarshal(lines[len(lines)-1], &e); err != nil {
		t.Fatal(err)
	}
	if e.Kind != "inbox-lifecycle" || e.Role != "orchestrator" {
		t.Errorf("kind/role = %q/%q, want inbox-lifecycle/orchestrator", e.Kind, e.Role)
	}
	if e.TS != rec.TS || e.Action != rec.Action || e.TaskID != rec.TaskID ||
		e.Message != rec.Message || e.GitHEAD != rec.GitHead || e.Cycle != rec.Cycle {
		t.Errorf("record fields not mapped: %+v vs %+v", e, rec)
	}
	if e.PrevHash == "" || e.PrevHash == ZeroSeed {
		t.Errorf("lifecycle entry must chain from its predecessor, prev_hash=%q", e.PrevHash)
	}
}
