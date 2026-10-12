package gc

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func blockingOn(verb string) sysexec.RunFunc {
	return func(ctx context.Context, name, d string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 0 && args[0] == verb {
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
	o.Exec = blockingOn("symbolic-ref")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
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
	o.Processes = func(ctx context.Context) ([]Process, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
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
	o.Exec = blockingOn("merge-base")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
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
