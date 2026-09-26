package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

// startRefFixture builds a bare origin with one commit on main and a runtime
// clone checked out on main, returning (seed, runtime).
func startRefFixture(t *testing.T) (*gittest.Repo, *gittest.Repo) {
	t.Helper()
	origin := gittest.Bare(t)
	seed := gittest.Fixture(t)
	writeCommit(t, seed, "base.txt", "base")
	seed.Git("remote", "add", "origin", origin.Dir)
	seed.Git("push", "-q", "origin", "main")
	return seed, gittest.Clone(t, origin.Dir)
}

func writeCommit(t *testing.T, r *gittest.Repo, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(r.Dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	r.Git("add", name)
	r.Git("commit", "-q", "-m", "add "+name)
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
		ref, err := laneStartRef(ctx, runtime.Dir)
		if err != nil || ref != "origin/main" {
			t.Fatalf("ref=%q err=%v, want origin/main", ref, err)
		}
	})
	t.Run("remote ahead ⇒ origin/main", func(t *testing.T) {
		seed, runtime := startRefFixture(t)
		writeCommit(t, seed, "remote.txt", "r")
		seed.Git("push", "-q", "origin", "main")
		ref, err := laneStartRef(ctx, runtime.Dir)
		if err != nil || ref != "origin/main" {
			t.Fatalf("ref=%q err=%v, want origin/main", ref, err)
		}
	})
	t.Run("local ahead ⇒ the local main HEAD", func(t *testing.T) {
		_, runtime := startRefFixture(t)
		writeCommit(t, runtime, "dossier.txt", "closeout")
		want := runtime.Git("rev-parse", "HEAD")
		ref, err := laneStartRef(ctx, runtime.Dir)
		if err != nil || ref != want {
			t.Fatalf("ref=%q err=%v, want the local main HEAD %s", ref, err, want)
		}
	})
	t.Run("diverged ⇒ loud refusal", func(t *testing.T) {
		seed, runtime := startRefFixture(t)
		writeCommit(t, runtime, "dossier.txt", "closeout")
		writeCommit(t, seed, "remote.txt", "r")
		seed.Git("push", "-q", "origin", "main")
		ref, err := laneStartRef(ctx, runtime.Dir)
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "diverged") {
			t.Fatalf("ref=%q err=%v, want a diverged refusal", ref, err)
		}
	})
}
