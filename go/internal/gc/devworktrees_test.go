package gc

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func TestPlanDevWorktrees_ReportsEveryTreeWithItsReasonAndDaysIdle(t *testing.T) {
	e := newDevEnv(t)
	idle := e.addDev("idle-clean")
	fresh := e.addDev("fresh")
	m := e.plan(e.options(idleOffset))

	if got := archiveActions(m); !reflect.DeepEqual(got, []DevAction{DevActionArchive, DevActionArchive}) {
		t.Fatalf("actions = %v, want both trees archived once 20 days have passed", got)
	}
	it := devItemFor(t, m, idle)
	if it.Action != DevActionArchive || it.Branch != "br-idle-clean" || it.IdleDays < 19.9 || it.IdleDays > 20.1 {
		t.Fatalf("idle item = %+v, want archive of br-idle-clean after about 20 idle days", it)
	}
	if !strings.Contains(it.Reason, "idle 20.0 days (limit 14)") || !strings.Contains(it.Reason, "branch br-idle-clean is in main and stays") {
		t.Errorf("reason = %q, want the idle days, the limit and the branch fate", it.Reason)
	}
	active := devItemFor(t, e.plan(e.options(0)), fresh)
	if active.Action != DevActionKeep || active.Reason != "active within the 14-day limit" || active.IdleDays > 0.1 {
		t.Errorf("fresh item = %+v, want kept as active within the 14-day limit", active)
	}
	if len(m.Items) != 2 || m.Items[0].Path > m.Items[1].Path {
		t.Errorf("items = %+v, want the two dev trees sorted by path and no plane tree", m.Items)
	}
}

func TestPlanDevWorktrees_AnActiveReflogKeepsATreeWhateverItsIndexTimeIs(t *testing.T) {
	e := newDevEnv(t)
	active := e.addDev("active-old-index")
	idle := e.addDev("idle-new-index")
	e.touchIndex(active, e.real.Add(-400*24*time.Hour))
	e.touchIndex(idle, e.real.Add(idleOffset))

	atNow := e.plan(e.options(0))
	if it := devItemFor(t, atNow, active); it.Action != DevActionKeep {
		t.Errorf("a tree with a recent reflog but a 400-day-old index: %+v, want kept", it)
	}
	later := e.plan(e.options(idleOffset))
	if it := devItemFor(t, later, idle); it.Action != DevActionArchive {
		t.Errorf("an idle tree whose index was just rewritten: %+v, want archived", it)
	}
}

func TestPlanDevWorktrees_TheNewestOfCommitAndReflogDecides(t *testing.T) {
	e := newDevEnv(t)
	oldCommit := e.addDev("old-commit-fresh-reflog")
	e.commitDatedIn(oldCommit, "a.txt", "a\n", e.real.Add(-100*24*time.Hour))
	futureCommit := e.addDev("fresh-commit-old-reflog")
	e.commitDatedIn(futureCommit, "b.txt", "b\n", e.real.Add(5*24*time.Hour))

	m := e.plan(e.options(16 * 24 * time.Hour))
	if it := devItemFor(t, m, oldCommit); it.Action != DevActionArchive {
		t.Errorf("a 100-day-old commit with a reflog entry 16 days old: %+v, want archived at day 16", it)
	}
	if it := devItemFor(t, m, futureCommit); it.Action != DevActionKeep {
		t.Errorf("a commit dated 11 days before now with a reflog entry 16 days old: %+v, want kept by the commit time", it)
	}
	if it := devItemFor(t, e.plan(e.options(0)), oldCommit); it.Action != DevActionKeep {
		t.Errorf("a 100-day-old commit with a reflog entry from now: %+v, want kept by the reflog time", it)
	}
}

func TestPlanDevWorktrees_ThePolicyHorizonSetsTheIdleLimit(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("horizon")
	o := e.options(10 * 24 * time.Hour)
	if it := devItemFor(t, e.plan(o), dir); it.Action != DevActionKeep {
		t.Fatalf("10 days idle under the default 14-day limit: %+v, want kept", it)
	}
	o.Policy = DevWorktreesPolicy{ArchiveAfterDays: 7}
	if it := devItemFor(t, e.plan(o), dir); it.Action != DevActionArchive || !strings.Contains(it.Reason, "(limit 7)") {
		t.Fatalf("10 days idle under a 7-day limit: %+v, want archived with the limit named", it)
	}
}

func TestPlanDevWorktrees_ADirtyTreeIsMarkedForACheckpointFirst(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("dirty")
	writeRepoFile(t, dir, "base.txt", "edited\n")
	writeRepoFile(t, dir, "new.txt", "untracked\n")
	it := devItemFor(t, e.plan(e.options(idleOffset)), dir)
	if it.Action != DevActionArchive || !it.Dirty || !strings.Contains(it.Reason, "uncommitted work is saved as a checkpoint first") {
		t.Fatalf("item = %+v, want archive with the dirty mark and the checkpoint step in the reason", it)
	}
}

func TestPlanDevWorktrees_SquashMergedAndUnlandedBranchesAreTold(t *testing.T) {
	e := newDevEnv(t)
	landed := e.addDev("landed")
	e.commitIn(landed, "landed.txt", "landed\n")
	e.store.Git("-C", landed, "push", "-q", e.store.Dir, "HEAD:refs/heads/tmp-landed")
	e.store.Git("worktree", "add", "-q", "--detach", filepath.Join(e.root, "scratch"), "main")
	scratch := filepath.Join(e.root, "scratch")
	e.store.Git("-C", scratch, "merge", "-q", "--squash", "br-landed")
	e.store.Git("-C", scratch, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "squash landed")
	e.store.Git("update-ref", "refs/remotes/origin/main", e.store.Git("-C", scratch, "rev-parse", "HEAD"))
	open := e.addDev("open")
	e.commitIn(open, "open.txt", "open\n")

	m := e.plan(e.options(idleOffset))
	if it := devItemFor(t, m, landed); !strings.Contains(it.Reason, "is in main") {
		t.Errorf("squash-merged branch reason = %q, want it to say the branch is in main", it.Reason)
	}
	if it := devItemFor(t, m, open); !strings.Contains(it.Reason, "is not in main") || it.Action != DevActionArchive {
		t.Errorf("unlanded branch item = %+v, want archive, with the reason saying the branch is not in main", it)
	}
}

func TestPlanDevWorktrees_ProtectsTheCallerTheProjectRootAndLiveProcesses(t *testing.T) {
	e := newDevEnv(t)
	caller := e.addDev("caller-cwd")
	busy := e.addDev("busy")
	root := e.addDev("project-root")

	o := e.options(idleOffset)
	o.Cwd = filepath.Join(caller, "sub", "dir")
	o.Processes = func(context.Context) ([]Process, error) {
		return []Process{{Pid: 4242, Ppid: 1, Cwd: filepath.Join(busy, "go")}}, nil
	}
	m := e.plan(o)
	if it := devItemFor(t, m, caller); it.Action != DevActionKeep || it.Reason != "the caller works in this directory" {
		t.Errorf("caller tree = %+v, want kept for the caller", it)
	}
	if it := devItemFor(t, m, busy); it.Action != DevActionKeep || !strings.Contains(it.Reason, "process 4242 runs inside") {
		t.Errorf("busy tree = %+v, want kept for the process", it)
	}
	if it := devItemFor(t, m, root); it.Action != DevActionArchive {
		t.Errorf("other tree = %+v, want archived", it)
	}

	o = e.options(idleOffset)
	o.ProjectRoot = root
	if it := devItemFor(t, e.plan(o), root); it.Action != DevActionKeep || it.Reason != "the project root of this run" {
		t.Errorf("project root tree = %+v, want kept as the project root", it)
	}
}

func TestIsPlaneWorktree_NamesTheRuntimeAndConsolePlanesOnly(t *testing.T) {
	hub := t.TempDir()
	for name, want := range map[string]bool{"runtime": true, "console": true, "dev/runtime": false, "dev/x": false} {
		if got := isPlaneWorktree(hub, filepath.Join(hub, name)); got != want {
			t.Errorf("isPlaneWorktree(%s) = %v, want %v", name, got, want)
		}
	}
}

func TestPlanDevWorktrees_ARealProcessInsideATreeKeepsItAndEndsWhenTheProcessEnds(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("sleeper")
	o := e.options(idleOffset)
	o.Processes = nil

	cmd := exec.Command("sleep", "60")
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	reaped := false
	reap := func() {
		if !reaped {
			reaped = true
			cmd.Process.Kill()
			cmd.Wait()
		}
	}
	defer reap()

	it := devItemFor(t, e.plan(o), dir)
	if it.Action != DevActionKeep || !strings.Contains(it.Reason, "runs inside") {
		t.Fatalf("with pid %d inside: %+v, want kept for a process inside", pid, it)
	}
	reap()
	if it := devItemFor(t, e.plan(o), dir); it.Action != DevActionArchive {
		t.Fatalf("after the process ended: %+v, want archived", it)
	}
}

func TestPlanDevWorktrees_ADetachedTreeIsKept(t *testing.T) {
	e := newDevEnv(t)
	dir := filepath.Join(e.root, "dev", "detached")
	e.store.Git("worktree", "add", "-q", "--detach", dir, "main")
	it := devItemFor(t, e.plan(e.options(idleOffset)), dir)
	if it.Action != DevActionKeep || !strings.Contains(it.Reason, "detached HEAD") {
		t.Fatalf("detached tree = %+v, want kept because no branch holds its commits", it)
	}
}

func TestPlanDevWorktrees_OpenPullRequestsKeepATreeOnlyWhileTheyAreFresh(t *testing.T) {
	e := newDevEnv(t)
	recent := e.addDev("pr-recent")
	stale := e.addDev("pr-stale")
	none := e.addDev("pr-none")
	o := e.options(idleOffset)
	now := o.Now()
	calls := 0
	o.OpenPRs = func(_ context.Context, dir string) (map[string][]time.Time, error) {
		calls++
		if dir != e.runtime {
			t.Errorf("lookup dir = %s, want the project root %s", dir, e.runtime)
		}
		return map[string][]time.Time{
			"br-pr-recent": {now.Add(-30 * 24 * time.Hour), now.Add(-3 * 24 * time.Hour)},
			"br-pr-stale":  {now.Add(-30 * 24 * time.Hour)},
		}, nil
	}
	m := e.plan(o)
	if it := devItemFor(t, m, recent); it.Action != DevActionKeep || !strings.Contains(it.Reason, "open pull request updated 3.0 days ago") {
		t.Errorf("recent PR tree = %+v, want kept with the PR age", it)
	}
	for _, dir := range []string{stale, none} {
		if it := devItemFor(t, m, dir); it.Action != DevActionArchive {
			t.Errorf("tree %s = %+v, want archived", dir, it)
		}
	}
	if calls != 1 {
		t.Errorf("pull request lookup ran %d times, want once for the whole sweep", calls)
	}
}

func TestPlanDevWorktrees_NoPullRequestLookupIsMadeWhenEveryTreeIsActive(t *testing.T) {
	e := newDevEnv(t)
	e.addDev("a")
	o := e.options(0)
	o.OpenPRs = func(context.Context, string) (map[string][]time.Time, error) {
		t.Error("the lookup ran although no tree was idle")
		return nil, nil
	}
	e.plan(o)
}

func TestPlanDevWorktrees_AMissingGhIsNotedAndAFailingGhFailsClosed(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("gh")
	o := e.options(idleOffset)
	o.OpenPRs = func(context.Context, string) (map[string][]time.Time, error) { return nil, ErrPRLookupUnavailable }
	m := e.plan(o)
	if it := devItemFor(t, m, dir); it.Action != DevActionArchive || len(m.Notes) != 1 || !strings.Contains(m.Notes[0], "gh is unavailable") {
		t.Errorf("without gh: item %+v notes %q, want archive and one note", it, m.Notes)
	}
	o.OpenPRs = nil
	m = e.plan(o)
	if len(m.Notes) != 1 || !strings.Contains(m.Notes[0], "no pull request lookup") {
		t.Errorf("without a lookup: notes %q, want one note", m.Notes)
	}
	o.OpenPRs = func(context.Context, string) (map[string][]time.Time, error) {
		return nil, errors.New("gh rate limited")
	}
	it := devItemFor(t, e.plan(o), dir)
	if it.Action != DevActionKeep || !it.Failed || !strings.Contains(it.Reason, "gh rate limited") {
		t.Errorf("failing gh: %+v, want kept, marked failed, with the cause", it)
	}
}

func TestPlanDevWorktrees_APlainRepositoryIsSkippedWithItsReason(t *testing.T) {
	plain := gittest.Fixture(t)
	commitRepoFile(t, plain, "a.txt", "a\n")
	m, err := PlanDevWorktrees(context.Background(), DevOptions{ProjectRoot: plain.Dir, Exec: sysexec.DefaultRunner})
	if err != nil || m.Skipped == "" || len(m.Items) != 0 {
		t.Fatalf("plan = %+v, %v; want a skip with a reason and no items", m, err)
	}
}

func TestPlanDevWorktrees_SetupFailuresSurface(t *testing.T) {
	e := newDevEnv(t)
	if _, err := PlanDevWorktrees(context.Background(), DevOptions{ProjectRoot: e.runtime}); err == nil {
		t.Error("no Exec: nil error, want a refusal")
	}
	o := e.options(0)
	o.Processes = func(context.Context) ([]Process, error) { return nil, errors.New("lsof vanished") }
	if _, err := PlanDevWorktrees(context.Background(), o); err == nil || !strings.Contains(err.Error(), "lsof vanished") {
		t.Errorf("process listing failure: %v, want it surfaced (fail closed)", err)
	}
	if err := os.WriteFile(filepath.Join(e.store.Dir, "HEAD"), []byte("garbage\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanDevWorktrees(context.Background(), e.options(0)); err == nil {
		t.Error("a corrupt store: nil error, want the worktree listing failure")
	}
}

func TestPlanDevWorktrees_AGitFailureMarksOneTreeFailedAndKeepsIt(t *testing.T) {
	boom := errors.New("git vanished")
	cases := []struct {
		name string
		exec func(args []string) bool
	}{
		{"rev-parse HEAD fails", func(a []string) bool { return a[0] == "rev-parse" && len(a) == 2 && a[1] == "HEAD" }},
		{"symbolic-ref errors", func(a []string) bool { return a[0] == "symbolic-ref" }},
		{"activity cannot be read", func(a []string) bool { return a[0] == "reflog" }},
		{"status fails", func(a []string) bool { return a[0] == "status" }},
		{"ancestry check fails", func(a []string) bool { return a[0] == "merge-base" && len(a) > 1 && a[1] == "--is-ancestor" }},
	}
	for _, c := range cases {
		e := newDevEnv(t)
		dir := e.addDev("victim")
		o := e.options(idleOffset)
		o.Exec = func(ctx context.Context, name, d string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
			if len(args) > 0 && c.exec(args) {
				return 0, boom
			}
			return sysexec.DefaultRunner(ctx, name, d, args, env, in, out, errw)
		}
		it := devItemFor(t, e.plan(o), dir)
		if it.Action != DevActionKeep || !it.Failed || !strings.Contains(it.Reason, "cannot judge") {
			t.Errorf("%s: %+v, want kept and marked failed", c.name, it)
		}
	}
}

func TestPlanDevWorktrees_ASymbolicRefFailureCodeIsNotTakenForDetached(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("sr")
	o := e.options(idleOffset)
	o.Exec = failOn("symbolic-ref", 128, nil)
	it := devItemFor(t, e.plan(o), dir)
	if !it.Failed || it.Action != DevActionKeep {
		t.Fatalf("symbolic-ref exit 128: %+v, want a failure, not a detached-HEAD keep", it)
	}
}

func TestGateProtected_RefusesAPlaneWorktreeEvenThoughTheListingNeverOffersOne(t *testing.T) {
	e := newDevEnv(t)
	e.store.Git("worktree", "add", "-q", "--detach", filepath.Join(e.root, "console"), "main")
	o := e.options(0)
	o.ProjectRoot = e.addDev("anchor")
	s, skipped, err := newDevSweep(context.Background(), o)
	if err != nil || skipped != "" {
		t.Fatalf("newDevSweep = %q, %v", skipped, err)
	}
	for _, plane := range []string{"runtime", "console"} {
		reason, err := s.gateProtected(context.Background(), &DevItem{Path: filepath.Join(e.root, plane)})
		if err != nil || !strings.Contains(reason, "plane worktree") {
			t.Errorf("%s: reason %q err %v, want the plane refusal", plane, reason, err)
		}
	}
	if reason, _ := s.gateProtected(context.Background(), &DevItem{Path: filepath.Join(e.root, "dev", "x")}); reason != "" {
		t.Errorf("a dev path: reason %q, want none", reason)
	}
}

func TestDevOptions_DefaultClockAndSaveAreTheRealOnes(t *testing.T) {
	if d := time.Since(DevOptions{}.now()); d < 0 || d > time.Minute {
		t.Errorf("default now() is %s away from the wall clock", d)
	}
	fixed := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if got := (DevOptions{Now: func() time.Time { return fixed }}).now(); !got.Equal(fixed) {
		t.Errorf("custom now() = %s, want %s", got, fixed)
	}
}

func TestListCwdProcesses_FailuresAndOutputSurface(t *testing.T) {
	dir := t.TempDir()
	if _, err := listCwdProcesses(context.Background(), stubExec(func([]string) (string, int, error) { return "", -1, errors.New("no lsof") }), dir); err == nil {
		t.Error("a runner error: nil error, want it surfaced")
	}
	if _, err := listCwdProcesses(context.Background(), stubExec(func([]string) (string, int, error) { return "", 1, nil }), dir); err == nil {
		t.Error("exit 1 with no output: nil error, want a failure")
	}
	procs, err := listCwdProcesses(context.Background(), stubExec(func([]string) (string, int, error) { return "p12\nR1\nn/some/dir\n", 1, nil }), dir)
	if err != nil || len(procs) != 1 || procs[0].Pid != 12 || procs[0].Cwd != "/some/dir" {
		t.Errorf("procs = %+v, err %v; want pid 12 in /some/dir (lsof exits 1 when it lists a subset)", procs, err)
	}
}
