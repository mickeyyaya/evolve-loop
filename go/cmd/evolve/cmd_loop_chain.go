// cmd_loop_chain.go — the unit-13 chain seam (ADR-0103). The outer
// batch-chaining loop (cycle 1075; the standing operator directive of
// 2026-07-11 that lanes keep running until the inbox is empty) and the
// boundary binary refresh (cycle 1314) live in internal/loopchain as the
// Driver and the Refresher. This file keeps the eleven package-var test
// seams (read INSIDE the engines' closures at every construction, so a swap
// between calls is always seen), the two process adapters the leaf takes as
// required deps (`make -C go build`, syscall.Exec), the chain's stdout
// envelope, the chain root's Signal Center and the facades the by-name tests
// keep. The chain owns no cycle-level logic: the quota wall is the batch's
// rc=5 contract, which the Driver refuses to relaunch into.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/mickeyyaya/evolve-loop/go/internal/loopchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/pkg/version"
)

// chainBrakeFile is the operator brake: `touch .evolve/loop-stop` and the
// chain stops at the next boundary.
const chainBrakeFile = paths.LoopStopFile

// The test seams — every one a package var the chain suites swap between
// calls; wiredRefresher and wiredChain read them at call time.
var (
	// runLoopBatchFn is the scripted batch (production: the real batch).
	runLoopBatchFn = runLoopBatch
	// chainBoundaryAheadFn is the "is my binary stale" check.
	chainBoundaryAheadFn = defaultChainBoundaryAhead
	// chainRunningCommitFn resolves the running binary's build commit.
	chainRunningCommitFn = version.Commit
	// chainBoundaryRepinProvenanceFn resolves the build commit + the
	// provenance predicate authorizing the boundary re-pin.
	chainBoundaryRepinProvenanceFn = defaultChainBoundaryRepinProvenance
	// chainReExecTargetFn resolves the executable the refresh re-execs into.
	chainReExecTargetFn = defaultChainReExecTarget
	// chainBoundaryRefreshAttemptFile is the on-disk re-exec loop breaker.
	chainBoundaryRefreshAttemptFile = loopchain.AttemptFile
	// chainRebuildFn is the seam over the sanctioned rebuild recipe.
	chainRebuildFn = defaultChainRebuild
	// chainReExecArgvFn resolves the argv to re-exec — just os.Args.
	chainReExecArgvFn = func() []string { return os.Args }
	// chainReExecFn is the seam over syscall.Exec.
	chainReExecFn = defaultChainReExec
	// chainBoundaryRefreshLogFile is the additive boundary-refresh audit trail.
	chainBoundaryRefreshLogFile = loopchain.LogFile
	// chainBoundaryFleetLaneFn is the "is a sibling fleet lane active" check.
	chainBoundaryFleetLaneFn = defaultChainBoundaryFleetLaneActive
)

// The chain's wire types: the leaf's, under the names the host and its tests
// spell. chainResult is the machine-readable chain summary emitted to stdout
// after the per-batch loopResult documents — the CLI wire contract, whose
// schema (the tags, `boundary_refresh` omitted when nil) has ONE home:
// loopchain.Result.
type (
	chainBoundaryRefreshLogEntry = loopchain.RefreshLogEntry
	chainResult                  = loopchain.Result
)

// --- the git/disk defaults, facaded for the by-name tests ---

func defaultChainBoundaryAhead(projectRoot, runningCommit string) (ahead bool, err error) {
	return loopchain.GitAhead(projectRoot, runningCommit)
}

// defaultChainBoundaryRepinProvenance reads chainRunningCommitFn at call time.
func defaultChainBoundaryRepinProvenance(projectRoot string) (string, phaseintegrity.ProvenanceVerified) {
	return loopchain.GitProvenance(chainRunningCommitFn)(projectRoot)
}

func defaultChainReExecTarget(projectRoot string) (string, error) {
	return loopchain.RebuiltBinary(projectRoot)
}

func defaultChainBoundaryFleetLaneActive(cfg loopConfig) (active bool, err error) {
	return loopchain.FleetLaneActive(cfg.EvolveDir)
}

// --- the two process adapters the leaf takes as REQUIRED deps ---

// defaultChainRebuild runs `make -C go build` (runtime-reference.md) from
// projectRoot so the on-disk binary catches up to HEAD.
func defaultChainRebuild(projectRoot string) error {
	cmd := exec.Command("make", "-C", "go", "build")
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("make -C go build: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// defaultChainReExec replaces the current process image with argv0/argv/envv.
// A successful call never returns; an error means the exec syscall itself
// failed to launch.
func defaultChainReExec(argv0 string, argv, envv []string) error {
	return syscall.Exec(argv0, argv, envv)
}

// --- the refresh seam ---

// wiredRefresher is the ONE loopchain.NewRefresher( site
// (TestChainEngines_OneConstructionSite): every package var is read INSIDE
// its closure, so the tests' swaps between calls are always seen; the
// Center's Flush is the refresh's flush-before-exec (nil-safe).
func wiredRefresher(cfg loopConfig, stderr io.Writer, signals *signalcenter.Center) *loopchain.Refresher {
	deps := loopchain.RefreshDeps{
		RunningCommit: func() string { return chainRunningCommitFn() },
		Ahead:         func(root, commit string) (bool, error) { return chainBoundaryAheadFn(root, commit) },
		LaneActive:    func() (bool, error) { return chainBoundaryFleetLaneFn(cfg) },
		Rebuild:       func(root string) error { return chainRebuildFn(root) },
		ReExecTarget:  func(root string) (string, error) { return chainReExecTargetFn(root) },
		Provenance: func(root string) (string, phaseintegrity.ProvenanceVerified) {
			return chainBoundaryRepinProvenanceFn(root)
		},
		Argv:    func() []string { return chainReExecArgvFn() },
		Environ: os.Environ,
		Flush:   signals.Flush,
		ReExec:  func(argv0 string, argv, envv []string) error { return chainReExecFn(argv0, argv, envv) },
	}
	return loopchain.NewRefresher(loopchain.Roots{ProjectRoot: cfg.ProjectRoot, EvolveDir: cfg.EvolveDir}, deps, stderr,
		loopchain.WithSignals(func() *signalcenter.Center { return signals }),
		loopchain.WithMarkerFiles(chainBoundaryRefreshAttemptFile, chainBoundaryRefreshLogFile))
}

// maybeRefreshChainBoundaryWithSignals runs the boundary refresh for
// boundary `batch` reporting through signals — the production spelling
// (cmd_loop_window.go's prepareIteration and the chain Driver). It is called
// only BETWEEN batches; every failure degrades to refreshed=false and the
// current binary keeps running — never a halt.
func maybeRefreshChainBoundaryWithSignals(cfg loopConfig, batch int, stderr io.Writer, signals *signalcenter.Center) (refreshed bool) {
	return wiredRefresher(cfg, stderr, signals).Refresh(batch)
}

// maybeRefreshChainBoundary is the by-name test facade: the same refresh
// over a throwaway root Center on stderr (the ONE sink topology), so the
// suites that assert a rendered degrade line keep reading it.
//
// Deprecated: production passes the batch Center through
// maybeRefreshChainBoundaryWithSignals.
func maybeRefreshChainBoundary(cfg loopConfig, batch int, stderr io.Writer) (refreshed bool) {
	signals := newRootSignalCenter(cfg.ProjectRoot, cfg.EvolveDir, stderr)
	defer signals.Flush()
	return maybeRefreshChainBoundaryWithSignals(cfg, batch, stderr, signals)
}

// lastChainBoundaryRefreshLogEntry reads the audit trail's LAST entry —
// nil-when-clean (a missing, empty or unparseable file is (nil, nil)).
func lastChainBoundaryRefreshLogEntry(evolveDir string) (*chainBoundaryRefreshLogEntry, error) {
	return loopchain.LastRefreshLogEntry(filepath.Join(evolveDir, chainBoundaryRefreshLogFile))
}

// --- the pure facades the by-name tests keep ---

func loadChainConfig(evolveDir string) policy.ChainConfig {
	return loopchain.LoadChainConfig(evolveDir)
}

func inboxPendingCount(evolveDir string) (int, []string, error) {
	return loopchain.InboxPendingCount(evolveDir)
}

func chainStartDecision(n, maxBatches, inboxPending int, brake bool) (reason string, stop bool) {
	return loopchain.StartDecision(n, maxBatches, inboxPending, brake)
}

func chainContinueDecision(rc int) (reason string, exit int, stop bool) {
	return loopchain.ContinueDecision(rc)
}

// --- the chain root ---

// wiredChain is the ONE loopchain.NewDriver( site: the real batch over the
// SAME config every time, the Center-bearing refresh, the audit trail, the
// fleet width read to record it, and the checkpoint's quota-pause block. The
// chain Center is flushed before EVERY batch: a chained batch can re-exec at
// a wave boundary and that refresh flushes only the batch's own Center, so
// what the chain root emitted at the boundary must already be delivered
// (TestWiredChain_FlushesTheChainCenterBeforeEveryBatch; F4 is the
// one-Center-per-process fix).
func wiredChain(cfg loopConfig, cc policy.ChainConfig, stdin io.Reader, stdout, stderr io.Writer, signals *signalcenter.Center) *loopchain.Driver {
	deps := loopchain.DriverDeps{
		Batch:       func() int { signals.Flush(); return runLoopBatchFn(cfg, stdin, stdout, stderr) },
		Refresh:     func(batch int) bool { return maybeRefreshChainBoundaryWithSignals(cfg, batch, stderr, signals) },
		LastRefresh: func() (*chainBoundaryRefreshLogEntry, error) { return lastChainBoundaryRefreshLogEntry(cfg.EvolveDir) },
		FleetWidth:  func() int { return loadFleetConfig(cfg.EvolveDir).Count },
		QuotaPause: func() (loopchain.QuotaPause, bool) {
			qp, ok := detectQuotaPause(cfg.EvolveDir)
			return loopchain.QuotaPause{Cycle: qp.Cycle, WakeAt: qp.WakeAt, Source: qp.Source}, ok
		},
	}
	return loopchain.NewDriver(loopchain.Roots{ProjectRoot: cfg.ProjectRoot, EvolveDir: cfg.EvolveDir}, cc, deps, stderr,
		loopchain.WithSignals(func() *signalcenter.Center { return signals }))
}

// runLoopChain drives runLoopBatch until a boundary condition stops it. The
// chain builds its own batch-level Signal Center (the ONE sink topology;
// cycle-0 events land in <evolveDir>/signals.ndjson) and flushes it at exit —
// the refresh flushes it again before every re-exec. The Driver's Result IS
// the summary JSON, printed to stdout after the per-batch loopResult
// documents.
func runLoopChain(cfg loopConfig, cc policy.ChainConfig, stdin io.Reader, stdout, stderr io.Writer) int {
	signals := newRootSignalCenter(cfg.ProjectRoot, cfg.EvolveDir, stderr)
	defer signals.Flush()
	res := wiredChain(cfg, cc, stdin, stdout, stderr, signals).Run()
	buf, _ := json.MarshalIndent(res, "", "  ")
	fmt.Fprintln(stdout, string(buf))
	return res.Exit
}
