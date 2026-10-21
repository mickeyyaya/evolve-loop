package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

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

func TestLaneStartRef_IntegrationHeadAuthority(t *testing.T) {
	t.Parallel()

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
	t.Run("local ahead by boundary commits ⇒ the local main HEAD", func(t *testing.T) {
		_, runtime := startRefFixture(t)
		commitPath(t, runtime, "knowledge-base/cycles/cycle-9.md", "closeout")
		commitPath(t, runtime, ".evolve/inbox/a.json", "{}")
		want := runtime.Git("rev-parse", "HEAD")
		ref, err := laneStartRef(ctx, runtime.Dir)
		if err != nil || ref != want {
			t.Fatalf("ref=%q err=%v, want the local main HEAD %s", ref, err, want)
		}
	})
	t.Run("local ahead by a commit no boundary step wrote ⇒ loud refusal naming it", func(t *testing.T) {
		_, runtime := startRefFixture(t)
		commitPath(t, runtime, "knowledge-base/cycles/cycle-9.md", "closeout")
		writeCommit(t, runtime, "stranded.go", "package x")
		stranded := runtime.Git("rev-parse", "--short=12", "HEAD")
		ref, err := laneStartRef(ctx, runtime.Dir)
		if err == nil || !strings.Contains(err.Error(), stranded) {
			t.Fatalf("ref=%q err=%v, want a refusal naming %s: a lane must not fork from a commit origin does not hold (cycle 1830)", ref, err, stranded)
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
