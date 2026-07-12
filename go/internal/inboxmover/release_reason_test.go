// release_reason_test.go — pins ReleaseCycleProcessingWithReason (cycle-752,
// inbox-promotion-requires-landed-ship): an explicit reason lands verbatim in
// the ledger entry for each released item; an empty reason keeps the generic
// "cycle-release" default (byte-compatible with the pre-existing wrapper).
package inboxmover

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// writeReasonFixture lays down processing/cycle-<cid>/<id>.json and returns
// the repo root.
func writeReasonFixture(t *testing.T, cid int, id string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "inbox", "processing", "cycle-"+strconv.Itoa(cid))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}
	body := `{"id":"` + id + `","title":"fixture"}`
	if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return root
}

func readLedger(t *testing.T, root string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, ".evolve", "ledger.jsonl"))
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	return string(body)
}

// TestReleaseCycleProcessingWithReason_ExplicitReason pins that a caller-
// supplied reason is written verbatim to the released item's ledger entry.
func TestReleaseCycleProcessingWithReason_ExplicitReason(t *testing.T) {
	root := writeReasonFixture(t, 91, "task-a")
	res, err := ReleaseCycleProcessingWithReason(Options{ProjectRoot: root}, 91, "cycle-release-unlanded-ship-retry")
	if err != nil {
		t.Fatalf("ReleaseCycleProcessingWithReason: %v", err)
	}
	if res.Recovered != 1 {
		t.Fatalf("Recovered = %d, want 1", res.Recovered)
	}
	ledger := readLedger(t, root)
	if !strings.Contains(ledger, `"reason":"cycle-release-unlanded-ship-retry"`) {
		t.Errorf("ledger missing explicit reason; got: %s", ledger)
	}
}

// TestReleaseCycleProcessingWithReason_EmptyReasonKeepsGenericDefault pins
// backward compatibility: empty reason == the wrapper's "cycle-release".
func TestReleaseCycleProcessingWithReason_EmptyReasonKeepsGenericDefault(t *testing.T) {
	root := writeReasonFixture(t, 92, "task-b")
	if _, err := ReleaseCycleProcessingWithReason(Options{ProjectRoot: root}, 92, ""); err != nil {
		t.Fatalf("ReleaseCycleProcessingWithReason: %v", err)
	}
	ledger := readLedger(t, root)
	if !strings.Contains(ledger, `"reason":"cycle-release"`) {
		t.Errorf("empty reason must default to generic cycle-release; got: %s", ledger)
	}
	if strings.Contains(ledger, "unlanded") {
		t.Errorf("empty reason must not carry an unlanded note; got: %s", ledger)
	}
}
