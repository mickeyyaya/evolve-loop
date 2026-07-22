package inboxbatch

import (
	"os"
	"path/filepath"
	"testing"
)

// continuation_test.go — ADR-0076 slice C schema: a FAILed cycle with
// salvageable preserved work stamps its item with a continuation binding; the
// next claim adopts it instead of restarting cold. The field must round-trip
// tolerantly through LoadDir (absent = nil, never an error).

func TestItem_ContinuationRoundTrip(t *testing.T) {
	dir := t.TempDir()
	body := `{
	  "id": "role-lessons-hardening",
	  "title": "x",
	  "continuation": {
	    "worktree": "/repo/.evolve/worktrees/cycle-1071",
	    "branch": "evolve/cycle-1071",
	    "snapshot_sha": "abc123def456",
	    "base_sha": "789fed",
	    "findings_path": ".evolve/runs/cycle-1071/failure-digest.json",
	    "cycle": 1071
	  }
	}`
	if err := os.WriteFile(filepath.Join(dir, "a.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	items, warns, err := LoadDir(dir)
	if err != nil || len(warns) != 0 || len(items) != 1 {
		t.Fatalf("LoadDir: items=%d warns=%v err=%v", len(items), warns, err)
	}
	c := items[0].Continuation
	if c == nil {
		t.Fatal("continuation must parse")
	}
	if c.SnapshotSHA != "abc123def456" || c.Branch != "evolve/cycle-1071" || c.Cycle != 1071 {
		t.Errorf("continuation fields: %+v", c)
	}
	if c.Worktree != "/repo/.evolve/worktrees/cycle-1071" || c.FindingsPath != ".evolve/runs/cycle-1071/failure-digest.json" {
		t.Errorf("continuation paths: %+v", c)
	}
}

func TestItem_ContinuationAbsentIsNil(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "b.json"), []byte(`{"id":"plain"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	items, _, err := LoadDir(dir)
	if err != nil || len(items) != 1 {
		t.Fatalf("LoadDir: %v", err)
	}
	if items[0].Continuation != nil {
		t.Errorf("absent continuation must be nil, got %+v", items[0].Continuation)
	}
}
