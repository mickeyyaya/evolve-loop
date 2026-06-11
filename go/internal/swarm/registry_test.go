package swarm

import (
	"os"
	"path/filepath"
	"testing"
)

func handle(id string) SessionHandle {
	return SessionHandle{WorkerID: id, Agent: "build-" + id, TmuxSession: "sess-" + id,
		PGID: 1000, Worktree: "/wt/" + id, Branch: "cycle-1-" + id}
}

func TestSessionRegistry_RegisterSnapshotRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	r := NewSessionRegistry(path, 1, "build", 4242)
	if err := r.Register(handle("w1")); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(handle("w0")); err != nil {
		t.Fatal(err)
	}
	snap := r.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("want 2 sessions, got %d", len(snap))
	}
	// Snapshot is sorted by WorkerID for determinism.
	if snap[0].WorkerID != "w0" || snap[1].WorkerID != "w1" {
		t.Errorf("snapshot not sorted: %v", []string{snap[0].WorkerID, snap[1].WorkerID})
	}
	if snap[0].Status != StatusLive {
		t.Errorf("registered session should be Live, got %q", snap[0].Status)
	}
}

func TestSessionRegistry_ManifestPersistsAndLoads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "sessions.json") // dir auto-created
	r := NewSessionRegistry(path, 7, "build", 99)
	if err := r.Register(handle("w0")); err != nil {
		t.Fatal(err)
	}
	cycle, phase, pid, sessions, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if cycle != 7 || phase != "build" || pid != 99 {
		t.Errorf("manifest header wrong: cycle=%d phase=%q pid=%d", cycle, phase, pid)
	}
	if len(sessions) != 1 || sessions[0].WorkerID != "w0" || sessions[0].Branch != "cycle-1-w0" {
		t.Errorf("persisted session wrong: %+v", sessions)
	}
}

func TestSessionRegistry_MarkReaped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	r := NewSessionRegistry(path, 1, "build", 1)
	_ = r.Register(handle("w0"))
	_ = r.Register(handle("w1"))
	if err := r.MarkReaped("w0"); err != nil {
		t.Fatal(err)
	}
	live := r.Live()
	if len(live) != 1 || live[0].WorkerID != "w1" {
		t.Errorf("after reaping w0, only w1 should be live: %+v", live)
	}
	// Persisted state reflects the reap.
	_, _, _, sessions, _ := LoadManifest(path)
	var w0 SessionHandle
	for _, s := range sessions {
		if s.WorkerID == "w0" {
			w0 = s
		}
	}
	if w0.Status != StatusReaped {
		t.Errorf("w0 should be persisted as reaped, got %q", w0.Status)
	}
}

func TestSessionRegistry_RegisterIdempotent(t *testing.T) {
	r := NewSessionRegistry(filepath.Join(t.TempDir(), "s.json"), 1, "build", 1)
	_ = r.Register(handle("w0"))
	h := handle("w0")
	h.PGID = 5555 // re-register same ID with new pgid (retry)
	_ = r.Register(h)
	snap := r.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("re-register same worker_id must replace, got %d entries", len(snap))
	}
	if snap[0].PGID != 5555 {
		t.Errorf("re-register should update fields, pgid=%d", snap[0].PGID)
	}
}

func TestLoadManifest_MissingFileIsEmpty(t *testing.T) {
	cycle, _, _, sessions, err := LoadManifest(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("missing manifest must not error: %v", err)
	}
	if cycle != 0 || sessions != nil {
		t.Errorf("missing manifest should be empty, got cycle=%d sessions=%v", cycle, sessions)
	}
}

func TestLoadManifest_CorruptJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("not json {{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, _, _, err := LoadManifest(path)
	if err == nil {
		t.Error("corrupt manifest must return error")
	}
}

// TestPersistLocked_InMemoryMode covers the empty-manifestPath fast path.
func TestPersistLocked_InMemoryMode(t *testing.T) {
	// empty path → in-memory mode; persist is a no-op and must return nil
	r := NewSessionRegistry("", 1, "build", 1)
	if err := r.Register(handle("w0")); err != nil {
		t.Fatalf("in-memory register must not error: %v", err)
	}
}

// TestPersistLocked_UnwritableDir covers the MkdirAll error path.
func TestPersistLocked_UnwritableDir(t *testing.T) {
	// Create a read-only parent directory so MkdirAll on the subdir fails.
	parent := t.TempDir()
	if err := os.Chmod(parent, 0o555); err != nil {
		t.Skip("cannot set read-only dir on this system")
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })

	path := filepath.Join(parent, "sub", "sessions.json")
	r := NewSessionRegistry(path, 1, "build", 1)
	if err := r.Register(handle("w0")); err == nil {
		t.Error("persist to unwritable dir must return error")
	}
}
