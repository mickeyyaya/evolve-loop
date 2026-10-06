package wtcheckpoint_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

type workState struct{ status, staged, unstaged, untracked string }

func captureWork(t *testing.T, dir string) workState {
	t.Helper()
	status, _, _, err := gitIn(dir, "status", "--porcelain", "-uall")
	if err != nil {
		t.Fatal(err)
	}
	staged, _, _, _ := gitIn(dir, "diff", "--cached")
	unstaged, _, _, _ := gitIn(dir, "diff")
	untracked, err := os.ReadFile(filepath.Join(dir, "notes", "untracked.txt"))
	if err != nil {
		t.Fatalf("untracked file: %v", err)
	}
	return workState{status: status, staged: staged, unstaged: unstaged, untracked: string(untracked)}
}

func restore(t *testing.T, h hubFixture, ref, into string) wtcheckpoint.RestoreResult {
	t.Helper()
	res, err := wtcheckpoint.Restore(context.Background(), h.hub(), ref, into)
	if err != nil {
		t.Fatalf("Restore(%s, %s): %v", ref, into, err)
	}
	return res
}

func TestRestore_IntoANewPathBringsBackTheStagedUnstagedAndUntrackedSplit(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	dirtyThreeWays(t, h, w.Dir)
	want := captureWork(t, w.Dir)
	base := h.git(w.Dir, "rev-parse", "HEAD")
	saved := save(t, w, wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T060846Z")})
	into := filepath.Join(h.root, "dev", "task-restored")

	res := restore(t, h, strings.TrimPrefix(saved.Ref, "refs/checkpoints/"), into)

	if !res.Created || res.Base != base || res.Ref != saved.Ref {
		t.Errorf("Restore = %+v, want a created worktree at base %s from %s", res, base, saved.Ref)
	}
	if got := h.git(into, "rev-parse", "HEAD"); got != base {
		t.Errorf("restored HEAD = %s, want the checkpoint's base %s", got, base)
	}
	if got := captureWork(t, into); got != want {
		t.Errorf("restored work differs from the saved worktree:\n got %+v\nwant %+v", got, want)
	}
}

func TestRestore_IntoTheSameCleanWorktreeAfterAnAccidentKeepsItsBranch(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	dirtyThreeWays(t, h, w.Dir)
	want := captureWork(t, w.Dir)
	saved := save(t, w, wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T060846Z")})
	h.git(w.Dir, "reset", "-q", "--hard")
	h.git(w.Dir, "clean", "-qfd")

	res := restore(t, h, saved.Ref, w.Dir)

	if res.Created || res.Detached {
		t.Errorf("Restore = %+v, want the existing worktree reused and still on its branch", res)
	}
	if got := h.git(w.Dir, "symbolic-ref", "--short", "HEAD"); got != "feat/task" {
		t.Errorf("branch after restore = %q, want feat/task kept", got)
	}
	if got := captureWork(t, w.Dir); got != want {
		t.Errorf("restored work differs:\n got %+v\nwant %+v", got, want)
	}
}

func TestRestore_RefusesADirtyTargetAndLeavesItsWorkAlone(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	writeFile(t, w.Dir, "a.txt", "saved\n")
	saved := save(t, w, wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T060846Z")})
	writeFile(t, w.Dir, "a.txt", "newer unsaved work\n")
	writeFile(t, w.Dir, "fresh.txt", "untracked unsaved work\n")

	_, err := wtcheckpoint.Restore(context.Background(), h.hub(), saved.Ref, w.Dir)

	if err == nil || !strings.Contains(err.Error(), "refused") || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("Restore into a dirty tree err = %v, want a dirty refusal", err)
	}
	for rel, body := range map[string]string{"a.txt": "newer unsaved work\n", "fresh.txt": "untracked unsaved work\n"} {
		if got, err := os.ReadFile(filepath.Join(w.Dir, rel)); err != nil || string(got) != body {
			t.Errorf("%s after the refusal = %q, %v; want %q untouched", rel, got, err, body)
		}
	}
}

func TestRestore_RefusesAPathThatIsNotALinkedWorktreeAndAnUnknownRef(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	writeFile(t, w.Dir, "a.txt", "saved\n")
	saved := save(t, w, wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T060846Z")})
	plain := t.TempDir()
	writeFile(t, plain, "keep.txt", "not a worktree\n")

	if _, err := wtcheckpoint.Restore(context.Background(), h.hub(), saved.Ref, plain); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Errorf("Restore into a plain directory err = %v, want a refusal", err)
	}
	if _, err := wtcheckpoint.Restore(context.Background(), h.hub(), "task/19990101T000000Z", filepath.Join(h.root, "dev", "x")); err == nil {
		t.Error("Restore of an unknown checkpoint succeeded")
	}
	if _, err := os.Stat(filepath.Join(h.root, "dev", "x")); !os.IsNotExist(err) {
		t.Errorf("a failed restore left a worktree behind: %v", err)
	}
}
