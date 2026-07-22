package inboxmover

// failurecount_read_test.go — ADR-0076 slice D: the exported read path from
// item id → durable failure_count (written by bumpFailureCount on FAIL
// release). Searches the inbox root AND processing/cycle-*/ (a claimed item
// mid-cycle still resolves). Absent item / absent field → (0, false/true)
// per contract below.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadFailureCount_InboxRootAndProcessing(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	proc := filepath.Join(inbox, "processing", "cycle-9")
	for _, d := range []string{inbox, proc} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(inbox, "a.json"), []byte(`{"id":"item-a","failure_count":2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proc, "b.json"), []byte(`{"id":"item-b","failure_count":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := Options{ProjectRoot: root}
	if n, ok := ReadFailureCount(opts, "item-a"); !ok || n != 2 {
		t.Fatalf("inbox-root item: want (2,true), got (%d,%v)", n, ok)
	}
	if n, ok := ReadFailureCount(opts, "item-b"); !ok || n != 1 {
		t.Fatalf("claimed item in processing/: want (1,true), got (%d,%v)", n, ok)
	}
	if n, ok := ReadFailureCount(opts, "item-missing"); ok || n != 0 {
		t.Fatalf("absent item: want (0,false), got (%d,%v)", n, ok)
	}
}

func TestReadFailureCount_FieldAbsentIsZero(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "c.json"), []byte(`{"id":"never-failed"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if n, ok := ReadFailureCount(Options{ProjectRoot: root}, "never-failed"); !ok || n != 0 {
		t.Fatalf("present item without the field: want (0,true), got (%d,%v)", n, ok)
	}
}
