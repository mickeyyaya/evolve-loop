package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func syncFetchFixture(t *testing.T) (seed, runtime *gittest.Repo) {
	t.Helper()
	origin := gittest.Bare(t)
	seed = gittest.Fixture(t)
	smCommitFile(t, seed, ".gitignore", ".evolve/\n")
	seed.Git("remote", "add", "origin", origin.Dir)
	seed.Git("push", "-q", "origin", "main", "main:train/wave64-boundary")
	return seed, gittest.Clone(t, origin.Dir)
}

func smCommitFile(t *testing.T, r *gittest.Repo, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(r.Dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	r.Git("add", name)
	r.Git("commit", "-q", "-m", "add "+name)
}

func holdRefLock(t *testing.T, r *gittest.Repo, ref string) string {
	t.Helper()
	lock := filepath.Join(r.Dir, ".git", filepath.FromSlash(ref)+".lock")
	if err := os.MkdirAll(filepath.Dir(lock), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(lock) })
	return lock
}

func TestSyncMain_ALockOnAnUnrelatedRemoteRefDoesNotFailTheSync(t *testing.T) {
	seed, runtime := syncFetchFixture(t)
	smCommitFile(t, seed, "landed.txt", "landed\n")
	seed.Git("push", "-q", "origin", "main", "+main:train/wave64-boundary")
	holdRefLock(t, runtime, "refs/remotes/origin/train/wave64-boundary")

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", runtime.Dir}, nil, &out, &errb)

	if code != 0 {
		t.Fatalf("sync-main exit %d: a held lock on an unrelated remote-tracking ref must not fail the sync\n%s", code, errb.String())
	}
	if got, want := runtime.Git("rev-parse", "HEAD"), seed.Git("rev-parse", "HEAD"); got != want {
		t.Errorf("HEAD = %s, want the landed origin/main %s", got, want)
	}
}

func TestSyncMain_RetriesALockOnTheBranchRef(t *testing.T) {
	seed, runtime := syncFetchFixture(t)
	smCommitFile(t, seed, "landed.txt", "landed\n")
	seed.Git("push", "-q", "origin", "main")
	lock := holdRefLock(t, runtime, "refs/remotes/origin/main")
	prev := syncMainFetchSleep
	t.Cleanup(func() { syncMainFetchSleep = prev })
	backoffs := 0
	syncMainFetchSleep = func(time.Duration) {
		backoffs++
		os.Remove(lock)
	}

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", runtime.Dir}, nil, &out, &errb)

	if code != 0 || backoffs != 1 {
		t.Fatalf("exit %d after %d backoffs, want 0 after one: ref-lock contention on origin/main is retried\n%s", code, backoffs, errb.String())
	}
	if !strings.Contains(errb.String(), "evolve sync-main: retry 1/2: git fetch origin main after ref-lock contention") {
		t.Errorf("the retry is announced, stderr=%q", errb.String())
	}
}

func TestSyncMain_RefusesADetachedHEADBeforeFetching(t *testing.T) {
	seed, runtime := syncFetchFixture(t)
	smCommitFile(t, seed, "landed.txt", "landed\n")
	seed.Git("push", "-q", "origin", "main")
	runtime.Git("checkout", "-q", "--detach")
	before := runtime.Git("rev-parse", "origin/main")

	var out, errb bytes.Buffer
	code := runSyncMain([]string{"--project-root", runtime.Dir}, nil, &out, &errb)

	if code != 1 || !strings.Contains(errb.String(), "detached HEAD; check out the branch to sync") {
		t.Fatalf("exit %d, stderr %q: a detached HEAD has no branch to sync and must be refused by name", code, errb.String())
	}
	if after := runtime.Git("rev-parse", "origin/main"); after != before {
		t.Errorf("origin/main moved %s -> %s: the refusal must come before any fetch", before, after)
	}
	if got := runtime.Git("symbolic-ref", "refs/remotes/origin/HEAD"); got != "refs/remotes/origin/main" {
		t.Errorf("refs/remotes/origin/HEAD = %q, want the untouched symbolic ref", got)
	}
}
