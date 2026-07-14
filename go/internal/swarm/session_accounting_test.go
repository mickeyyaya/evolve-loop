package swarm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// session_accounting_test.go — the two session-accounting persistence failures
// must FAIL LOUD, not be swallowed (inbox: swarm-session-accounting-fail-loud).
// Both use the same real-contract error injection: a registry whose manifest
// path has a regular FILE as its parent directory makes persistLocked's
// os.MkdirAll(filepath.Dir(path)) genuinely fail — no mock, the production
// persist path actually errors.

// blockedRegistry returns a SessionRegistry whose persistence is guaranteed to
// fail: its manifest's parent path is a regular file, so MkdirAll cannot create
// the directory. Every Register / MarkReaped on it returns a real error.
func blockedRegistry(t *testing.T) *SessionRegistry {
	t.Helper()
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed blocker file: %v", err)
	}
	return NewSessionRegistry(filepath.Join(blocker, "s.json"), 1, "build", os.Getpid())
}

// A worker whose pre-registration fails must be ABORTED before any spawn: the
// crash-safe reaper's manifest is its only record of live sessions, so
// launching an unregistered session would leak an invisible orphan. The
// dispatcher must return wr.Err and NEVER call the launcher.
func TestLaunchWorker_RegisterFailure_AbortsBeforeLaunch(t *testing.T) {
	fk := &fakeLauncher{}
	deps := Deps{Launcher: fk, Registry: blockedRegistry(t)}
	w := WorkerSpec{WorkerID: "w0", CLI: "claude-tmux"}

	wr := launchWorker(context.Background(), SwarmPlan{}, DispatchRequest{Cycle: 1, Workspace: t.TempDir()}, w, t.TempDir(), 0, deps)

	if wr.Err == nil || !strings.Contains(wr.Err.Error(), "pre-register") {
		t.Fatalf("Register failure must abort the worker with a pre-register error, got %v", wr.Err)
	}
	if len(fk.launched) != 0 {
		t.Errorf("launcher must NOT be called when pre-registration failed (no unregistered spawn); launched=%v", fk.launched)
	}
}

// When MarkReaped's persist fails during a sweep, the kill itself still
// succeeded (the process is dead) — so the worker stays in rep.Killed — but the
// stale-Live manifest entry (which would make the next sweep re-target a corpse)
// must be surfaced in rep.Errors, not swallowed.
func TestReap_MarkReapedFailure_SurfacedInErrors(t *testing.T) {
	dir := t.TempDir()
	reg := NewSessionRegistry(filepath.Join(dir, "sub", "s.json"), 1, "build", os.Getpid())
	if err := reg.Register(handle("w0")); err != nil {
		t.Fatalf("setup register (should succeed): %v", err)
	}
	// Now break persistence: replace the manifest's parent dir with a file so the
	// MarkReaped inside Reap fails.
	if err := os.RemoveAll(filepath.Join(dir, "sub")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	rep := Reap(context.Background(), reg, &fakeKiller{})

	if len(rep.Killed) != 1 || rep.Killed[0] != "w0" {
		t.Errorf("kill succeeded → worker stays in Killed, got %v", rep.Killed)
	}
	found := false
	for _, e := range rep.Errors {
		if strings.Contains(e, "mark-reaped") {
			found = true
		}
	}
	if !found {
		t.Fatalf("MarkReaped persist failure must surface in rep.Errors, got %v", rep.Errors)
	}
}

// A Register whose persist fails must NOT leave a phantom entry in memory: the
// on-disk manifest (the reaper's source of truth) never recorded it, so an
// in-memory Live entry would be a divergence the disk contradicts. The mutation
// rolls back to the pre-Register state.
func TestRegister_PersistFailure_RollsBackInMemory(t *testing.T) {
	reg := blockedRegistry(t)
	if err := reg.Register(handle("w0")); err == nil {
		t.Fatal("Register must surface the persist failure")
	}
	if snap := reg.Snapshot(); len(snap) != 0 {
		t.Fatalf("failed Register must roll back the in-memory entry (no phantom), got %+v", snap)
	}
	if live := reg.Live(); len(live) != 0 {
		t.Fatalf("no session may be Live after a failed Register, got %+v", live)
	}
}

// A MarkReaped whose persist fails must roll back the status flip: the on-disk
// manifest still shows the session Live, so memory must too (consistent), not
// claim a Reaped state that never reached disk.
func TestMarkReaped_PersistFailure_RollsBackStatus(t *testing.T) {
	dir := t.TempDir()
	reg := NewSessionRegistry(filepath.Join(dir, "sub", "s.json"), 1, "build", os.Getpid())
	if err := reg.Register(handle("w0")); err != nil {
		t.Fatalf("setup register (should succeed): %v", err)
	}
	// Break persistence: replace the manifest's parent dir with a file.
	if err := os.RemoveAll(filepath.Join(dir, "sub")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := reg.MarkReaped("w0"); err == nil {
		t.Fatal("MarkReaped must surface the persist failure")
	}
	live := reg.Live()
	if len(live) != 1 || live[0].WorkerID != "w0" {
		t.Fatalf("failed MarkReaped must roll back the status flip (w0 stays Live), got %+v", live)
	}
}

// Re-registering an existing WorkerID takes upsertLocked's in-place-replace
// branch; if that persist fails, the rollback must restore the ORIGINAL entry,
// not leave the half-applied replacement in memory.
func TestRegister_ReplacePersistFailure_RollsBackToOriginal(t *testing.T) {
	dir := t.TempDir()
	reg := NewSessionRegistry(filepath.Join(dir, "sub", "s.json"), 1, "build", os.Getpid())
	if err := reg.Register(handle("w0")); err != nil { // original Branch: cycle-1-w0
		t.Fatalf("setup register (should succeed): %v", err)
	}
	// Break persistence, then re-register w0 with a changed field.
	if err := os.RemoveAll(filepath.Join(dir, "sub")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed := handle("w0")
	changed.Branch = "cycle-1-w0-REPLACED"
	if err := reg.Register(changed); err == nil {
		t.Fatal("re-register with a failing persist must surface the error")
	}
	snap := reg.Snapshot()
	if len(snap) != 1 || snap[0].Branch != "cycle-1-w0" {
		t.Fatalf("failed re-register must roll back to the original entry, got %+v", snap)
	}
}
