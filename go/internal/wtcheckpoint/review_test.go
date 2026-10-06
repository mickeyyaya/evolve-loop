package wtcheckpoint_test

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

func TestSave_ComparesAgainstTheNewestCheckpointNotAnOlderOne(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	o := wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T060000Z", "20261006T070000Z", "20261006T080000Z")}
	writeFile(t, w.Dir, "a.txt", "A\n")
	save(t, w, o)
	writeFile(t, w.Dir, "a.txt", "B\n")
	second := save(t, w, o)

	third := save(t, w, o)

	if third.Status != wtcheckpoint.StatusUnchanged || third.Ref != second.Ref {
		t.Errorf("third save = %+v, want unchanged against the newest %s", third, second.Ref)
	}
}

func TestRestore_IntoACleanWorktreeOnAnotherCommitDetachesAtTheBaseAndKeepsTheBranchRef(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	dirtyThreeWays(t, h, w.Dir)
	want := captureWork(t, w.Dir)
	base := h.git(w.Dir, "rev-parse", "HEAD")
	saved := save(t, w, wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T060000Z")})
	h.git(w.Dir, "add", "-A")
	h.git(w.Dir, "commit", "-q", "-m", "moved on")
	moved := h.git(w.Dir, "rev-parse", "HEAD")

	res := restore(t, h, saved.Ref, w.Dir)

	if res.Created || !res.Detached {
		t.Errorf("Restore = %+v, want the existing worktree reused and reported detached", res)
	}
	if got := h.git(w.Dir, "rev-parse", "HEAD"); got != base {
		t.Errorf("HEAD = %s, want the checkpoint's base %s", got, base)
	}
	if got := h.git(w.Dir, "rev-parse", "refs/heads/feat/task"); got != moved {
		t.Errorf("feat/task = %s, want it left at %s", got, moved)
	}
	if got := captureWork(t, w.Dir); got != want {
		t.Errorf("restored work differs:\n got %+v\nwant %+v", got, want)
	}
}

func TestPrune_LandedKeepsACheckpointThatChangedNothingAgainstItsBase(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	writeFile(t, w.Dir, "a.txt", "staged only\n")
	h.git(w.Dir, "add", "a.txt")
	writeFile(t, w.Dir, "a.txt", "a\n")
	saved := save(t, w, wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T060000Z")})
	if saved.Status != wtcheckpoint.StatusSaved {
		t.Fatalf("a staged-only change saved as %s, want saved", saved.Status)
	}

	pruned, err := wtcheckpoint.Prune(context.Background(), h.hub(), wtcheckpoint.PruneOptions{Keep: 20, Landed: true})

	if err != nil || len(pruned) != 0 {
		t.Errorf("Prune --landed = %+v, %v; want the staged-only checkpoint kept: no changed path proves nothing landed", pruned, err)
	}
}

func TestPrune_ARefPastRetentionThatAlsoLandedIsDeletedOnce(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	o := wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T060000Z", "20261006T070000Z")}
	writeFile(t, w.Dir, "b.txt", "first\n")
	save(t, w, o)
	writeFile(t, w.Dir, "c.txt", "second\n")
	save(t, w, o)
	landing := h.addWorktree("dev/landing", "feat/landing")
	writeFile(t, landing.Dir, "b.txt", "first\n")
	writeFile(t, landing.Dir, "c.txt", "second\n")
	h.git(landing.Dir, "add", "-A")
	h.git(landing.Dir, "commit", "-qm", "land")
	h.git(landing.Dir, "update-ref", "refs/remotes/origin/main", "HEAD")

	pruned, err := wtcheckpoint.Prune(context.Background(), h.hub(), wtcheckpoint.PruneOptions{Keep: 1, Landed: true})

	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	want := []wtcheckpoint.Pruned{
		{Ref: "refs/checkpoints/task/20261006T060000Z", Reason: wtcheckpoint.PruneRetention},
		{Ref: "refs/checkpoints/task/20261006T070000Z", Reason: wtcheckpoint.PruneLanded},
	}
	if len(pruned) != 2 || pruned[0] != want[0] || pruned[1] != want[1] {
		t.Errorf("Prune = %+v, want %+v", pruned, want)
	}
}

func TestList_CountsABinaryFileAsAFileWithNoLines(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	writeFile(t, w.Dir, "blob.bin", "\x00\x01\x02binary\x00")
	writeFile(t, w.Dir, "a.txt", "a\nb\n")
	save(t, w, wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T060000Z")})

	got, err := wtcheckpoint.List(context.Background(), h.hub(), w.Dir)

	if err != nil || len(got) != 1 {
		t.Fatalf("List = %+v, %v", got, err)
	}
	if got[0].Files != 2 || got[0].Added != 1 || got[0].Deleted != 0 {
		t.Errorf("entry = %+v, want 2 files (the binary one with no lines), +1 -0", got[0])
	}
}
