package inboxmover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeInboxRecord(t *testing.T, dir, id string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	b, _ := json.Marshal(map[string]any{"id": id, "title": id})
	p := filepath.Join(dir, "2026-08-22T15-02-52Z-"+id+".json")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func TestResolveDispatchState_PendingCarriesTheLivePath(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	live := writeInboxRecord(t, inbox, "pipeline-defect-pipeline-blocker")
	for _, sub := range []string{"processed", "processed/cycle-0"} {
		writeInboxRecord(t, filepath.Join(inbox, sub), "pipeline-defect-pipeline-blocker")
	}

	st := ResolveDispatchState(Options{ProjectRoot: root}, "pipeline-defect-pipeline-blocker")
	if st.State != StatePending {
		t.Fatalf("state = %q, want pending", st.State)
	}
	if st.Path != live {
		t.Fatalf("Path must be the LIVE record, not a consumed namesake: got %q want %q", st.Path, live)
	}
}

func TestResolveDispatchState_NonPendingCarriesNoPath(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeInboxRecord(t, filepath.Join(inbox, "processed"), "done-task")

	st := ResolveDispatchState(Options{ProjectRoot: root}, "done-task")
	if st.State != StateProcessed {
		t.Fatalf("state = %q, want processed", st.State)
	}
	if st.Path != "" {
		t.Fatalf("a non-pending state must not hand out a path; got %q", st.Path)
	}
}
