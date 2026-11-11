package gc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

const freeOfUniqueWork = "free of unique work"

func (e *devEnv) addDetached(name string) string {
	e.t.Helper()
	dir := filepath.Join(e.root, "dev", name)
	e.store.Git("worktree", "add", "-q", "--detach", dir, "main")
	return dir
}

func (e *devEnv) addDetachedWithUniqueWork(name string) (dir, head string) {
	e.t.Helper()
	dir = e.addDetached(name)
	e.commitIn(dir, name+".txt", name+"\n")
	return dir, e.store.Git("-C", dir, "rev-parse", "HEAD")
}

func (e *devEnv) landHead(head string) {
	e.t.Helper()
	e.store.Git("update-ref", "refs/remotes/origin/main", head)
}

func pullRequestHeads(oids ...string) func(context.Context, string) (map[string]bool, error) {
	set := map[string]bool{}
	for _, oid := range oids {
		set[oid] = true
	}
	return func(context.Context, string) (map[string]bool, error) { return set, nil }
}

func (e *devEnv) reachableFromCheckpoints(head string) string {
	e.t.Helper()
	return e.store.Git("for-each-ref", "--format=%(refname)", "--contains", head, "refs/checkpoints/")
}

func requireArchive(t *testing.T, it DevItem, reasonPart string) {
	t.Helper()
	if it.Action != DevActionArchive || it.Failed || it.Skipped || !strings.Contains(it.Reason, reasonPart) {
		t.Fatalf("item = %+v, want an archive that says %q", it, reasonPart)
	}
}

func requireKeep(t *testing.T, it DevItem, reasonPart string) {
	t.Helper()
	if it.Action != DevActionKeep || it.Failed || !strings.Contains(it.Reason, reasonPart) {
		t.Fatalf("item = %+v, want a keep that says %q", it, reasonPart)
	}
}

func TestPlanDevWorktrees_DetachedTreeOnAMergedHeadIsReleased(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDetached("merged")
	it := devItemFor(t, e.plan(e.options(idleOffset)), dir)
	requireArchive(t, it, freeOfUniqueWork)
	if strings.Contains(it.Reason, "no branch keeps") {
		t.Fatalf("reason = %q, the old keep wording must be gone", it.Reason)
	}
}

func TestPlanDevWorktrees_DetachedMergedAndCleanTreeSkipsTheActivityRule(t *testing.T) {
	e := newDevEnv(t)
	dir, head := e.addDetachedWithUniqueWork("fresh-merged")
	e.landHead(head)
	it := devItemFor(t, e.plan(e.options(0)), dir)
	requireArchive(t, it, "")
}

func TestPlanDevWorktrees_DetachedMergedButDirtyTreeKeepsItsActivityRule(t *testing.T) {
	e := newDevEnv(t)
	dir, head := e.addDetachedWithUniqueWork("dirty-merged")
	e.landHead(head)
	writeRepoFile(t, dir, "wip.txt", "unsaved\n")
	requireKeep(t, devItemFor(t, e.plan(e.options(0)), dir), "active within")
}

func TestPlanDevWorktrees_DetachedMergedDirtyIdleTreeIsSavedAsACheckpointFirst(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDetached("dirty-idle")
	writeRepoFile(t, dir, "wip.txt", "unsaved\n")
	it := devItemFor(t, e.plan(e.options(idleOffset)), dir)
	requireArchive(t, it, "uncommitted work is saved as a checkpoint first")
	if !it.Dirty {
		t.Fatalf("item = %+v, want Dirty", it)
	}
}

func TestPlanDevWorktrees_DetachedTreeWithAUniqueHeadIsArchivedByTheIdleRule(t *testing.T) {
	e := newDevEnv(t)
	dir, _ := e.addDetachedWithUniqueWork("unique-idle")
	it := devItemFor(t, e.plan(e.options(idleOffset)), dir)
	requireArchive(t, it, "unique")
	if strings.Contains(it.Reason, freeOfUniqueWork) {
		t.Fatalf("reason = %q, a unique head is not free of unique work", it.Reason)
	}
}

func TestPlanDevWorktrees_DetachedTreeWithAUniqueHeadIsKeptWhileItIsActive(t *testing.T) {
	e := newDevEnv(t)
	dir, _ := e.addDetachedWithUniqueWork("unique-fresh")
	requireKeep(t, devItemFor(t, e.plan(e.options(0)), dir), "active within")
}

func TestPlanDevWorktrees_DetachedTreeOnAPullRequestHeadIsReleased(t *testing.T) {
	e := newDevEnv(t)
	dir, head := e.addDetachedWithUniqueWork("pr-head")
	o := e.options(idleOffset)
	o.PRHeads = pullRequestHeads("0000000000000000000000000000000000000000", head)
	requireArchive(t, devItemFor(t, e.plan(o), dir), freeOfUniqueWork)
}

func TestPlanDevWorktrees_DetachedTreeOnAPullRequestHeadKeepsTheActivityRule(t *testing.T) {
	e := newDevEnv(t)
	dir, head := e.addDetachedWithUniqueWork("pr-head-fresh")
	o := e.options(0)
	o.PRHeads = pullRequestHeads(head)
	requireKeep(t, devItemFor(t, e.plan(o), dir), "active within")
}

func TestPlanDevWorktrees_DetachedTreeWhoseHeadIsNotAPullRequestHeadStaysUnique(t *testing.T) {
	e := newDevEnv(t)
	dir, _ := e.addDetachedWithUniqueWork("other-pr")
	o := e.options(idleOffset)
	o.PRHeads = pullRequestHeads("1111111111111111111111111111111111111111")
	it := devItemFor(t, e.plan(o), dir)
	requireArchive(t, it, "unique")
	if strings.Contains(it.Reason, freeOfUniqueWork) {
		t.Fatalf("reason = %q, another pull request head does not free this tree", it.Reason)
	}
}

func TestPlanDevWorktrees_DetachedTreeIsUniqueWhenThePullRequestLookupIsMissingOrUnavailable(t *testing.T) {
	lookups := map[string]func(context.Context, string) (map[string]bool, error){
		"no lookup wired":    nil,
		"gh unavailable":     func(context.Context, string) (map[string]bool, error) { return nil, ErrPRLookupUnavailable },
		"lookup failed hard": func(context.Context, string) (map[string]bool, error) { return nil, errors.New("gh: rate limit") },
	}
	for name, lookup := range lookups {
		e := newDevEnv(t)
		dir, _ := e.addDetachedWithUniqueWork("nolookup")
		o := e.options(idleOffset)
		o.PRHeads = lookup
		it := devItemFor(t, e.plan(o), dir)
		if it.Action != DevActionArchive || it.Failed || !strings.Contains(it.Reason, "unique") {
			t.Fatalf("%s: item = %+v, want an archive of a unique head", name, it)
		}
	}
}

func TestPlanDevWorktrees_ThePullRequestHeadsAreReadOncePerSweep(t *testing.T) {
	e := newDevEnv(t)
	first, _ := e.addDetachedWithUniqueWork("once-a")
	second, _ := e.addDetachedWithUniqueWork("once-b")
	o := e.options(idleOffset)
	calls := 0
	o.PRHeads = func(context.Context, string) (map[string]bool, error) {
		calls++
		return nil, nil
	}
	m := e.plan(o)
	requireArchive(t, devItemFor(t, m, first), "unique")
	requireArchive(t, devItemFor(t, m, second), "unique")
	if calls != 1 {
		t.Fatalf("PRHeads calls = %d, want exactly 1 for two unique detached trees", calls)
	}
}

func TestPlanDevWorktrees_ABranchTreeNeverAsksForThePullRequestHeads(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("branch-only")
	e.commitIn(dir, "work.txt", "x\n")
	o := e.options(idleOffset)
	o.PRHeads = func(context.Context, string) (map[string]bool, error) {
		t.Error("PRHeads called for a tree that sits on a branch")
		return nil, nil
	}
	requireArchive(t, devItemFor(t, e.plan(o), dir), "")
}

func TestApplyDevWorktrees_DetachedMergedHeadIsRemovedWithoutNeedingASave(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDetached("apply-merged")
	o := e.options(idleOffset)
	o.Save = failingDevSave
	recs, err := e.apply(o, e.plan(o))
	if err != nil || len(recs) != 1 || recs[0].Checkpoint != "" {
		t.Fatalf("apply = %+v, %v; want one record with no checkpoint and no error", recs, err)
	}
	if _, serr := os.Stat(dir); !os.IsNotExist(serr) {
		t.Fatalf("tree %s still exists (stat err %v)", dir, serr)
	}
}

func TestApplyDevWorktrees_DetachedPullRequestHeadIsRemovedWithoutNeedingASave(t *testing.T) {
	e := newDevEnv(t)
	dir, head := e.addDetachedWithUniqueWork("apply-pr")
	o := e.options(idleOffset)
	o.PRHeads = pullRequestHeads(head)
	o.Save = failingDevSave
	recs, err := e.apply(o, e.plan(o))
	if err != nil || len(recs) != 1 {
		t.Fatalf("apply = %+v, %v; want one record and no error", recs, err)
	}
	if _, serr := os.Stat(dir); !os.IsNotExist(serr) {
		t.Fatalf("tree %s still exists (stat err %v)", dir, serr)
	}
}

func TestApplyDevWorktrees_DetachedUniqueHeadIsSavedBeforeTheTreeIsRemoved(t *testing.T) {
	e := newDevEnv(t)
	dir, head := e.addDetachedWithUniqueWork("apply-unique")
	if got := e.reachableFromCheckpoints(head); got != "" {
		t.Fatalf("precondition: head %s already sits under %q", head, got)
	}
	o := e.options(idleOffset)
	saves := 0
	o.Save = func(ctx context.Context, w wtcheckpoint.Worktree) (wtcheckpoint.SaveResult, error) {
		saves++
		if _, err := os.Stat(w.Dir); err != nil {
			t.Errorf("save ran after the tree was gone: %v", err)
		}
		return wtcheckpoint.Save(ctx, w, wtcheckpoint.SaveOptions{Label: devArchiveLabel, Now: o.Now})
	}
	recs, err := e.apply(o, e.plan(o))
	if err != nil || len(recs) != 1 {
		t.Fatalf("apply = %+v, %v; want one record and no error", recs, err)
	}
	if saves == 0 {
		t.Fatal("the unique head was removed without a save")
	}
	if _, serr := os.Stat(dir); !os.IsNotExist(serr) {
		t.Fatalf("tree %s still exists (stat err %v)", dir, serr)
	}
	if recs[0].Checkpoint == "" || !strings.Contains(e.reachableFromCheckpoints(head), recs[0].Checkpoint) {
		t.Fatalf("record checkpoint %q does not hold head %s (refs holding it: %q)", recs[0].Checkpoint, head, e.reachableFromCheckpoints(head))
	}
}

func TestApplyDevWorktrees_DetachedUniqueHeadStaysWhenTheSaveFails(t *testing.T) {
	e := newDevEnv(t)
	dir, head := e.addDetachedWithUniqueWork("apply-save-fails")
	o := e.options(idleOffset)
	m := e.plan(o)
	o.Save = failingDevSave
	recs, err := e.apply(o, m)
	if err == nil || len(recs) != 0 {
		t.Fatalf("apply = %+v, %v; want a refusal and no record", recs, err)
	}
	if _, serr := os.Stat(filepath.Join(dir, "apply-save-fails.txt")); serr != nil {
		t.Fatalf("the tree lost its committed work: %v", serr)
	}
	if got := e.store.Git("-C", dir, "rev-parse", "HEAD"); got != head {
		t.Fatalf("HEAD = %s, want %s", got, head)
	}
}

func TestApplyDevWorktrees_DetachedTreeIsRefusedOnApplyWhenItsHeadMovedSincePlan(t *testing.T) {
	e := newDevEnv(t)
	dir, _ := e.addDetachedWithUniqueWork("apply-moved")
	o := e.options(idleOffset)
	m := e.plan(o)
	e.commitIn(dir, "later.txt", "later\n")
	recs, err := e.apply(o, m)
	if err == nil || len(recs) != 0 || !strings.Contains(err.Error(), "HEAD moved") {
		t.Fatalf("apply = %+v, %v; want a refusal because HEAD moved since the plan", recs, err)
	}
	if _, serr := os.Stat(dir); serr != nil {
		t.Fatalf("the tree was removed: %v", serr)
	}
}

func TestApplyDevWorktrees_DetachedTreeIsJudgedAgainOnApplyAndKeptWhenActive(t *testing.T) {
	e := newDevEnv(t)
	dir, _ := e.addDetachedWithUniqueWork("apply-young")
	o := e.options(idleOffset)
	m := e.plan(o)
	o.Now = func() time.Time { return e.real }
	recs, err := e.apply(o, m)
	if err == nil || len(recs) != 0 {
		t.Fatalf("apply = %+v, %v; want a refusal because the tree is active again", recs, err)
	}
	if _, serr := os.Stat(dir); serr != nil {
		t.Fatalf("the tree was removed: %v", serr)
	}
}
