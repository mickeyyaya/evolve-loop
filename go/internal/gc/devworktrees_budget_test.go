package gc

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

func blockingOn(verb string, endBudget context.CancelFunc) sysexec.RunFunc {
	return func(ctx context.Context, name, d string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 0 && args[0] == verb {
			endBudget()
			<-ctx.Done()
			return -1, ctx.Err()
		}
		return sysexec.DefaultRunner(ctx, name, d, args, env, in, out, errw)
	}
}

func TestPlanDevWorktrees_AGitCallThatOutlivesTheBudgetSkipsTheTreeInsteadOfFailingIt(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("slow")
	o := e.options(idleOffset)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	o.Exec = blockingOn("symbolic-ref", cancel)
	m, err := PlanDevWorktrees(ctx, o)
	if err != nil {
		t.Fatalf("PlanDevWorktrees: %v", err)
	}
	it := devItemFor(t, m, dir)
	if !it.Skipped || it.Failed || it.Action != DevActionKeep {
		t.Errorf("item = %+v, want skipped, not failed, kept", it)
	}
}

func TestPlanDevWorktrees_AProcessListThatOutlivesTheBudgetArchivesNothingAndFailsNothing(t *testing.T) {
	e := newDevEnv(t)
	e.addDev("slow")
	o := e.options(idleOffset)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	o.Processes = func(ctx context.Context) ([]Process, error) {
		cancel()
		<-ctx.Done()
		return nil, ctx.Err()
	}
	m, err := PlanDevWorktrees(ctx, o)
	if err != nil || !m.OverBudget || m.Skipped == "" || len(m.Items) != 0 {
		t.Errorf("manifest = %+v err = %v, want over budget, a reason, no items, no error", m, err)
	}
}

func TestPlanDevWorktrees_AHubListCutByTheBudgetIsOverBudgetNotAnError(t *testing.T) {
	e := newDevEnv(t)
	e.addDev("slow")
	o := e.options(idleOffset)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	o.Processes = func(context.Context) ([]Process, error) {
		cancel()
		return nil, nil
	}
	m, err := PlanDevWorktrees(ctx, o)
	if err != nil || !m.OverBudget {
		t.Errorf("manifest = %+v err = %v, want over budget and no error", m, err)
	}
}

func TestPlanDevWorktrees_ALandedCheckThatOutlivesTheBudgetSkipsTheTree(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("late")
	e.commitDatedIn(dir, "late.txt", "x\n", e.real.Add(-40*24*time.Hour))
	o := e.options(idleOffset)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	o.Exec = blockingOn("merge-base", cancel)
	it := devItemFor(t, mustPlan(t, ctx, o), dir)
	if !it.Skipped || it.Failed || it.Action != DevActionKeep {
		t.Errorf("item = %+v, want skipped, not failed, kept", it)
	}
}

func mustPlan(t *testing.T, ctx context.Context, o DevOptions) DevManifest {
	t.Helper()
	m, err := PlanDevWorktrees(ctx, o)
	if err != nil {
		t.Fatalf("PlanDevWorktrees: %v", err)
	}
	return m
}

func TestPlanDevWorktrees_AGitFailureWithTheBudgetAliveStaysNotJudged(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("broken")
	o := e.options(idleOffset)
	o.Exec = func(ctx context.Context, name, d string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 0 && args[0] == "symbolic-ref" {
			return -1, errors.New("git: cannot exec")
		}
		return sysexec.DefaultRunner(ctx, name, d, args, env, in, out, errw)
	}
	it := devItemFor(t, mustPlan(t, context.Background(), o), dir)
	if !it.Failed || it.Skipped {
		t.Errorf("item = %+v, want failed and not skipped", it)
	}
}

func TestApplyDevWorktrees_ABudgetThatEndsMidApplyDefersTheTreeWithoutAnError(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("late")
	o := e.options(idleOffset)
	m := e.plan(o)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	o.Exec = func(c context.Context, name, d string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 0 && args[0] == "symbolic-ref" {
			cancel()
			return -1, c.Err()
		}
		return sysexec.DefaultRunner(c, name, d, args, env, in, out, errw)
	}
	var deferred []string
	o.Deferred = func(p string) { deferred = append(deferred, p) }
	recs, err := ApplyDevWorktrees(ctx, o, m)
	if err != nil || len(recs) != 0 || len(deferred) != 1 {
		t.Fatalf("recs=%+v err=%v deferred=%v, want no record, no error, one deferral", recs, err, deferred)
	}
	if _, serr := os.Stat(dir); serr != nil {
		t.Errorf("the deferred tree is gone: %v", serr)
	}
}

func TestApplyDevWorktrees_AReJudgeFailureWithTheBudgetAliveStillErrors(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("broken")
	o := e.options(idleOffset)
	m := e.plan(o)
	o.Exec = func(c context.Context, name, d string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 0 && args[0] == "symbolic-ref" {
			return -1, errors.New("git: cannot exec")
		}
		return sysexec.DefaultRunner(c, name, d, args, env, in, out, errw)
	}
	deferred := 0
	o.Deferred = func(string) { deferred++ }
	_, err := ApplyDevWorktrees(context.Background(), o, m)
	if err == nil || deferred != 0 {
		t.Errorf("err=%v deferred=%d, want a refusal and no deferral", err, deferred)
	}
	if _, serr := os.Stat(dir); serr != nil {
		t.Errorf("tree gone: %v", serr)
	}
}

func savingThenCancelling(o DevOptions, cancel context.CancelFunc) DevSaveFunc {
	return func(c context.Context, wt wtcheckpoint.Worktree) (wtcheckpoint.SaveResult, error) {
		res, err := wtcheckpoint.Save(c, wt, wtcheckpoint.SaveOptions{Label: devArchiveLabel, Now: o.Now})
		cancel()
		return res, err
	}
}

type archiveState struct {
	exists      bool
	checkpoints int
	recorded    bool
}

func (e *devEnv) archiveStateOf(dir string) archiveState {
	e.t.Helper()
	_, statErr := os.Stat(dir)
	entries, err := wtcheckpoint.List(context.Background(), e.hub(), filepath.Base(dir))
	if err != nil {
		e.t.Fatalf("list checkpoints of %s: %v", dir, err)
	}
	stored, err := ReadDevArchive(e.evolve)
	if err != nil {
		e.t.Fatalf("read the archive manifest: %v", err)
	}
	want := filepath.Join(resolvePath(filepath.Dir(dir)), filepath.Base(dir))
	recorded := false
	for _, rec := range stored {
		recorded = recorded || resolvePath(rec.Path) == want
	}
	return archiveState{exists: statErr == nil, checkpoints: len(entries), recorded: recorded}
}

func (s archiveState) fullyArchived() bool { return !s.exists && s.checkpoints == 1 && s.recorded }

func (s archiveState) untouched() bool { return s.exists && s.checkpoints == 0 && !s.recorded }

func TestApplyDevWorktrees_ArchiveRunsUnderDedicatedContext(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("atomic")
	writeRepoFile(t, dir, "base.txt", "dirty edit\n")
	o := e.options(idleOffset)
	m := e.plan(o)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	o.Save = savingThenCancelling(o, cancel)
	recs, err := ApplyDevWorktrees(ctx, o, m)
	if err != nil || len(recs) != 1 || recs[0].Checkpoint == "" {
		t.Fatalf("recs=%+v err=%v, want one record with its checkpoint and no error", recs, err)
	}
	if st := e.archiveStateOf(dir); !st.fullyArchived() {
		t.Errorf("state = %+v, want the started archive finished: tree removed, one checkpoint, one manifest row", st)
	}
}

func TestApplyDevWorktrees_ABudgetThatEndsAsTheRemoveStartsStillFinishesTheArchive(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("removing")
	writeRepoFile(t, dir, "base.txt", "dirty edit\n")
	o := e.options(idleOffset)
	m := e.plan(o)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	o.Exec = func(c context.Context, name, d string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 1 && args[0] == "worktree" && args[1] == "remove" {
			cancel()
		}
		return sysexec.DefaultRunner(c, name, d, args, env, in, out, errw)
	}
	recs, err := ApplyDevWorktrees(ctx, o, m)
	if err != nil || len(recs) != 1 {
		t.Fatalf("recs=%+v err=%v, want one record and no error", recs, err)
	}
	if st := e.archiveStateOf(dir); !st.fullyArchived() {
		t.Errorf("state = %+v, want the remove finished and recorded despite the spent budget", st)
	}
}

func TestApplyDevWorktrees_BudgetCutLeavesTreeUntouchedOrFullyArchived(t *testing.T) {
	e := newDevEnv(t)
	dirs := []string{e.addDev("tree-one"), e.addDev("tree-two")}
	for _, dir := range dirs {
		writeRepoFile(t, dir, "base.txt", "dirty "+filepath.Base(dir)+"\n")
	}
	o := e.options(idleOffset)
	m := e.plan(o)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	o.Save = savingThenCancelling(o, cancel)
	var deferred []string
	o.Deferred = func(p string) { deferred = append(deferred, p) }
	recs, err := ApplyDevWorktrees(ctx, o, m)
	if err != nil {
		t.Errorf("ApplyDevWorktrees: %v, want no error when the budget ends mid-sweep", err)
	}
	archived, untouched := 0, 0
	for _, dir := range dirs {
		st := e.archiveStateOf(dir)
		switch {
		case st.fullyArchived():
			archived++
		case st.untouched():
			untouched++
		default:
			t.Errorf("tree %s is half-archived: %+v", dir, st)
		}
	}
	if archived != 1 || untouched != 1 || len(recs) != 1 || len(deferred) != 1 {
		t.Errorf("archived=%d untouched=%d recs=%+v deferred=%v, want the started tree archived and the next one deferred untouched", archived, untouched, recs, deferred)
	}
}
