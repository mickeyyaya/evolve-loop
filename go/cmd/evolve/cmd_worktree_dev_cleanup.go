package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type devCleanupOptions struct {
	mergedPRHeads mergedPRHeadsFunc
	dryRun        bool
	now           time.Time
}

const (
	devRemovable = 0
	devKept      = 1
	devFailed    = 2
)

type devOutcome struct {
	code   int
	branch string
	detail string
}

func devKeep(format string, args ...any) devOutcome {
	return devOutcome{code: devKept, detail: fmt.Sprintf(format, args...)}
}

func devFail(err error) devOutcome { return devOutcome{code: devFailed, detail: err.Error()} }

type devEffects struct {
	prepare     func(c devCleaner, stdout io.Writer) error
	remove      func(c devCleaner, dir string, j devJudgement) devOutcome
	verb, tally string
}

func devEffectsFor(dryRun bool) devEffects {
	if dryRun {
		return devEffects{prepare: devCleaner.previewMain, remove: devCleaner.keepForPreview, verb: "would remove", tally: "would be removed"}
	}
	return devEffects{prepare: devCleaner.fetchMain, remove: devCleaner.remove, verb: "removed", tally: "removed"}
}

type devCleaner struct {
	ctx       context.Context
	hub       devHub
	store     gitexec.Git
	opts      devCleanupOptions
	effects   devEffects
	quietFor  time.Duration
	worktrees map[string]bool
}

func newDevCleaner(projectRoot string, opts devCleanupOptions, quietFor time.Duration) (devCleaner, devOutcome) {
	hub, err := resolveDevHub(projectRoot)
	if err != nil {
		return devCleaner{}, devKeep("refused: %v", err)
	}
	if opts.now.IsZero() {
		opts.now = time.Now()
	}
	c := devCleaner{ctx: context.Background(), hub: hub, store: gitexec.Default(hub.store), opts: opts, effects: devEffectsFor(opts.dryRun), quietFor: quietFor}
	porcelain, err := c.store.Output(c.ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return devCleaner{}, devFail(err)
	}
	c.worktrees = map[string]bool{}
	for _, wt := range parseWorktreeList(porcelain) {
		c.worktrees[resolvedPath(wt.path)] = true
	}
	return c, devOutcome{}
}

func (c devCleaner) fetchMain(io.Writer) error { return fetchOriginMain(c.ctx, c.store) }

func (c devCleaner) keepForPreview(_ string, j devJudgement) devOutcome {
	return devOutcome{code: devRemovable, branch: j.branch, detail: j.how}
}

func devQuietPeriod(projectRoot string, stderr io.Writer) time.Duration {
	pol, err := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		fmt.Fprintf(stderr, "evolve worktree cleanup: WARN: policy load failed: %v; using the default quiet period\n", err)
	}
	if pol.GC == nil {
		return gcpolicy.WorktreesPolicy{}.DevQuietPeriod()
	}
	return pol.GC.Worktrees.DevQuietPeriod()
}

func (c devCleaner) previewMain(stdout io.Writer) error {
	sha, err := c.store.Output(c.ctx, "rev-parse", devOriginMain)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "dry run: judging against the local origin/main at %s, without fetching\n", shortSHA(sha))
	return nil
}

func runWorktreeCleanupDev(projectRoot, task string, opts devCleanupOptions, stdout, stderr io.Writer) int {
	const name = "evolve worktree cleanup --dev"
	c, refused := newDevCleaner(projectRoot, opts, 0)
	if refused.code != devRemovable {
		fmt.Fprintf(stderr, "%s: %s\n", name, refused.detail)
		return refused.code
	}
	dir := c.hub.taskDir(task)
	if _, err := os.Stat(dir); err != nil {
		fmt.Fprintf(stderr, "%s: refused: no dev worktree at %s: %v\n", name, dir, err)
		return devKept
	}
	if err := c.effects.prepare(c, stdout); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return devFailed
	}
	o := c.cleanup(dir)
	if o.code != devRemovable {
		fmt.Fprintf(stderr, "%s: %s\n", name, o.detail)
		return o.code
	}
	fmt.Fprintf(stdout, "%s %s and branch %s (%s)\n", c.effects.verb, dir, o.branch, o.detail)
	return o.code
}

func runWorktreeCleanupDevAll(projectRoot string, opts devCleanupOptions, stdout, stderr io.Writer) int {
	const name = "evolve worktree cleanup --dev --all"
	c, refused := newDevCleaner(projectRoot, opts, devQuietPeriod(projectRoot, stderr))
	if refused.code != devRemovable {
		fmt.Fprintf(stderr, "%s: %s\n", name, refused.detail)
		return refused.code
	}
	tasks, err := os.ReadDir(c.hub.dev)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return devFailed
	}
	if err := c.effects.prepare(c, stdout); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return devFailed
	}
	counts := [3]int{}
	for _, t := range tasks {
		if !t.IsDir() {
			continue
		}
		o := c.cleanup(c.hub.taskDir(t.Name()))
		counts[o.code]++
		c.report(stdout, stderr, "dev/"+t.Name(), o)
	}
	fmt.Fprintf(stdout, "%s: %d %s, %d kept, %d failed\n", name, counts[devRemovable], c.effects.tally, counts[devKept], counts[devFailed])
	if counts[devFailed] > 0 {
		return devFailed
	}
	return 0
}

func (c devCleaner) report(stdout, stderr io.Writer, task string, o devOutcome) {
	switch o.code {
	case devKept:
		fmt.Fprintf(stdout, "kept %s: %s\n", task, o.detail)
	case devFailed:
		fmt.Fprintf(stderr, "failed %s: %s\n", task, o.detail)
	default:
		fmt.Fprintf(stdout, "%s %s and branch %s (%s)\n", c.effects.verb, task, o.branch, o.detail)
	}
}

type devJudgement struct {
	branch, head, how string
	dirty             bool
	state             devTreeState
}

func (c devCleaner) cleanup(dir string) devOutcome {
	j, o := c.judge(dir)
	if o.code != devRemovable {
		return o
	}
	return c.effects.remove(c, dir, j)
}

func (c devCleaner) judge(dir string) (devJudgement, devOutcome) {
	if !c.worktrees[resolvedPath(dir)] {
		return devJudgement{}, devKeep("refused: %s is not a worktree of the hub store", dir)
	}
	tree := devReadGit(dir)
	adminDir, err := tree.Output(c.ctx, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return devJudgement{}, devFail(err)
	}
	adminChange, err := lastChange(adminDir, []string{"HEAD", "index", filepath.Join("logs", "HEAD")})
	if err != nil {
		return devJudgement{}, devFail(err)
	}
	branch, gitErr, code, err := tree.Capture(c.ctx, "symbolic-ref", "--quiet", "--short", "HEAD")
	switch {
	case err != nil || code > 1:
		return devJudgement{}, devFail(fmt.Errorf("git symbolic-ref rc=%d err=%v: %s", code, err, strings.TrimSpace(gitErr)))
	case code == 1:
		return devJudgement{}, devKeep("refused: %s is not on a branch (detached HEAD)", dir)
	}
	j := devJudgement{branch: strings.TrimSpace(branch)}
	if j.head, err = tree.HEAD(c.ctx); err != nil {
		return j, devFail(err)
	}
	dirty, err := changedPaths(c.ctx, tree)
	if err != nil {
		return j, devFail(err)
	}
	j.dirty = len(dirty) > 0
	if o, quiet := c.quiet(dir, adminChange, dirty); !quiet {
		return j, o
	}
	return c.prove(dir, tree, j)
}

func (c devCleaner) quiet(dir string, adminChange time.Time, dirty []string) (devOutcome, bool) {
	if c.quietFor <= 0 {
		return devOutcome{}, true
	}
	last := adminChange
	t, err := lastChange(dir, dirty)
	if err != nil {
		return devFail(err), false
	}
	if t.After(last) {
		last = t
	}
	if c.opts.now.Sub(last) < c.quietFor {
		return devKeep("refused: %s changed within the last %s (at %s); a lane may still be working in it", dir, c.quietFor, last.Format(time.RFC3339)), false
	}
	return devOutcome{}, true
}

func changedPaths(ctx context.Context, tree gitexec.Git) ([]string, error) {
	out, gitErr, code, err := tree.Capture(ctx, "status", "--porcelain", "-z", "-uall")
	if err != nil || code != 0 {
		return nil, fmt.Errorf("git status -z rc=%d err=%v: %s", code, err, strings.TrimSpace(gitErr))
	}
	if out == "" {
		return nil, nil
	}
	entries := strings.Split(strings.TrimSuffix(out, "\x00"), "\x00")
	var paths []string
	for i := 0; i < len(entries); i++ {
		if len(entries[i]) < len("XY p") {
			return nil, fmt.Errorf("git status -z: unexpected entry %q", entries[i])
		}
		paths = append(paths, entries[i][3:])
		if status := entries[i][0]; (status == 'R' || status == 'C') && i+1 < len(entries) {
			i++
			paths = append(paths, entries[i])
		}
	}
	return paths, nil
}

func lastChange(root string, rel []string) (time.Time, error) {
	var newest time.Time
	for _, p := range rel {
		info, err := os.Lstat(filepath.Join(root, p))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil:
			return time.Time{}, fmt.Errorf("quiet check: %w", err)
		case info.ModTime().After(newest):
			newest = info.ModTime()
		}
	}
	return newest, nil
}

func (c devCleaner) prove(dir string, tree gitexec.Git, j devJudgement) (devJudgement, devOutcome) {
	if !j.dirty {
		switch _, gitErr, code, err := c.store.Capture(c.ctx, "merge-base", "--is-ancestor", j.head, devOriginMain); {
		case err != nil || code > 1:
			return j, devFail(fmt.Errorf("git merge-base --is-ancestor rc=%d err=%v: %s", code, err, strings.TrimSpace(gitErr)))
		case code == 0:
			return c.landed(j, "head is in origin/main")
		}
	}
	state, err := readDevTreeState(c.ctx, tree, j.head)
	if err != nil {
		return j, devFail(err)
	}
	j.state = state
	verdict, err := worktreeLandedInMain(c.ctx, tree, state)
	if err != nil {
		return j, devFail(err)
	}
	if verdict.Landed {
		return c.landed(j, fmt.Sprintf("every change since merge-base %s is already in origin/main", shortSHA(state.base)))
	}
	if j.dirty {
		return j, devKeep("refused: %s is dirty with work not in origin/main (%s); tree and branch kept", dir, verdict.Reason)
	}
	merged, how := c.mergedPR(dir, j)
	if merged {
		return c.landed(j, how)
	}
	return j, devKeep("refused: branch %s is not merged into origin/main (%s; %s); tree and branch kept", j.branch, verdict.Reason, how)
}

func (c devCleaner) landed(j devJudgement, how string) (devJudgement, devOutcome) {
	j.how = how
	return j, devOutcome{code: devRemovable, branch: j.branch, detail: how}
}

func (c devCleaner) mergedPR(dir string, j devJudgement) (bool, string) {
	out, err := c.opts.mergedPRHeads(c.ctx, dir, j.branch)
	switch {
	case errors.Is(err, exec.ErrNotFound):
		return false, "gh is unavailable to prove a squash-merged PR"
	case err != nil:
		return false, fmt.Sprintf("gh pr list failed: %v", err)
	case mergedPRHeadIs(out, j.head):
		return true, fmt.Sprintf("gh reports a merged PR for %s whose head is %s", j.branch, j.head)
	}
	return false, fmt.Sprintf("no merged PR for %s has %s as its head", j.branch, j.head)
}

func (c devCleaner) remove(dir string, j devJudgement) devOutcome {
	if j.state.head != "" {
		now, err := readDevTreeState(c.ctx, devReadGit(dir), j.head)
		if err != nil {
			return devFail(err)
		}
		if !now.equal(j.state) {
			return devKeep("refused: %s changed while it was being checked; tree and branch kept", dir)
		}
	}
	args := []string{"worktree", "remove", dir}
	if j.dirty {
		args = []string{"worktree", "remove", "--force", dir}
	}
	if _, gitErr, code, err := c.store.Capture(c.ctx, args...); err != nil || code != 0 {
		return devFail(fmt.Errorf("git worktree remove rc=%d err=%v: %s", code, err, strings.TrimSpace(gitErr)))
	}
	if _, gitErr, code, err := c.store.Capture(c.ctx, "update-ref", "-d", "refs/heads/"+j.branch, j.head); err != nil || code != 0 {
		return devFail(fmt.Errorf("tree removed but deleting branch %s at %s failed rc=%d err=%v: %s", j.branch, j.head, code, err, strings.TrimSpace(gitErr)))
	}
	return devOutcome{code: devRemovable, branch: j.branch, detail: j.how}
}
