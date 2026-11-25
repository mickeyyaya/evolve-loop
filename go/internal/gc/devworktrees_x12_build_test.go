package gc

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func TestPlanDevWorktrees_X12_ClosedPRAndOpenPRAndInvalidGlobCoverage(t *testing.T) {
	e := newDevEnv(t)
	closed := e.addDev("closed-clean")
	e.commitIn(closed, "c.txt", "c\n")
	closedTip := e.store.Git("-C", closed, "rev-parse", "HEAD")

	reopened := e.addDev("closed-reopened")
	e.commitIn(reopened, "r.txt", "r\n")
	reopenedTip := e.store.Git("-C", reopened, "rev-parse", "HEAD")

	o := e.options(0)
	now := o.Now()
	o.ClosedPRs = func(context.Context, string) (map[string][]string, error) {
		return map[string][]string{
			"br-closed-clean":    {closedTip},
			"br-closed-reopened": {reopenedTip},
		}, nil
	}
	o.OpenPRs = func(context.Context, string) (map[string][]time.Time, error) {
		return map[string][]time.Time{
			"br-closed-reopened": {
				now.Add(-5 * 24 * time.Hour),
				now.Add(-2 * 24 * time.Hour),
				now.Add(-7 * 24 * time.Hour),
			},
		}, nil
	}

	m := e.plan(o)
	if it := devItemFor(t, m, closed); it.Action != DevActionArchive || !strings.Contains(it.Reason, "has a closed pull request") {
		t.Errorf("closed tree = %+v, want archived with closed pull request reason", it)
	}
	if it := devItemFor(t, m, reopened); it.Action != DevActionKeep || !strings.Contains(it.Reason, "open pull request updated 2.0 days ago") {
		t.Errorf("reopened tree = %+v, want kept with newest open PR age", it)
	}
}

func TestPlanDevWorktrees_X12_ProtectedGlobErrorAndOutsideDevDir(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("badglob")
	o := e.options(0)
	o.Policy = DevWorktreesPolicy{Protected: []string{"["}}

	it := devItemFor(t, e.plan(o), dir)
	if it.Action != DevActionKeep || !it.Failed || !strings.Contains(it.Reason, "gc.dev_worktrees.protected") {
		t.Errorf("invalid glob = %+v, want kept and marked failed", it)
	}

	s, skipped, err := newDevSweep(context.Background(), o)
	if err != nil || skipped != "" {
		t.Fatalf("newDevSweep = %q, %v", skipped, err)
	}
	if glob, err := s.protectedGlob(e.root); err != nil || glob != "" {
		t.Errorf("protectedGlob(root) = %q, %v; want empty, nil", glob, err)
	}
}

func TestPlanDevWorktrees_X12_ActiveTreeDirtyErrorAndBudgetCancel(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("active-fail")
	o := e.options(0)
	o.Exec = failOn("status", -1, errors.New("git status vanished"))
	it := devItemFor(t, e.plan(o), dir)
	if it.Action != DevActionKeep || !strings.Contains(it.Reason, "landed check:") {
		t.Errorf("active dirty failure = %+v, want landed check error in reason", it)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	o2 := e.options(0)
	o2.Exec = func(c context.Context, name, d string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 0 && args[0] == "status" {
			cancel()
			<-c.Done()
			return -1, c.Err()
		}
		return sysexec.DefaultRunner(c, name, d, args, env, in, out, errw)
	}
	m, err := PlanDevWorktrees(ctx, o2)
	if err != nil {
		t.Fatalf("PlanDevWorktrees = %v", err)
	}
	if got := devItemFor(t, m, dir); !got.Skipped || got.Failed {
		t.Errorf("active budget cancel = %+v, want skipped and not failed", got)
	}
}
