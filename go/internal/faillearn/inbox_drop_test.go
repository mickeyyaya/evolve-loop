package faillearn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// inbox_drop_test.go — cycle-1282 DEF-4 regression lock. Inbox filenames are
// fully deterministic (`retro-<cycle>-<slug>` over agent-authored defect text),
// so a concurrent fleet lane at standing width 3, or stale state from an earlier
// run of the same cycle number, can already hold the name. writeIfAbsent
// returned nil there and WriteArtifacts still reported success: the real
// remediation item was DROPPED with no error, no diagnostic, no telemetry —
// reproducing the exact 1255 state ("filed" in the report, absent from the
// queue) that this package exists to make unreachable.

func inboxFixture(t *testing.T) (inboxDir string, item InboxItem) {
	t.Helper()
	return t.TempDir(), InboxItem{
		ID: "retro-1255-stale-worktree", Title: "stale cs.ActiveWorktree survives fleet teardown",
		Weight: 0.9, Kind: "bug", Priority: "H", InjectedBy: "faillearn-failure-floor",
	}
}

// TestWriteInboxItems_CollidingFilenameIsNotSilentlyDropped is the lock: a
// DIFFERENT item already occupying our id must fail loudly rather than let the
// caller believe the remediation was queued.
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

// TestWriteInboxItems_IdenticalItemIsIdempotent — the POSITIVE half. The floor
// re-runs; a byte-identical item already on disk is a retry, not a collision,
// and must not turn a recoverable re-run into a hard failure.
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
