package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// startRefFixture builds a bare origin with one commit on main and a runtime
// clone checked out on main, returning (origin, runtime).
func startRefFixture(t *testing.T) (string, string) {
	t.Helper()
	origin := filepath.Join(t.TempDir(), "origin.git")
	seed := t.TempDir()
	runtime := filepath.Join(t.TempDir(), "runtime")
	gitAt(t, "", "init", "-q", "--bare", "-b", "main", origin)
	gitAt(t, seed, "init", "-q", "-b", "main")
	gitAt(t, seed, "config", "user.email", "t@example.com")
	gitAt(t, seed, "config", "user.name", "t")
	writeCommit(t, seed, "base.txt", "base")
	gitAt(t, seed, "remote", "add", "origin", origin)
	gitAt(t, seed, "push", "-q", "origin", "main")
	gitAt(t, "", "clone", "-q", origin, runtime)
	gitAt(t, runtime, "config", "user.email", "t@example.com")
	gitAt(t, runtime, "config", "user.name", "t")
	return seed, runtime
}

func gitAt(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeCommit(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAt(t, dir, "add", name)
	gitAt(t, dir, "commit", "-q", "-m", "add "+name)
}

// TestLaneStartRef_IntegrationHeadAuthority — 2026-09-09 token-waste root cause
// #3: a fresh lane based on origin/main while the landing branch (the local
// main, AHEAD by unpushed dossier closeouts) sat elsewhere, so Ship rebased and
// re-dispatched Build/Audit for a diff of twelve dossier files. The lane base
// is the INTEGRATION HEAD the landing targets: origin/main when the local main
// is current or behind (the boundary fast-forwards it), the local main when it
// is strictly ahead (its unpublished landings ride the next push), and a
// loud refusal when the two have diverged — no base choice avoids a rebase
// then, so the plane must be reconciled before any spend.
func TestLaneStartRef_IntegrationHeadAuthority(t *testing.T) {
	ctx := context.Background()
	t.Run("current ⇒ origin/main", func(t *testing.T) {
		_, runtime := startRefFixture(t)
		ref, err := laneStartRef(ctx, runtime)
		if err != nil || ref != "origin/main" {
			t.Fatalf("ref=%q err=%v, want origin/main", ref, err)
		}
	})
	t.Run("remote ahead ⇒ origin/main", func(t *testing.T) {
		seed, runtime := startRefFixture(t)
		writeCommit(t, seed, "remote.txt", "r")
		gitAt(t, seed, "push", "-q", "origin", "main")
		ref, err := laneStartRef(ctx, runtime)
		if err != nil || ref != "origin/main" {
			t.Fatalf("ref=%q err=%v, want origin/main", ref, err)
		}
	})
	t.Run("local ahead ⇒ the local main HEAD", func(t *testing.T) {
		_, runtime := startRefFixture(t)
		writeCommit(t, runtime, "dossier.txt", "closeout")
		want := gitAt(t, runtime, "rev-parse", "HEAD")
		ref, err := laneStartRef(ctx, runtime)
		if err != nil || ref != want {
			t.Fatalf("ref=%q err=%v, want the local main HEAD %s", ref, err, want)
		}
	})
	t.Run("diverged ⇒ loud refusal", func(t *testing.T) {
		seed, runtime := startRefFixture(t)
		writeCommit(t, runtime, "dossier.txt", "closeout")
		writeCommit(t, seed, "remote.txt", "r")
		gitAt(t, seed, "push", "-q", "origin", "main")
		ref, err := laneStartRef(ctx, runtime)
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "diverged") {
			t.Fatalf("ref=%q err=%v, want a diverged refusal", ref, err)
		}
	})
}
