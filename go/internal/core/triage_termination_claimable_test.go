package core

import (
	"os"
	"path/filepath"
	"testing"
)

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
	if hasClaimableInboxWork(root, 0, routingOnlyLaneMenu) {
		t.Fatal("a queue of pipeline-* items holds no lane-claimable work")
	}
	write("2026-09-15T00-00-01Z-bug.json", `{"id":"bug","kind":"bug","weight":0.5,"title":"lane work"}`)
	if !hasClaimableInboxWork(root, 0, routingOnlyLaneMenu) {
		t.Fatal("a lane-dispatchable item is claimable work")
	}
}
