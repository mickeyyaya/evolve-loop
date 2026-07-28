package main

// cmd_loop_wavesync_test.go — ADR-0080 S3: the runtime plane refreshes `main`
// from origin at wave boundaries, fast-forward ONLY. Real git fixtures: a
// bare origin plus two clones (the runtime and a "console" that lands work
// via origin), because the failure modes under test are git's own (non-FF
// divergence, missing remote, detached branch).

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitrun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// syncFixture: bare origin with one commit on main; runtime clone on main.
func syncFixture(t *testing.T) (origin, runtime string) {
	t.Helper()
	seed := t.TempDir()
	gitrun(t, seed, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(seed, "f.txt"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitrun(t, seed, "add", "f.txt")
	gitrun(t, seed, "commit", "-q", "-m", "c1")
	origin = filepath.Join(t.TempDir(), "origin.git")
	gitrun(t, seed, "clone", "-q", "--bare", ".", origin)
	runtime = filepath.Join(t.TempDir(), "runtime")
	gitrun(t, filepath.Dir(runtime), "clone", "-q", origin, runtime)
	return origin, runtime
}

// originAdvance lands a new commit on origin main via a scratch clone (the
// console-plane landing path).
func originAdvance(t *testing.T, origin string) {
	t.Helper()
	c := filepath.Join(t.TempDir(), "console")
	gitrun(t, filepath.Dir(c), "clone", "-q", origin, c)
	if err := os.WriteFile(filepath.Join(c, "g.txt"), []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitrun(t, c, "add", "g.txt")
	gitrun(t, c, "commit", "-q", "-m", "c2")
	gitrun(t, c, "push", "-q", "origin", "main")
}

func TestSyncMainAtWaveBoundary_FastForwards(t *testing.T) {
	origin, runtime := syncFixture(t)
	originAdvance(t, origin)
	var warn bytes.Buffer
	if synced := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn); !synced {
		t.Fatalf("expected FF sync, got skip: %s", warn.String())
	}
	if _, err := os.Stat(filepath.Join(runtime, "g.txt")); err != nil {
		t.Fatalf("runtime tree not fast-forwarded to origin: %v", err)
	}
}

func TestSyncMainAtWaveBoundary_AlreadyCurrentIsQuietNoop(t *testing.T) {
	_, runtime := syncFixture(t)
	var warn bytes.Buffer
	if synced := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn); synced {
		t.Fatal("no origin movement must report synced=false (nothing to do)")
	}
	if s := warn.String(); strings.Contains(s, "WARN") {
		t.Errorf("up-to-date must not WARN: %s", s)
	}
}

func TestSyncMainAtWaveBoundary_LocalAheadSkipsLoudly(t *testing.T) {
	origin, runtime := syncFixture(t)
	// Local commit not on origin (unpushed dossier shape) + origin also moves.
	if err := os.WriteFile(filepath.Join(runtime, "local.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitrun(t, runtime, "add", "local.txt")
	gitrun(t, runtime, "commit", "-q", "-m", "local")
	originAdvance(t, origin)
	var warn bytes.Buffer
	if synced := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn); synced {
		t.Fatal("diverged history must never merge or rebase — FF-only")
	}
	if !strings.Contains(warn.String(), "WARN") {
		t.Errorf("a skipped diverged sync must be loud: %q", warn.String())
	}
}

func TestSyncMainAtWaveBoundary_NotOnMainSkips(t *testing.T) {
	_, runtime := syncFixture(t)
	gitrun(t, runtime, "checkout", "-q", "-b", "feature")
	var warn bytes.Buffer
	if synced := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn); synced {
		t.Fatal("a non-main checkout must never be synced")
	}
}

func TestSyncMainAtWaveBoundary_NoRemoteSkipsWithoutError(t *testing.T) {
	seed := t.TempDir()
	gitrun(t, seed, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(seed, "f.txt"), []byte("v"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitrun(t, seed, "add", "f.txt")
	gitrun(t, seed, "commit", "-q", "-m", "c")
	var warn bytes.Buffer
	if synced := syncMainFromOriginAtWaveBoundary(context.Background(), seed, &warn); synced {
		t.Fatal("no remote must be a quiet skip, not a sync")
	}
}

// TestSyncMainAtWaveBoundary_DirtyTrackedFileWarnsBlockedNotDiverged pins the
// review-HIGH misdiagnosis: --ff-only refused by LOCAL TRACKED CHANGES must
// name that cause — the "diverged" prescription ("next ship reconciles")
// invites the ship to adopt the dirt (the stowaway class).
func TestSyncMainAtWaveBoundary_DirtyTrackedFileWarnsBlockedNotDiverged(t *testing.T) {
	origin, runtime := syncFixture(t)
	originAdvance(t, origin)
	// A local file at the path origin's new commit ADDS: git refuses the FF
	// with the would-be-overwritten error — the binary-churn shape.
	if err := os.WriteFile(filepath.Join(runtime, "g.txt"), []byte("local dirt"), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	if synced := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn); synced {
		t.Fatalf("FF over conflicting local changes must not report synced: %s", warn.String())
	}
	if s := warn.String(); !strings.Contains(s, "local tracked changes block") || strings.Contains(s, "diverged") {
		t.Errorf("blocked-by-dirt must be named as such, never as divergence: %q", s)
	}
}
