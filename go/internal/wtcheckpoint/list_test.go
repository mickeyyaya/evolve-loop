package wtcheckpoint_test

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

func TestList_ShowsTimeLabelBaseAndTheChangeAgainstTheBase(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	task := h.addWorktree("dev/task", "feat/task")
	other := h.addWorktree("dev/other", "feat/other")
	dirtyThreeWays(t, h, task.Dir)
	writeFile(t, other.Dir, "b.txt", "b\nmore\n")
	save(t, task, wtcheckpoint.SaveOptions{Keep: 20, Label: "  before the\nrefactor ", Now: clockAt("20261006T060846Z")})
	save(t, other, wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T070000Z")})
	base := h.git(task.Dir, "rev-parse", "HEAD")

	all, err := wtcheckpoint.List(context.Background(), h.hub(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].Worktree != "other" || all[1].Worktree != "task" {
		t.Fatalf("List(all) = %+v, want other then task", all)
	}
	got, err := wtcheckpoint.List(context.Background(), h.hub(), "task")
	if err != nil {
		t.Fatal(err)
	}
	want := wtcheckpoint.Entry{
		Ref: "refs/checkpoints/task/20261006T060846Z", Worktree: "task",
		Time:  time.Date(2026, 10, 6, 6, 8, 46, 0, time.UTC),
		Label: "before the refactor", Base: base, Files: 3, Added: 3, Deleted: 1,
	}
	if len(got) != 1 || got[0] != want {
		t.Errorf("List(task) = %+v, want [%+v]", got, want)
	}
	if none, err := wtcheckpoint.List(context.Background(), h.hub(), "nobody"); err != nil || len(none) != 0 {
		t.Errorf("List(nobody) = %+v, %v; want empty", none, err)
	}
}
