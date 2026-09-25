package core

import (
	"os"
	"path/filepath"
	"testing"
)

// hasClaimableInboxWork decides whether an empty triage commitment is the
// planner's fault (claimable work existed) or the honest state of the queue.
// Research F25 (wave 6, cycle 1688): pipeline-* items are console-owned by
// the ADR-0074 classifier, so a queue holding only those is NOT claimable
// work — a lane cannot claim them, and blaming triage for leaving them would
// route a cycle to the claim-failed termination for work no lane may do.
func TestHasClaimableInboxWork_ConsoleOwnedKindsAreNotClaimable(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(inbox, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("2026-09-15T00-00-00Z-repair.json", `{"id":"repair","kind":"pipeline-repair","weight":0.9,"title":"console-owned"}`)
	if hasClaimableInboxWork(root, 0) {
		t.Fatal("a queue of pipeline-* items holds no lane-claimable work")
	}
	write("2026-09-15T00-00-01Z-bug.json", `{"id":"bug","kind":"bug","weight":0.5,"title":"lane work"}`)
	if !hasClaimableInboxWork(root, 0) {
		t.Fatal("a lane-dispatchable item is claimable work")
	}
}
