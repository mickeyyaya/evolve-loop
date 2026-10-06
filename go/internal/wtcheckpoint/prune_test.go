package wtcheckpoint_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

func refsIn(t *testing.T, h hubFixture) string {
	t.Helper()
	return strings.Join(lines(h.git(h.store.Dir, "for-each-ref", "--format=%(refname)", "refs/checkpoints/")), ",")
}

func TestPrune_KeepsTheNewestNPerWorktree(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	task := h.addWorktree("dev/task", "feat/task")
	other := h.addWorktree("dev/other", "feat/other")
	o := wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T060000Z", "20261006T070000Z", "20261006T080000Z")}
	for _, body := range []string{"one\n", "two\n", "three\n"} {
		writeFile(t, task.Dir, "a.txt", body)
		save(t, task, o)
	}
	writeFile(t, other.Dir, "a.txt", "other\n")
	save(t, other, wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T050000Z")})

	pruned, err := wtcheckpoint.Prune(context.Background(), h.hub(), wtcheckpoint.PruneOptions{Keep: 1})
	if err != nil {
		t.Fatal(err)
	}

	want := []wtcheckpoint.Pruned{
		{Ref: "refs/checkpoints/task/20261006T060000Z", Reason: wtcheckpoint.PruneRetention},
		{Ref: "refs/checkpoints/task/20261006T070000Z", Reason: wtcheckpoint.PruneRetention},
	}
	if len(pruned) != 2 || pruned[0] != want[0] || pruned[1] != want[1] {
		t.Errorf("Prune = %+v, want %+v", pruned, want)
	}
	if got := refsIn(t, h); got != "refs/checkpoints/other/20261006T050000Z,refs/checkpoints/task/20261006T080000Z" {
		t.Errorf("refs left = %s", got)
	}
}

func TestPrune_LandedDeletesOnlyCheckpointsWhoseChangesAreAllOnOriginMain(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	landed := h.addWorktree("dev/landed", "feat/landed")
	partial := h.addWorktree("dev/partial", "feat/partial")
	open := h.addWorktree("dev/open", "feat/open")
	writeFile(t, landed.Dir, "a.txt", "shipped\n")
	writeFile(t, partial.Dir, "a.txt", "shipped\n")
	writeFile(t, partial.Dir, "b.txt", "still only here\n")
	writeFile(t, open.Dir, "c.txt", "not shipped\n")
	for _, w := range []wtcheckpoint.Worktree{landed, partial, open} {
		save(t, w, wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T060000Z")})
	}
	landing := h.addWorktree("dev/landing", "feat/landing")
	writeFile(t, landing.Dir, "a.txt", "shipped\n")
	writeFile(t, landing.Dir, "unrelated.txt", "someone else's work\n")
	h.git(landing.Dir, "add", "-A")
	h.git(landing.Dir, "commit", "-q", "-m", "land")
	h.git(landing.Dir, "update-ref", "refs/remotes/origin/main", "HEAD")

	keepOnly, err := wtcheckpoint.Prune(context.Background(), h.hub(), wtcheckpoint.PruneOptions{Keep: 20})
	if err != nil || len(keepOnly) != 0 {
		t.Fatalf("Prune without --landed = %+v, %v; want nothing deleted", keepOnly, err)
	}
	pruned, err := wtcheckpoint.Prune(context.Background(), h.hub(), wtcheckpoint.PruneOptions{Keep: 20, Landed: true})
	if err != nil {
		t.Fatal(err)
	}

	want := wtcheckpoint.Pruned{Ref: "refs/checkpoints/landed/20261006T060000Z", Reason: wtcheckpoint.PruneLanded}
	if len(pruned) != 1 || pruned[0] != want {
		t.Errorf("Prune --landed = %+v, want only %+v", pruned, want)
	}
	if got := refsIn(t, h); got != "refs/checkpoints/open/20261006T060000Z,refs/checkpoints/partial/20261006T060000Z" {
		t.Errorf("refs left = %s, want the partial and open checkpoints kept", got)
	}
}

func TestPrune_LandedWithoutOriginMainFailsAndDeletesNothing(t *testing.T) {
	t.Parallel()
	h := newHub(t)
	w := h.addWorktree("dev/task", "feat/task")
	writeFile(t, w.Dir, "a.txt", "work\n")
	save(t, w, wtcheckpoint.SaveOptions{Keep: 20, Now: clockAt("20261006T060000Z")})
	h.store.Git("update-ref", "-d", "refs/remotes/origin/main")

	if _, err := wtcheckpoint.Prune(context.Background(), h.hub(), wtcheckpoint.PruneOptions{Keep: 20, Landed: true}); err == nil {
		t.Error("Prune --landed with no origin/main succeeded; it cannot prove anything landed")
	}
	if got := refsIn(t, h); got != "refs/checkpoints/task/20261006T060000Z" {
		t.Errorf("refs left = %s, want the checkpoint kept", got)
	}
}
