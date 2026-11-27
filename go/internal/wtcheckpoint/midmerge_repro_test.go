package wtcheckpoint_test

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

func TestSave_MidMergeWorktreeIsSkippedNotFailed(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	other := h.addWorktree("dev/other", "feat/other")
	writeFile(t, other.Dir, "a.txt", "theirs\n")
	h.git(other.Dir, "commit", "-qam", "theirs")
	writeFile(t, w.Dir, "a.txt", "ours\n")
	h.git(w.Dir, "commit", "-qam", "ours")
	if _, _, code, _ := gitIn(w.Dir, "merge", "-q", "feat/other"); code == 0 {
		t.Fatal("the fixture merge did not conflict")
	}

	res, err := wtcheckpoint.Save(context.Background(), w, wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T060000Z")})

	if err != nil {
		t.Fatalf("Save of a mid-merge worktree returned an error, want a skip: %v", err)
	}
	if string(res.Status) != "skipped" {
		t.Errorf("status = %q, want skipped", res.Status)
	}
	if got := refsIn(t, h); got != "" {
		t.Errorf("refs after a skipped save = %s, want none", got)
	}
}
