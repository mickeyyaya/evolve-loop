package gc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

func (e *devEnv) exclude(patterns ...string) {
	e.t.Helper()
	p := filepath.Join(e.store.Dir, "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		e.t.Fatal(err)
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		e.t.Fatal(err)
	}
	defer f.Close()
	f.WriteString(strings.Join(patterns, "\n") + "\n")
}

func TestPlanDevWorktrees_AnIgnoredFileBlocksTheArchiveAndIsNamed(t *testing.T) {
	e := newDevEnv(t)
	e.exclude(".env", ".envrc", "local.cfg", "extra.cfg")
	dir := e.addDev("secrets")
	for _, f := range []string{".env", ".envrc", "local.cfg", "extra.cfg"} {
		writeRepoFile(t, dir, f, "KEY=1\n")
	}
	o := e.options(idleOffset)
	it := devItemFor(t, e.plan(o), dir)
	if it.Action != DevActionKeep || it.Failed || !strings.HasPrefix(it.Reason, "holds ignored files: ") || !strings.Contains(it.Reason, ".env") || !strings.Contains(it.Reason, "and 1 more") {
		t.Fatalf("item = %+v, want kept with a reason that names the first ignored files and counts the rest", it)
	}
	m := DevManifest{Items: []DevItem{{Path: dir, Action: DevActionArchive, Head: it.Head}}}
	recs, err := e.apply(o, m)
	if err == nil || len(recs) != 0 {
		t.Fatalf("apply = %+v, %v; want a refusal", recs, err)
	}
	if got, rerr := os.ReadFile(filepath.Join(dir, ".env")); rerr != nil || string(got) != "KEY=1\n" {
		t.Errorf(".env = %q (err %v), want it untouched", got, rerr)
	}
}

func TestPlanDevWorktrees_OnlyRebuildableIgnoredOutputsAllowTheArchive(t *testing.T) {
	e := newDevEnv(t)
	e.exclude("go/bin/", "node_modules/", "go/coverage*.txt", "go/evolve")
	dir := e.addDev("builds")
	writeRepoFile(t, dir, "go/bin/evolve", "bin\n")
	writeRepoFile(t, dir, "web/node_modules/pkg/index.js", "x\n")
	writeRepoFile(t, dir, "go/coverage.txt", "mode: atomic\n")
	writeRepoFile(t, dir, "go/evolve", "bin\n")
	it := devItemFor(t, e.plan(e.options(idleOffset)), dir)
	if it.Action != DevActionArchive {
		t.Fatalf("item = %+v, want archive: every ignored entry is a rebuildable output", it)
	}
}

func TestRebuildableIgnored(t *testing.T) {
	cases := map[string]bool{
		"go/bin/": true, "go/bin/evolve": true, "go/evolve": true, "go/coverage.txt": true, "go/coverage.func.txt": true,
		"node_modules/": true, "web/node_modules/": true, ".gocache/": true, ".gomodcache/": true,
		".env": false, "go/evolve.bak": false, "go/coverage.out": false, "src/node_modules_old/": false, "notes.local": false,
	}
	for entry, want := range cases {
		if got := rebuildableIgnored(entry); got != want {
			t.Errorf("rebuildableIgnored(%q) = %v, want %v", entry, got, want)
		}
	}
	for pat, why := range devRebuildableIgnored {
		if why == "" {
			t.Errorf("allowlist entry %q has no justification", pat)
		}
	}
}

func TestFirstNames(t *testing.T) {
	if got := firstNames([]string{"a", "b"}); got != "a, b" {
		t.Errorf("firstNames = %q", got)
	}
	if got := firstNames([]string{"a", "b", "c", "d", "e"}); got != "a, b, c and 2 more" {
		t.Errorf("firstNames = %q", got)
	}
}

func TestPlanDevWorktrees_ANestedGitRepositoryBlocksTheArchive(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("nested")
	inner := filepath.Join(dir, "vendor", "lib")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", inner, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	writeRepoFile(t, inner, "inner.txt", "inner work\n")
	it := devItemFor(t, e.plan(e.options(idleOffset)), dir)
	if it.Action != DevActionKeep || !strings.Contains(it.Reason, "holds a nested git repository") {
		t.Fatalf("item = %+v, want kept for the nested repository", it)
	}
}

func TestPlanDevWorktrees_LeftoverListFailuresMarkTheTreeNotJudged(t *testing.T) {
	for _, flag := range []string{"--ignored", "--directory"} {
		e := newDevEnv(t)
		dir := e.addDev("lsfiles")
		o := e.options(idleOffset)
		o.Exec = func(ctx context.Context, name, d string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
			if len(args) > 0 && args[0] == "ls-files" && containsArg(args, flag) && (flag == "--ignored" || !containsArg(args, "--ignored")) {
				return 128, nil
			}
			return sysexec.DefaultRunner(ctx, name, d, args, env, in, out, errw)
		}
		if it := devItemFor(t, e.plan(o), dir); !it.Failed || it.Action != DevActionKeep {
			t.Errorf("ls-files failing on %s: %+v, want kept and not judged", flag, it)
		}
	}
}

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func TestApplyDevWorktrees_ATreeEditedWhileItsCheckpointIsSavedIsKeptNotArchived(t *testing.T) {
	cases := map[string]func(dir string){
		"a new file":                           func(dir string) { writeRepoFile(t, dir, "late.txt", "late\n") },
		"new content in an already dirty file": func(dir string) { writeRepoFile(t, dir, "wip.txt", "changed after the save\n") },
	}
	for name, edit := range cases {
		e := newDevEnv(t)
		dir := e.addDev("racy")
		writeRepoFile(t, dir, "wip.txt", "first\n")
		o := e.options(idleOffset)
		m := e.plan(o)
		calls := 0
		o.Save = func(ctx context.Context, w wtcheckpoint.Worktree) (wtcheckpoint.SaveResult, error) {
			res, err := wtcheckpoint.Save(ctx, w, wtcheckpoint.SaveOptions{Now: func() time.Time { return o.Now().Add(time.Duration(calls) * time.Second) }})
			if calls == 0 {
				edit(dir)
			}
			calls++
			return res, err
		}
		recs, err := e.apply(o, m)
		if err == nil || !strings.Contains(err.Error(), "changed while its checkpoint was saved") || len(recs) != 0 {
			t.Errorf("%s: apply = %+v, %v; want the race refused", name, recs, err)
		}
		if _, serr := os.Stat(filepath.Join(dir, "wip.txt")); serr != nil {
			t.Errorf("%s: the tree lost its work: %v", name, serr)
		}
		if stored, _ := ReadDevArchive(e.evolve); len(stored) != 0 {
			t.Errorf("%s: manifest = %+v, want none", name, stored)
		}
	}
}

func TestApplyDevWorktrees_TheRaceCheckFailuresRefuse(t *testing.T) {
	boom := errors.New("git vanished")
	e := newDevEnv(t)
	dir := e.addDev("racefail")
	writeRepoFile(t, dir, "wip.txt", "w\n")
	o := e.options(idleOffset)
	m := e.plan(o)

	statusCalls := 0
	o.Exec = func(ctx context.Context, name, d string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 0 && args[0] == "status" {
			statusCalls++
			if statusCalls == 2 {
				return 0, boom
			}
		}
		return sysexec.DefaultRunner(ctx, name, d, args, env, in, out, errw)
	}
	if _, err := e.apply(o, m); err == nil || !strings.Contains(err.Error(), "dirty check failed") {
		t.Errorf("first dirty check failure: %v, want a refusal", err)
	}
	statusCalls = 0
	o.Exec = func(ctx context.Context, name, d string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 0 && args[0] == "status" {
			statusCalls++
			if statusCalls == 3 {
				return 0, boom
			}
		}
		return sysexec.DefaultRunner(ctx, name, d, args, env, in, out, errw)
	}
	if _, err := e.apply(o, m); err == nil || !strings.Contains(err.Error(), "dirty re-check failed") {
		t.Errorf("second dirty check failure: %v, want a refusal", err)
	}
	saves := 0
	o.Exec = e.options(0).Exec
	o.Save = func(ctx context.Context, w wtcheckpoint.Worktree) (wtcheckpoint.SaveResult, error) {
		saves++
		if saves == 2 {
			return wtcheckpoint.SaveResult{}, boom
		}
		return wtcheckpoint.Save(ctx, w, wtcheckpoint.SaveOptions{Now: func() time.Time { return o.Now().Add(time.Duration(saves) * time.Second) }})
	}
	if _, err := e.apply(o, m); err == nil || !strings.Contains(err.Error(), "save re-check failed") {
		t.Errorf("second save failure: %v, want a refusal", err)
	}
	if _, serr := os.Stat(filepath.Join(dir, "wip.txt")); serr != nil {
		t.Errorf("the tree lost its work: %v", serr)
	}
}

func TestApplyDevWorktrees_AnUnavailableGhNeverSkipsThePullRequestRuleOnApply(t *testing.T) {
	for name, lookup := range map[string]OpenPRLookup{
		"gh missing":      func(context.Context, string) (map[string][]time.Time, error) { return nil, ErrPRLookupUnavailable },
		"no lookup wired": nil,
	} {
		e := newDevEnv(t)
		dir := e.addDev("nogh")
		o := e.options(idleOffset)
		o.OpenPRs = lookup
		m := e.plan(o)
		if it := devItemFor(t, m, dir); it.Action != DevActionArchive || len(m.Notes) != 1 {
			t.Fatalf("%s: the dry plan = %+v notes %q, want an archive candidate and the note", name, it, m.Notes)
		}
		recs, err := e.apply(o, m)
		if err == nil || len(recs) != 0 || !strings.Contains(err.Error(), "cannot judge") {
			t.Errorf("%s: apply = %+v, %v; want the tree not judged on apply", name, recs, err)
		}
		if _, serr := os.Stat(dir); serr != nil {
			t.Errorf("%s: the tree was removed without the pull request rule: %v", name, serr)
		}
	}
}

func TestPlanDevWorktrees_ABranchWithNoNetChangesSaysSoAndIsNeverCalledInMain(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("netzero")
	e.commitIn(dir, "x.txt", "x\n")
	e.store.Git("-C", dir, "rm", "-q", "x.txt")
	e.store.Git("-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "revert x")
	it := devItemFor(t, e.plan(e.options(idleOffset)), dir)
	if it.Action != DevActionArchive || !strings.Contains(it.Reason, "has no net changes against main (2 commit(s) not in main)") {
		t.Fatalf("item = %+v, want the no-net-changes wording with the commit count", it)
	}
}

func TestSquashResultLanding(t *testing.T) {
	cases := []struct {
		name string
		r    squashResult
		want landing
		del  bool
	}{
		{"not landed", squashResult{}, landingNone, false},
		{"landed", squashResult{Landed: true}, landingSquash, true},
		{"no net change and nothing ahead", squashResult{Landed: true, NoNet: true}, landingNoNet, true},
		{"no net change with commits ahead", squashResult{Landed: true, NoNet: true, Ahead: 2}, landingNone, false},
	}
	for _, c := range cases {
		if got := c.r.landing(); got != c.want || c.r.deletable() != c.del {
			t.Errorf("%s: landing %v deletable %v, want %v %v", c.name, got, c.r.deletable(), c.want, c.del)
		}
	}
}

func TestNoNetChanges_CountFailuresSurface(t *testing.T) {
	r := newSquashRepo(t)
	r.Git("branch", "same")
	g := realGit(r.Dir)
	g.Exec = failOn("rev-list", 128, nil)
	if _, err := noNetChanges(context.Background(), g, "main", "refs/heads/same"); err == nil {
		t.Error("a failing rev-list: nil error")
	}
	g.Exec = stubExec(func([]string) (string, int, error) { return "many\n", 0, nil })
	if _, err := noNetChanges(context.Background(), g, "main", "refs/heads/same"); err == nil {
		t.Error("a non-numeric count: nil error")
	}
}

func TestBranchLandedBySquash_ABranchNamedLikeATagIsReadThroughItsFullRefName(t *testing.T) {
	r := newSquashRepo(t)
	r.Git("tag", "feature")
	r.Git("switch", "-q", "-c", "feature")
	commitRepoFile(t, r, "unlanded.txt", "not in main\n")
	r.Git("switch", "-q", "main")
	landed, err := landedBySquash(context.Background(), realGit(r.Dir), "main", "feature")
	if err != nil || landed {
		t.Fatalf("landed = %v, %v; want false: the branch holds a commit that main lacks, whatever the tag of the same name points at", landed, err)
	}
}

func TestBranchLandedBySquash_TheDiffsRunWithSubmoduleChangesVisible(t *testing.T) {
	r := newSquashRepo(t)
	r.Git("switch", "-q", "-c", "feature")
	commitRepoFile(t, r, "a.txt", "a\n")
	r.Git("switch", "-q", "main")
	var diffs [][]string
	g := realGit(r.Dir)
	g.Exec = func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if containsArg(args, "diff") {
			diffs = append(diffs, args)
		}
		return sysexec.DefaultRunner(ctx, name, dir, args, env, in, out, errw)
	}
	if _, err := branchLandedBySquash(context.Background(), g, "main", "feature"); err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 2 {
		t.Fatalf("saw %d diffs, want the name list and the comparison", len(diffs))
	}
	for _, args := range diffs {
		if !containsArg(args, "diff.ignoreSubmodules=none") {
			t.Errorf("diff %v does not turn submodule hiding off", args)
		}
	}
}

func TestDeleteBranch_AnAncestorBranchGoesThroughBranchDAndNeverUpdateRef(t *testing.T) {
	e := newCycleSquashEnv(t)
	e.repo.Git("branch", "cycle-anc-960")
	head := e.repo.Git("rev-parse", "cycle-anc-960")
	var calls []string
	var log bytes.Buffer
	o := e.options()
	o.Log = &log
	o.Exec = func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		calls = append(calls, strings.Join(args, " "))
		return sysexec.DefaultRunner(ctx, name, dir, args, env, in, out, errw)
	}
	if err := o.deleteBranch(WorktreeItem{Branch: "cycle-anc-960", Head: head, Action: WorktreeActionDeleteBranch}); err != nil {
		t.Fatal(err)
	}
	for _, c := range calls {
		if strings.HasPrefix(c, "update-ref") {
			t.Errorf("update-ref ran for a branch that git branch -d deleted: %v", calls)
		}
	}
	if !strings.Contains(log.String(), "gc: deleted branch cycle-anc-960@"+head) {
		t.Errorf("log = %q, want the branch@head line", log.String())
	}
}

func TestDeleteBranch_UpdateRefOnlyAfterGitSaysNotFullyMergedAndNoWorktreeHoldsIt(t *testing.T) {
	e := newCycleSquashEnv(t)
	e.repo.Git("switch", "-q", "-c", "cycle-sq-961")
	commitRepoFile(t, e.repo, "sq961.txt", "x\n")
	squashMerge(e.repo, "cycle-sq-961")
	head := e.repo.Git("rev-parse", "refs/heads/cycle-sq-961")
	var log bytes.Buffer
	o := e.options()
	o.Log = &log
	item := WorktreeItem{Branch: "cycle-sq-961", Head: head, Action: WorktreeActionDeleteBranch}

	if err := o.deleteBranch(item); err != nil {
		t.Fatal(err)
	}
	if e.repo.Git("branch", "--list", "cycle-sq-961") != "" {
		t.Error("the squash-merged branch survived")
	}
	if !strings.Contains(log.String(), "cycle-sq-961@"+head) {
		t.Errorf("log = %q, want branch@head", log.String())
	}
}

func TestDeleteBranch_RefusalsAndFailures(t *testing.T) {
	e := newCycleSquashEnv(t)
	e.repo.Git("switch", "-q", "-c", "cycle-held-962")
	commitRepoFile(t, e.repo, "held.txt", "x\n")
	squashMerge(e.repo, "cycle-held-962")
	head := e.repo.Git("rev-parse", "refs/heads/cycle-held-962")
	held := filepath.Join(t.TempDir(), "held-tree")
	e.repo.Git("worktree", "add", "-q", held, "cycle-held-962")
	item := WorktreeItem{Branch: "cycle-held-962", Head: head}

	notMerged := func(base sysexec.RunFunc) sysexec.RunFunc {
		return func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
			if len(args) > 1 && args[0] == "branch" && args[1] == "-d" {
				io.WriteString(errw, "error: the branch 'x' is not fully merged\n")
				return 1, nil
			}
			return base(ctx, name, dir, args, env, in, out, errw)
		}
	}
	o := e.options()
	o.Exec = notMerged(sysexec.DefaultRunner)
	if err := o.deleteBranch(item); err == nil || !strings.Contains(err.Error(), "checked out") {
		t.Errorf("a branch held by a worktree: %v, want a refusal that says so", err)
	}
	e.repo.Git("worktree", "remove", "--force", held)

	e.repo.Git("switch", "-q", "cycle-held-962")
	commitRepoFile(t, e.repo, "late.txt", "late\n")
	e.repo.Git("switch", "-q", "main")
	lateHead := e.repo.Git("rev-parse", "refs/heads/cycle-held-962")
	if err := o.deleteBranch(WorktreeItem{Branch: "cycle-held-962", Head: lateHead}); err == nil || !strings.Contains(err.Error(), "squash check no longer passes") {
		t.Errorf("a branch with a commit main lacks: %v, want the squash check refusal", err)
	}
	if e.repo.Git("branch", "--list", "cycle-held-962") == "" {
		t.Error("the branch was deleted")
	}

	if err := o.deleteBranch(WorktreeItem{Branch: "cycle-held-962"}); err == nil {
		t.Error("an item without a head after 'not fully merged': nil error, want the git failure")
	}
	other := e.options()
	other.Exec = func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 1 && args[0] == "branch" && args[1] == "-d" {
			io.WriteString(errw, "error: something else\n")
			return 1, nil
		}
		return sysexec.DefaultRunner(ctx, name, dir, args, env, in, out, errw)
	}
	if err := other.deleteBranch(item); err == nil || !strings.Contains(err.Error(), "something else") {
		t.Errorf("another git failure: %v, want it returned and no update-ref", err)
	}
	runnerDown := e.options()
	runnerDown.Exec = failOn("branch", -1, errors.New("git vanished"))
	if err := runnerDown.deleteBranch(item); err == nil {
		t.Error("a runner error: nil error")
	}
	listFails := e.options()
	listFails.Exec = func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 1 && args[0] == "worktree" && args[1] == "list" {
			return 128, nil
		}
		return notMerged(sysexec.DefaultRunner)(ctx, name, dir, args, env, in, out, errw)
	}
	if err := listFails.deleteBranch(item); err == nil {
		t.Error("a failing worktree list: nil error")
	}
	squashFails := e.options()
	squashFails.Exec = func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 0 && args[0] == "merge-base" {
			return 128, nil
		}
		return notMerged(sysexec.DefaultRunner)(ctx, name, dir, args, env, in, out, errw)
	}
	if err := squashFails.deleteBranch(item); err == nil {
		t.Error("a failing squash check: nil error")
	}
	updateFails := e.options()
	updateFails.Exec = func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 0 && args[0] == "update-ref" {
			return 128, nil
		}
		return notMerged(sysexec.DefaultRunner)(ctx, name, dir, args, env, in, out, errw)
	}
	if err := updateFails.deleteBranch(item); err == nil {
		t.Error("a failing update-ref: nil error")
	}
	if e.repo.Git("branch", "--list", "cycle-held-962") == "" {
		t.Error("the branch was deleted although every path refused")
	}
}

func TestBranchHead_AnUnknownBranchHasNoHeadAndLoggingNeedsNoWriter(t *testing.T) {
	e := newCycleSquashEnv(t)
	o := e.options()
	if got := o.branchHead("cycle-missing-1"); got != "" {
		t.Errorf("branchHead = %q, want empty", got)
	}
	o.logDeleted("b", "h")
}

func TestOrphanBranchItems_ANoNetChangeBranchWithNothingAheadIsPlannedWithItsHead(t *testing.T) {
	o := WorktreeOptions{ProjectRoot: t.TempDir(), Exec: stubExec(func(args []string) (string, int, error) {
		if args[0] == "branch" {
			return "  cycle-nn-1\n", 0, nil
		}
		return "abc123\n", 0, nil
	})}
	set := &mergedSet{o: o, ancestors: map[string]bool{}, memo: map[string]landing{"cycle-nn-1": landingNoNet}}
	items, err := o.orphanBranchItems(set, map[string]bool{})
	if err != nil || len(items) != 1 || items[0].Head != "abc123" || !strings.Contains(items[0].Reason, "no net changes") {
		t.Fatalf("items = %+v, %v; want a delete with the head and the no-net wording", items, err)
	}
	if got := removalReason(landingNoNet); !strings.Contains(got, "no net changes") {
		t.Errorf("removalReason = %q", got)
	}
}

func TestDeleteBranch_AFailingUpdateRefIsReportedAndLeavesTheBranch(t *testing.T) {
	e := newCycleSquashEnv(t)
	e.repo.Git("switch", "-q", "-c", "cycle-upd-963")
	commitRepoFile(t, e.repo, "upd.txt", "x\n")
	squashMerge(e.repo, "cycle-upd-963")
	head := e.repo.Git("rev-parse", "refs/heads/cycle-upd-963")
	o := e.options()
	o.Exec = func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		switch {
		case len(args) > 1 && args[0] == "branch" && args[1] == "-d":
			io.WriteString(errw, "error: the branch 'x' is not fully merged\n")
			return 1, nil
		case len(args) > 0 && args[0] == "update-ref":
			return 128, nil
		}
		return sysexec.DefaultRunner(ctx, name, dir, args, env, in, out, errw)
	}
	if err := o.deleteBranch(WorktreeItem{Branch: "cycle-upd-963", Head: head}); err == nil {
		t.Error("a failing update-ref: nil error")
	}
	if e.repo.Git("branch", "--list", "cycle-upd-963") == "" {
		t.Error("the branch was deleted")
	}
}

func TestPlanDevWorktrees_AnIgnoredDirectoryIsListedFileByFileAndAFailingListingMarksTheTreeNotJudged(t *testing.T) {
	e := newDevEnv(t)
	e.exclude("secrets/")
	dir := e.addDev("ignored-dir")
	writeRepoFile(t, dir, "secrets/token.txt", "t\n")
	it := devItemFor(t, e.plan(e.options(idleOffset)), dir)
	if it.Action != DevActionKeep || !strings.Contains(it.Reason, "holds ignored files: secrets/token.txt") {
		t.Fatalf("item = %+v, want the ignored directory expanded to its file", it)
	}
	o := e.options(idleOffset)
	o.Exec = func(ctx context.Context, name, d string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 0 && args[0] == "ls-files" && containsArg(args, "--") {
			return 128, nil
		}
		return sysexec.DefaultRunner(ctx, name, d, args, env, in, out, errw)
	}
	if it := devItemFor(t, e.plan(o), dir); !it.Failed {
		t.Errorf("a failing expansion: %+v, want not judged", it)
	}
}

func TestPlanDevWorktrees_ASquashCheckFailureMarksTheTreeNotJudged(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("sqfail")
	e.commitIn(dir, "x.txt", "x\n")
	o := e.options(idleOffset)
	o.Exec = func(ctx context.Context, name, d string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 1 && args[0] == "merge-base" && args[1] != "--is-ancestor" {
			return 128, nil
		}
		return sysexec.DefaultRunner(ctx, name, d, args, env, in, out, errw)
	}
	if it := devItemFor(t, e.plan(o), dir); !it.Failed || it.Action != DevActionKeep {
		t.Errorf("item = %+v, want not judged", it)
	}
}

func TestPlanDevWorktrees_AnUnreadableSubdirectoryOfAnUntrackedDirectoryMarksTheTreeNotJudged(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("unreadable")
	locked := filepath.Join(dir, "scratch", "locked")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, dir, "scratch/ok.txt", "x\n")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	it := devItemFor(t, e.plan(e.options(idleOffset)), dir)
	if it.Action != DevActionKeep || !it.Failed || !strings.Contains(it.Reason, "cannot judge") {
		t.Fatalf("item = %+v, want kept and not judged: an unreadable directory cannot be proven free of a nested repository", it)
	}
}

func TestNestedRepo_ReportsAWalkErrorAndAMissingTopDirectory(t *testing.T) {
	if _, err := nestedRepo(filepath.Join(t.TempDir(), "gone")); err == nil {
		t.Error("a missing directory: nil error")
	}
	clean := t.TempDir()
	if got, err := nestedRepo(clean); err != nil || got != "" {
		t.Errorf("an empty directory: %q, %v", got, err)
	}
}
