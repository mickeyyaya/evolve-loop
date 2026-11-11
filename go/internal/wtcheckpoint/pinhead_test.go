package wtcheckpoint_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

func TestPinHead_RecordsACheckpointThatHoldsTheHeadOfACleanTree(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	h.git(w.Dir, "checkout", "-q", "--detach")
	head := h.git(w.Dir, "rev-parse", "HEAD")

	res, err := wtcheckpoint.PinHead(context.Background(), w, wtcheckpoint.SaveOptions{Now: clockAt("20261106T060846Z")})
	if err != nil {
		t.Fatal(err)
	}

	if res.Status != wtcheckpoint.StatusSaved || res.Ref != "refs/checkpoints/task/20261106T060846Z" {
		t.Fatalf("PinHead = %+v, want saved at refs/checkpoints/task/20261106T060846Z", res)
	}
	if got := h.git(w.Dir, "rev-parse", res.Ref+"^^"); got != head {
		t.Errorf("the staged commit's parent = %s, want HEAD %s", got, head)
	}
	if got := h.git(h.store.Dir, "for-each-ref", "--format=%(refname)", "--contains", head, "refs/checkpoints/"); got != res.Ref {
		t.Errorf("refs holding HEAD = %q, want %q", got, res.Ref)
	}
	entries, err := wtcheckpoint.List(context.Background(), h.hub(), "task")
	if err != nil || len(entries) != 1 {
		t.Fatalf("List = %+v, %v; want the pinned head listed as one checkpoint", entries, err)
	}
}

func TestPinHead_PrunesBeyondTheKeepCount(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	clock := clockAt("20261106T060000Z", "20261106T060100Z")
	for range 2 {
		if _, err := wtcheckpoint.PinHead(context.Background(), w, wtcheckpoint.SaveOptions{Keep: 1, Now: clock}); err != nil {
			t.Fatal(err)
		}
	}
	if got := refsIn(t, h); got != "refs/checkpoints/task/20261106T060100Z" {
		t.Fatalf("refs = %q, want only the newest", got)
	}
}

func TestPinHead_RefusesAWorktreeNameThatCannotNameARef(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	w.Name = "bad name"
	if _, err := wtcheckpoint.PinHead(context.Background(), w, wtcheckpoint.SaveOptions{}); err == nil || !strings.Contains(err.Error(), "cannot name a ref") {
		t.Fatalf("err = %v, want a refusal of the worktree name", err)
	}
}

func TestPinHead_FailsWhenTheDirectoryIsNotARepository(t *testing.T) {
	t.Parallel()
	w := wtcheckpoint.Worktree{Dir: t.TempDir(), Name: "gone"}
	if _, err := wtcheckpoint.PinHead(context.Background(), w, wtcheckpoint.SaveOptions{}); err == nil {
		t.Fatal("PinHead of a directory with no repository succeeded")
	}
}

func TestPinHead_FailsWhenTheCheckpointRefAlreadyExists(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	h.store.Git("update-ref", "refs/checkpoints/task/20261106T060000Z", "main")
	if _, err := wtcheckpoint.PinHead(context.Background(), w, wtcheckpoint.SaveOptions{Now: clockAt("20261106T060000Z")}); err == nil {
		t.Fatal("PinHead overwrote an existing checkpoint ref")
	}
}
