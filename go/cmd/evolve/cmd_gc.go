package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// runGC implements `evolve gc [--dry-run]`.
func runGC(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve gc", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dryRun := fs.Bool("dry-run", false, "preview only: list what WOULD be released (orphan tmux sessions and sockets, finished-cycle orphan processes, worktrees and branches, run dirs, logs of the log catalog, go build cache entries), mutating nothing")
	// The back-quoted `dir` is the flag package's argument placeholder (it
	// renders as "-project-root dir"); no other back-quotes here, or the first
	// one would be consumed as the placeholder instead.
	projectRoot := fs.String("project-root", "", "repository root `dir` the workspace (worktree/branch) sweep is aimed at; required: a non-dry run without it is refused, while --dry-run alone falls back to the current directory.\n\tNOTE the deliberate asymmetry: an explicit 'evolve gc' APPLIES the workspace sweep (an operator run is enforce),\n\twhile the in-loop hook's default mode stays 'shadow' (plan + publish only) — policy.json owns the loop's mode,\n\tthe operator owns their own invocation. Use --dry-run to preview.")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	root, code, ok := resolveGCProjectRoot(*projectRoot, *dryRun, stderr)
	if !ok {
		return code
	}
	// Bound the sweep so a wedged tmux socket can't hang the command.
	ctx, cancel := context.WithTimeout(context.Background(), orphanGCTimeout)
	defer cancel()

	r := newGCRun(ctx, *dryRun, stdout, stderr)
	failed := r.sessions()
	failed = r.sockets() || failed
	failed = r.project(root) != 0 || failed
	if failed {
		return 1
	}
	return 0
}

type gcReapers struct {
	sessions func(context.Context) swarm.OrphanReapReport
	sockets  func(context.Context) swarm.OrphanSocketReport
}

var gcInjectedReapers *gcReapers

type gcRun struct {
	ctx            context.Context
	dryRun         bool
	stdout, stderr io.Writer
	reapers        gcReapers
	kill           func(pid int) error
	remove         func(string) error
	removeAll      func(string) error
}

func newGCRun(ctx context.Context, dryRun bool, stdout, stderr io.Writer) gcRun {
	r := gcRun{ctx: ctx, dryRun: dryRun, stdout: stdout, stderr: stderr,
		reapers: gcReapers{sessions: swarm.ExecReapOrphans, sockets: swarm.ExecReapOrphanSockets},
		kill:    func(pid int) error { return syscall.Kill(pid, syscall.SIGTERM) },
		remove:  os.Remove, removeAll: os.RemoveAll}
	if gcInjectedReapers != nil {
		r.reapers = *gcInjectedReapers
	}
	if dryRun {
		noop := func(string) error { return nil }
		r.kill, r.remove, r.removeAll = func(int) error { return nil }, noop, noop
	}
	return r
}

func (r gcRun) summary(preview, applied string, args ...any) {
	if r.dryRun {
		fmt.Fprintf(r.stdout, "evolve gc --dry-run: "+preview+"\n", args...)
		return
	}
	fmt.Fprintf(r.stdout, "evolve gc: "+applied+"\n", args...)
}

func (r gcRun) sessions() bool {
	var rep swarm.OrphanReapReport
	if r.dryRun {
		noop := func(_ context.Context, _ string) error { return nil }
		rep = swarm.ReapOrphanSessions(r.ctx, swarm.ExecListBridgeSessions, swarm.ExecPidAlive, noop)
	} else {
		rep = r.reapers.sessions(r.ctx)
	}
	r.summary("%d orphan session(s) would be reaped", "reaped %d orphan session(s)", len(rep.Killed))
	verb := "reaped"
	if r.dryRun {
		verb = "WOULD-REAP"
	}
	for _, s := range rep.Killed {
		fmt.Fprintf(r.stdout, "  %s %s\n", verb, s)
	}
	fmt.Fprintf(r.stdout, "skipped: live=%d foreign=%d no-pid=%d; errors=%d\n",
		rep.SkippedLive, rep.SkippedForeign, rep.SkippedUnparseable, len(rep.Errors))
	for _, e := range rep.Errors {
		fmt.Fprintf(r.stderr, "evolve gc: error: %s\n", e)
	}
	return len(rep.Errors) > 0
}

func (r gcRun) sockets() bool {
	var srep swarm.OrphanSocketReport
	if r.dryRun {
		noopKill := func(_ context.Context, _ string) error { return nil }
		srep = swarm.ReapOrphanSockets(r.ctx, swarm.ExecListBridgeSockets, swarm.ExecPidAlive, noopKill)
	} else {
		srep = r.reapers.sockets(r.ctx)
	}
	r.summary("%d dead per-run socket(s) would be reaped", "reaped %d dead per-run socket(s)", len(srep.Killed))
	for _, s := range srep.Killed {
		fmt.Fprintf(r.stdout, "  socket %s\n", s)
	}
	for _, e := range srep.Errors {
		fmt.Fprintf(r.stderr, "evolve gc: socket error: %s\n", e)
	}
	return len(srep.Errors) > 0
}

func (r gcRun) project(projectRoot string) int {
	projectRoot, code, ok := resolveGCProjectRoot(projectRoot, r.dryRun, r.stderr)
	if !ok {
		return code
	}
	projectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: resolve --project-root: %v\n", err)
		return 1
	}
	evolveDir := filepath.Join(projectRoot, ".evolve")
	pol, err := policy.Load(filepath.Join(evolveDir, "policy.json"))
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: WARN: policy load failed: %v; using zero-value gc policy\n", err)
	}
	var gcPol gc.Policy
	if pol.GC != nil {
		gcPol = *pol.GC
	}
	opts := worktreeGCOptions(projectRoot, evolveDir, gcPol.Worktrees)
	before, freeErr := looppreflight.DiskFreeBytes(projectRoot)
	failed := r.cycleProcesses(opts)
	failed = r.worktrees(opts) || failed
	failed = r.runDirs(evolveDir, gcPol) || failed
	failed = r.goCache(gcPol) || failed
	failed = r.pipelineTemp(gcPol.TempTTLHours) || failed
	if !r.dryRun && freeErr == nil {
		r.reportDiskFree(projectRoot, before)
	}
	if failed {
		return 1
	}
	return 0
}

func (r gcRun) cycleProcesses(opts gc.WorktreeOptions) bool {
	rep := gc.ReapFinishedCycleOrphans(r.ctx, opts, r.kill)
	r.summary("%d finished-cycle orphan process(es) would be terminated", "terminated %d finished-cycle orphan process(es)", len(rep.Reaped))
	verb := "terminated"
	if r.dryRun {
		verb = "WOULD-TERMINATE"
	}
	for _, p := range rep.Reaped {
		fmt.Fprintf(r.stdout, "  %s pid=%d cwd=%s\n", verb, p.Pid, p.Cwd)
	}
	for _, e := range rep.Errors {
		fmt.Fprintf(r.stderr, "evolve gc: process error: %s\n", e)
	}
	return len(rep.Errors) > 0
}

func (r gcRun) worktrees(opts gc.WorktreeOptions) bool {
	manifest, err := gc.PlanWorktrees(opts)
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: workspace sweep plan failed: %v\n", err)
		return true
	}
	planned, flagged := gcWorktreeCounts(manifest)
	r.summary("%d workspace item(s) would be reaped (%d flagged for manual review)", "applying workspace sweep — %d item(s) (%d flagged for manual review)", planned, flagged)
	for _, it := range manifest.Items {
		switch it.Action {
		case gc.WorktreeActionFlagDirty, gc.WorktreeActionFlagUnmerged:
			// Never prefixed WOULD-: a flag is not a planned mutation in
			// either mode — it is work this sweep is refusing to touch.
			fmt.Fprintf(r.stdout, "  %s %s (%s)\n", strings.ToUpper(string(it.Action)), gcWorktreeItemLabel(it), it.Reason)
		default:
			fmt.Fprintf(r.stdout, "  WOULD-%s %s (%s)\n", strings.ToUpper(string(it.Action)), gcWorktreeItemLabel(it), it.Reason)
		}
	}
	if r.dryRun {
		return false
	}
	if err := gc.ApplyWorktrees(opts, manifest); err != nil {
		// Partial application is NORMAL: ApplyWorktrees joins per-item
		// refusals (a branch that became unmerged, a worktree that went dirty
		// since planning). Report and continue — refusing to reap is the
		// safe direction, so it is not a command failure.
		fmt.Fprintf(r.stderr, "evolve gc: workspace sweep partial: %v\n", err)
	}
	fmt.Fprintf(r.stdout, "evolve gc: workspace sweep applied\n")
	return false
}

func (r gcRun) runDirs(evolveDir string, pol gc.Policy) bool {
	discoverFailed := false
	m, err := planRunDirGC(evolveDir, pol, func(err error) {
		discoverFailed = true
		fmt.Fprintf(r.stderr, "evolve gc: run-dir discovery failed: %v; planning only the TTL rules\n", err)
	})
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: run-dir retention plan failed: %v\n", err)
		return true
	}
	r.summary("run-dir retention — %d item(s)", "applying run-dir retention — %d item(s)", len(m.Items))
	for _, w := range m.Warnings {
		fmt.Fprintf(r.stderr, "evolve gc: WARN: %s\n", w)
	}
	verb := ""
	if r.dryRun {
		verb = "WOULD-"
	}
	for _, it := range m.Items {
		fmt.Fprintf(r.stdout, "  %s%s %s (%s)\n", verb, strings.ToUpper(string(it.Action)), it.Path, it.Rule)
	}
	if r.dryRun {
		return discoverFailed
	}
	if err := gc.Apply(evolveDir, m); err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: run-dir retention partial: %v\n", err)
		return true
	}
	return discoverFailed
}

func (r gcRun) goCache(pol gc.Policy) bool {
	dir, err := gcGoCacheDir(r.ctx)
	if errors.Is(err, exec.ErrNotFound) {
		fmt.Fprintf(r.stdout, "evolve gc: go build cache trim skipped: no go toolchain on PATH\n")
		return false
	}
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: go build cache trim skipped: %v\n", err)
		return true
	}
	capBytes := pol.GoCacheMaxBytes()
	rep := gc.TrimGoCache(dir, gc.GoCacheBounds{Now: time.Now(), UnusedFor: time.Duration(pol.GoCacheTTLHours) * time.Hour, MaxBytes: capBytes}, r.remove)
	if pol.GoCacheTTLHours > 0 {
		r.summary("%d go build cache file(s) would be trimmed (%s unused > %dh in %s)", "trimmed %d go build cache file(s) (%s unused > %dh in %s)", rep.Files, gcSize(rep.Bytes), pol.GoCacheTTLHours, dir)
	} else {
		fmt.Fprintf(r.stdout, "evolve gc: go build cache age trim off (gc.go_cache_ttl_hours unset)\n")
	}
	r.summary("%d go build cache file(s) would be trimmed to fit the %s cap (%s, least recently used first); %s would remain", "trimmed %d go build cache file(s) to fit the %s cap (%s, least recently used first); %s remains", rep.CapFiles, gcSize(capBytes), gcSize(rep.CapBytes), gcSize(rep.RemainingBytes))
	if rep.RemainingBytes > capBytes {
		fmt.Fprintf(r.stderr, "evolve gc: WARN: go build cache still holds %s, over its %s cap: the cap never trims an entry whose mtime is under %s old (the go command refreshes an entry's mtime once it is an hour old, so this covers every entry looked up within about the last hour, plus a one-hour hold)\n", gcSize(rep.RemainingBytes), gcSize(capBytes), gc.GoCacheInUseWindow)
	}
	for _, e := range rep.Errors {
		fmt.Fprintf(r.stderr, "evolve gc: go build cache error: %s\n", e)
	}
	return len(rep.Errors) > 0
}

func (r gcRun) pipelineTemp(ttlHours int) bool {
	if ttlHours <= 0 {
		fmt.Fprintf(r.stdout, "evolve gc: pipeline temp sweep off (gc.temp_ttl_hours unset)\n")
		return false
	}
	dir := os.TempDir()
	rep := gc.ReapPipelineTemp(dir, time.Now().Add(-time.Duration(ttlHours)*time.Hour), r.removeAll)
	r.summary("%d stale pipeline temp artifact(s) would be removed (%s unused > %dh in %s)", "removed %d stale pipeline temp artifact(s) (%s unused > %dh in %s)", rep.Entries, gcSize(rep.Bytes), ttlHours, dir)
	for _, e := range rep.Errors {
		fmt.Fprintf(r.stderr, "evolve gc: temp error: %s\n", e)
	}
	return len(rep.Errors) > 0
}

func (r gcRun) reportDiskFree(path string, before uint64) {
	after, err := looppreflight.DiskFreeBytes(path)
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: disk free after the run unreadable: %v\n", err)
		return
	}
	fmt.Fprintf(r.stdout, "evolve gc: disk free %s → %s (net released %s)\n", gcSize(int64(before)), gcSize(int64(after)), gcSize(int64(after)-int64(before)))
}

func gcGoCacheDir(ctx context.Context) (string, error) {
	var out strings.Builder
	code, err := sysexec.DefaultRunner(ctx, "go", "", []string{"env", "GOCACHE"}, nil, nil, &out, nil)
	if err != nil {
		return "", fmt.Errorf("go env GOCACHE: %w", err)
	}
	if code != 0 {
		return "", fmt.Errorf("go env GOCACHE: exit %d", code)
	}
	dir := strings.TrimSpace(out.String())
	if dir == "" || dir == "off" {
		return "", fmt.Errorf("GOCACHE is %q", dir)
	}
	return dir, nil
}

func gcSize(b int64) string { return fmt.Sprintf("%.2f GB", float64(b)/1e9) }

// resolveGCProjectRoot resolves --project-root for a workspace sweep: an
// explicit value always passes through; an empty value on a mutating run is
// refused (a sweep requires explicit aim, not whatever repo we happen to be
// standing in), while --dry-run alone may fall back to cwd. ok=false means
// the caller must return code immediately.
func resolveGCProjectRoot(projectRoot string, dryRun bool, stderr io.Writer) (string, int, bool) {
	if projectRoot != "" {
		return projectRoot, 0, true
	}
	if !dryRun {
		fmt.Fprintf(stderr, "evolve gc: mutating run refused: --project-root must be explicitly set\n")
		return "", 1, false
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "evolve gc: workspace sweep skipped: no --project-root and cwd is unreadable: %v\n", err)
		return "", 1, false
	}
	return cwd, 0, true
}

// gcWorktreeItemLabel renders an item's identity: branch-only backlog entries
// have no worktree dir, so naming the path unconditionally would print an
// empty field for exactly the entries the branch sweep is about.
func gcWorktreeItemLabel(it gc.WorktreeItem) string {
	if it.Path == "" {
		return "branch=" + it.Branch
	}
	if it.Branch == "" {
		return "path=" + it.Path
	}
	return "branch=" + it.Branch + " path=" + it.Path
}

// gcWorktreeCounts splits a manifest into mutating (planned) and flag-only
// (never touched) items.
func gcWorktreeCounts(m gc.WorktreeManifest) (planned, flagged int) {
	for _, it := range m.Items {
		switch it.Action {
		case gc.WorktreeActionFlagDirty, gc.WorktreeActionFlagUnmerged:
			flagged++
		default:
			planned++
		}
	}
	return planned, flagged
}
