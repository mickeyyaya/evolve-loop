package main

import (
	"cmp"
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
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
)

func runGC(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve gc", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dryRun := fs.Bool("dry-run", false, "preview only: list what WOULD be released (orphan tmux sessions and sockets, finished-cycle orphan processes, stale dispatch processes and orphan log tails, worktrees and branches, run dirs, logs of the log catalog, go build cache entries, empty claim dirs of stale cycles), mutating nothing")
	projectRoot := fs.String("project-root", "", "repository root `dir` the workspace (worktree/branch) sweep is aimed at; required: a non-dry run without it is refused, while --dry-run alone falls back to the current directory.\n\tNOTE the deliberate asymmetry: an explicit 'evolve gc' APPLIES the workspace sweep (an operator run is enforce),\n\twhile the in-loop hook's default mode stays 'shadow' (plan + publish only) — policy.json owns the loop's mode,\n\tthe operator owns their own invocation. Use --dry-run to preview.")
	only := fs.String("only", "", "run one `scope` and no other sweep. The one scope is cache: the go build cache trim and the pipeline temp sweep. It touches no worktree, branch, session or process, so it is safe while lanes run. When gc.cache_trim_free_gb is set, it trims only while disk free is under that value")
	asJSON := fs.Bool("json", false, "print the result as JSON: the scope and the report lines")
	leaseWait := fs.Duration("lease-wait", 0, "wait up to this long for the gc lease that another gc holds; 0 refuses at once (exit 3)")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if *only != "" && *only != gcScopeCache {
		fmt.Fprintf(stderr, "evolve gc: unknown --only scope %q (valid: %s)\n", *only, gcScopeCache)
		return 10
	}
	root, code, ok := resolveGCProjectRoot(*projectRoot, *dryRun, stderr)
	if !ok {
		return code
	}
	plan := gcPlan{root: root, only: *only, dryRun: *dryRun, asJSON: *asJSON}
	if *dryRun {
		return plan.run(stdout, stderr)
	}
	release, rc := takeGCLease(root, *leaseWait, stderr)
	if rc != cliExitOK {
		return rc
	}
	defer release()
	plan.liveRuns = liveRunDirs(root, stderr)
	return plan.run(stdout, stderr)
}

type gcPlan struct {
	root, only     string
	liveRuns       []string
	dryRun, asJSON bool
}

func (p gcPlan) sweeper(stdout, stderr io.Writer) gcRun {
	r := newGCRun(context.Background(), p.dryRun, stdout, stderr)
	r.liveRuns = p.liveRuns
	return r
}

func (p gcPlan) run(stdout, stderr io.Writer) int {
	if !p.asJSON {
		return p.sweeper(stdout, stderr).sweep(p.root, p.only)
	}
	var report strings.Builder
	rc := p.sweeper(&report, stderr).sweep(p.root, p.only)
	result := gcResult{DryRun: p.dryRun, Scope: cmp.Or(p.only, "all"), LiveRuns: p.liveRuns, Report: strings.FieldsFunc(report.String(), func(r rune) bool { return r == '\n' })}
	return max(printVerbJSON("gc", result, stdout, stderr), rc)
}

func takeGCLease(root string, wait time.Duration, stderr io.Writer) (func(), int) {
	deadline, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	path := filepath.Join(paths.EvolveDirOf(root), gcLeaseFile)
	release, err := verifylock.AcquireAt(deadline, path, nil)
	if err != nil {
		fmt.Fprintf(stderr, "evolve gc: refused — another gc holds the gc lease (%s): %v\n", path, err)
		return nil, cliExitRefused
	}
	return release, cliExitOK
}

func liveRunDirs(root string, stderr io.Writer) []string {
	live := runlease.LiveRuns(filepath.Join(paths.EvolveDirOf(root), "runs"), time.Now())
	if len(live) == 0 {
		return nil
	}
	dirs := make([]string, 0, len(live))
	for _, r := range live {
		dirs = append(dirs, filepath.Base(r.Dir))
	}
	fmt.Fprintf(stderr, "evolve gc: %d run(s) hold a live lease (%s), so the sweeps that stop processes (sessions, sockets, cycle and dispatch processes) wait for a wave boundary\n", len(live), strings.Join(dirs, ", "))
	return dirs
}

type gcResult struct {
	DryRun   bool     `json:"dry_run"`
	Scope    string   `json:"scope"`
	LiveRuns []string `json:"live_runs,omitempty"`
	Report   []string `json:"report"`
}

const gcLeaseFile = "gc.lock"

func (r gcRun) sweep(root, only string) int {
	if only == gcScopeCache {
		return r.cacheScope(root)
	}
	failed := false
	if len(r.liveRuns) == 0 {
		failed = r.step(gcRun.sessions)
		failed = r.step(gcRun.sockets) || failed
	}
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
	abs            func(string) (string, error)
	liveRuns       []string
}

func newGCRun(ctx context.Context, dryRun bool, stdout, stderr io.Writer) gcRun {
	r := gcRun{ctx: ctx, dryRun: dryRun, stdout: stdout, stderr: stderr,
		reapers: gcReapers{sessions: swarm.ExecReapOrphans, sockets: swarm.ExecReapOrphanSockets},
		kill:    func(pid int) error { return syscall.Kill(pid, syscall.SIGTERM) },
		remove:  os.Remove, removeAll: os.RemoveAll, abs: filepath.Abs}
	if gcInjectedReapers != nil {
		r.reapers = *gcInjectedReapers
	}
	if dryRun {
		noop := func(string) error { return nil }
		r.kill, r.remove, r.removeAll = func(int) error { return nil }, noop, noop
	}
	return r
}

func (r gcRun) step(run func(gcRun) bool) bool {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.ctx), orphanGCTimeout)
	defer cancel()
	r.ctx = ctx
	return run(r)
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
	projectRoot, pol, loaded, ok := r.loadFullPolicy(projectRoot)
	if !ok {
		return 1
	}
	gcPol := gcBlock(pol)
	evolveDir := filepath.Join(projectRoot, ".evolve")
	opts := worktreeGCOptions(projectRoot, evolveDir, gcPol.Worktrees, r.stdout)
	before, freeErr := looppreflight.DiskFreeBytes(projectRoot)
	ageOf := continuationAge{maxAge: pol.ContinuationConfig().MaxAgeCycles, loaded: loaded}
	steps := []func(gcRun) bool{
		func(s gcRun) bool { return s.agedContinuations(projectRoot, ageOf, opts.WorktreeBase) },
		func(s gcRun) bool { return s.worktrees(opts) },
		func(s gcRun) bool { return s.devWorktrees(projectRoot, evolveDir, gcPol) },
		func(s gcRun) bool { return s.runDirs(evolveDir, gcPol) },
		func(s gcRun) bool { return s.goCache(gcPol) },
		func(s gcRun) bool { return s.pipelineTemp(gcPol.TempTTLHours) },
		func(s gcRun) bool { return s.testCache(evolveDir, gcPol) },
		func(s gcRun) bool { return s.claimDirs(projectRoot) },
	}
	if len(r.liveRuns) == 0 {
		steps = append([]func(gcRun) bool{
			func(s gcRun) bool { return s.cycleProcesses(opts) },
			func(s gcRun) bool { return s.dispatchProcesses(projectRoot, gcPol.TempTTLHours) },
		}, steps...)
	}
	failed := false
	for _, run := range steps {
		failed = r.step(run) || failed
	}
	if !r.dryRun && freeErr == nil {
		r.reportDiskFree(projectRoot, before)
	}
	if failed {
		return 1
	}
	return 0
}

func (r gcRun) loadPolicy(projectRoot string) (string, gc.Policy, bool) {
	projectRoot, pol, _, ok := r.loadFullPolicy(projectRoot)
	return projectRoot, gcBlock(pol), ok
}

func (r gcRun) loadFullPolicy(projectRoot string) (string, policy.Policy, bool, bool) {
	projectRoot, err := r.abs(projectRoot)
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: resolve --project-root: %v\n", err)
		return "", policy.Policy{}, false, false
	}
	pol, err := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: WARN: policy load failed: %v; using zero-value gc policy\n", err)
	}
	return projectRoot, pol, err == nil, true
}

func gcBlock(pol policy.Policy) gc.Policy {
	if pol.GC == nil {
		return gc.Policy{}
	}
	return *pol.GC
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
			fmt.Fprintf(r.stdout, "  %s %s (%s)\n", strings.ToUpper(string(it.Action)), gcWorktreeItemLabel(it), it.Reason)
		default:
			fmt.Fprintf(r.stdout, "  WOULD-%s %s (%s)\n", strings.ToUpper(string(it.Action)), gcWorktreeItemLabel(it), it.Reason)
		}
	}
	if r.dryRun {
		return false
	}
	if err := gc.ApplyWorktrees(opts, manifest); err != nil {
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

func resolveGCProjectRoot(projectRoot string, dryRun bool, stderr io.Writer) (string, int, bool) {
	if projectRoot != "" {
		return projectRoot, 0, true
	}
	if !dryRun {
		fmt.Fprintf(stderr, "evolve gc: mutating run refused: --project-root must be explicitly set\n")
		return "", 1, false
	}
	cwd, err := processGetwd()
	if err != nil {
		fmt.Fprintf(stderr, "evolve gc: workspace sweep skipped: no --project-root and cwd is unreadable: %v\n", err)
		return "", 1, false
	}
	return cwd, 0, true
}

func gcWorktreeItemLabel(it gc.WorktreeItem) string {
	if it.Path == "" {
		return "branch=" + it.Branch
	}
	if it.Branch == "" {
		return "path=" + it.Path
	}
	return "branch=" + it.Branch + " path=" + it.Path
}

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
