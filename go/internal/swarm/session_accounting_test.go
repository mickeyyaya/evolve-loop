package swarm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// blockedRegistry's manifest parent is a regular file, so every persist fails for real, with no mock.
func blockedRegistry(t *testing.T) *SessionRegistry {
	t.Helper()
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed blocker file: %v", err)
	}
	return NewSessionRegistry(filepath.Join(blocker, "s.json"), 1, "build", os.Getpid())
}

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

func TestReap_MarkReapedFailure_SurfacedInErrors(t *testing.T) {
	dir := t.TempDir()
	reg := NewSessionRegistry(filepath.Join(dir, "sub", "s.json"), 1, "build", os.Getpid())
	if err := reg.Register(handle("w0")); err != nil {
		t.Fatalf("setup register (should succeed): %v", err)
	}
	// Replace the manifest's parent dir with a file so the next persist fails.
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

func TestMarkReaped_PersistFailure_RollsBackStatus(t *testing.T) {
	dir := t.TempDir()
	reg := NewSessionRegistry(filepath.Join(dir, "sub", "s.json"), 1, "build", os.Getpid())
	if err := reg.Register(handle("w0")); err != nil {
		t.Fatalf("setup register (should succeed): %v", err)
	}
	// Replace the manifest's parent dir with a file so the next persist fails.
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

func TestRegister_ReplacePersistFailure_RollsBackToOriginal(t *testing.T) {
	dir := t.TempDir()
	reg := NewSessionRegistry(filepath.Join(dir, "sub", "s.json"), 1, "build", os.Getpid())
	if err := reg.Register(handle("w0")); err != nil { // original Branch: cycle-1-w0
		t.Fatalf("setup register (should succeed): %v", err)
	}
	// Replace the manifest's parent dir with a file so the next persist fails.
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
