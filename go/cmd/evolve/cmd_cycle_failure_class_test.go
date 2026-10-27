package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFailureClassReader_ReadsTheLastFailureClassOfTheProjectInbox(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "2026-10-27T00-00-00Z-classed.json"), []byte(`{"id":"classed","failure_count":1,"last_failure_class":"capability"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	read := failureClassReader(root)
	if got := read("classed"); got != "capability" {
		t.Errorf("failure class = %q, want the item's capability", got)
	}
	if got := read("absent-task"); got != "" {
		t.Errorf("failure class of an absent task = %q, want empty", got)
	}
}
