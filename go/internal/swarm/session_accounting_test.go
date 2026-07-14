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
