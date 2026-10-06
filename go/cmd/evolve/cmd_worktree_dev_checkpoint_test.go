package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func gitIn(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

type checkpointPair struct{ ref, staged, full string }

func saveCheckpoint(t *testing.T, h landedHub, dir, name, stamp string) checkpointPair {
	t.Helper()
	head := gitIn(t, dir, nil, "rev-parse", "HEAD")
	staged := gitIn(t, dir, nil, "commit-tree", gitIn(t, dir, nil, "write-tree"), "-p", head, "-m", "checkpoint: staged")
	index := []string{"GIT_INDEX_FILE=" + filepath.Join(t.TempDir(), "index")}
	gitIn(t, dir, index, "read-tree", "HEAD")
	gitIn(t, dir, index, "add", "-A")
	full := gitIn(t, dir, nil, "commit-tree", gitIn(t, dir, index, "write-tree"), "-p", staged, "-m", "checkpoint: full tree")
	ref := "refs/checkpoints/" + name + "/" + stamp
	gitIn(t, h.store, nil, "update-ref", ref, full)
	return checkpointPair{ref: ref, staged: staged, full: full}
}

func assertCheckpointIntact(t *testing.T, store string, cp checkpointPair) {
	t.Helper()
	cmd := exec.Command("git", "-C", store, "rev-parse", "--verify", "--quiet", cp.ref)
	out, err := cmd.Output()
	if got := strings.TrimSpace(string(out)); err != nil || got != cp.full {
		t.Errorf("checkpoint %s = %q (err=%v), want %s", cp.ref, got, err, cp.full)
	}
	for _, sha := range []string{cp.staged, cp.full} {
		if exec.Command("git", "-C", store, "cat-file", "-e", sha+"^{commit}").Run() != nil {
			t.Errorf("checkpoint commit %s of %s is gone from the store", sha, cp.ref)
		}
	}
}

func TestWorktreeDevCleanup_RemovingALandedTreeKeepsEveryCheckpointRefAndItsCommits(t *testing.T) {
	t.Parallel()
	h := newLandedHub(t)
	dir := h.dev(t, "t1", "b1")
	older := saveCheckpoint(t, h, dir, "t1", "20261006T010000Z")
	writeDevFile(t, dir, "feature.txt", "patch landed by the train\n")
	brGit(t, dir, "add", "feature.txt")
	newest := saveCheckpoint(t, h, dir, "t1", "20261006T020000Z")
	other := h.dev(t, "t2", "b2")
	writeDevFile(t, other, "draft.txt", "another lane's draft\n")
	sibling := saveCheckpoint(t, h, other, "t2", "20261006T020000Z")
	h.land(t, "feature.txt", "patch landed by the train\n", "train: lane patch")

	out, code := cleanupAll(t, h, devCleanupOptions{now: futureNow()})

	if code != 0 || !strings.Contains(out, "removed dev/t1") {
		t.Fatalf("--all exit=%d, want dev/t1 removed\n%s", code, out)
	}
	for _, cp := range []checkpointPair{older, newest, sibling} {
		assertCheckpointIntact(t, h.store, cp)
	}
	assertDevKept(t, h, other, "b2", out)
}

func TestWorktreeDevCleanup_AnUnlandedTreeIsKeptEvenWhenItsNewestCheckpointHoldsIt(t *testing.T) {
	t.Parallel()
	h := newLandedHub(t)
	dirty := h.dev(t, "dirty", "b-dirty")
	writeDevFile(t, dirty, "draft.txt", "unlanded draft, checkpointed\n")
	saveCheckpoint(t, h, dirty, "dirty", "20261006T020000Z")
	committed := h.dev(t, "committed", "b-committed")
	brCommit(t, committed, "work.txt", "committed, unlanded, checkpointed\n", "lane: work")
	saveCheckpoint(t, h, committed, "committed", "20261006T020000Z")

	for task, dir := range map[string]string{"dirty": dirty, "committed": committed} {
		out, code := cleanupOne(t, h, task, devCleanupOptions{})
		if code != 1 || !strings.Contains(out, "not in origin/main") {
			t.Errorf("%s: cleanup of an unlanded tree whose newest checkpoint holds it exit=%d, want 1: a checkpoint never licenses a removal\n%s", task, code, out)
		}
		assertDevKept(t, h, dir, "b-"+task, out)
	}
}

func futureNow() time.Time { return time.Now().Add(3 * time.Hour) }
