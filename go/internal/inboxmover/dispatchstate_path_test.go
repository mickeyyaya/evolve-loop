package inboxmover

// dispatchstate_path_test.go — a pending task's dispatch state must carry the
// LIVE record's path.
//
// cycle-1548 (soak-20260823a halt): the dispatched scope id resolved to 17
// on-disk records — one LIVE in inbox/, sixteen namesakes in consumed/ — and
// the resolver, which had the live path in hand (FindFileByTaskID), DISCARDED
// it. The prompt then carried a bare id, the agent name-searched the tree, and
// every phase report cited a consumed record from a halt CURED two weeks
// earlier. The auto-minted P0 ids are deliberately stable per category (the
// dedup identity), so namesakes accumulate forever: name-based resolution is
// structurally unsafe here, and the resolved PATH is the only safe handle.

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

// THE headline, in the live shape: one pending record plus consumed namesakes.
// The state must be Pending AND carry the LIVE path — never a namesake's.
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

// Non-pending states carry no path: their records are lifecycle history, and
// handing an agent a processed/consumed path is exactly the defect.
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
