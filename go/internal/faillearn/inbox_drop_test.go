package faillearn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func inboxFixture(t *testing.T) (inboxDir string, item InboxItem) {
	t.Helper()
	return t.TempDir(), InboxItem{
		ID: "retro-1255-stale-worktree", Title: "stale cs.ActiveWorktree survives fleet teardown",
		Weight: 0.9, Kind: "bug", Priority: "H", PriorityClass: "correctness", InjectedBy: "faillearn-failure-floor",
	}
}

func TestWriteInboxItems_CollidingFilenameIsNotSilentlyDropped(t *testing.T) {
	dir, item := inboxFixture(t)
	squatter := filepath.Join(dir, item.ID+".json")
	if err := os.WriteFile(squatter, []byte(`{"id":"retro-1255-stale-worktree","title":"someone else's item"}`), 0o644); err != nil {
		t.Fatalf("seed squatter: %v", err)
	}

	err := writeConfig{inboxDir: dir, inboxItems: []InboxItem{item}}.writeInboxItems()
	if err == nil {
		t.Fatal("writeInboxItems() = nil with a colliding file on disk — the remediation item was dropped while the caller was told it was queued")
	}
	if !strings.Contains(err.Error(), item.ID) {
		t.Errorf("the error must name the dropped item's id; got %v", err)
	}
}

func TestWriteInboxItems_IdenticalItemIsIdempotent(t *testing.T) {
	dir, item := inboxFixture(t)
	cfg := writeConfig{inboxDir: dir, inboxItems: []InboxItem{item}}
	if err := cfg.writeInboxItems(); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := cfg.writeInboxItems(); err != nil {
		t.Fatalf("second write of an identical item must be idempotent; got %v", err)
	}
}
