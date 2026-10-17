package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/loopchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/version"
)

const chainBrakeFile = paths.LoopStopFile

var (
	runLoopBatchFn                  = runLoopBatch
	chainBoundaryAheadFn            = defaultChainBoundaryAhead
	chainRunningCommitFn            = version.Commit
	chainBoundaryRepinProvenanceFn  = defaultChainBoundaryRepinProvenance
	chainReExecTargetFn             = defaultChainReExecTarget
	chainBoundaryRefreshAttemptFile = loopchain.AttemptFile
	chainRebuildFn                  = defaultChainRebuild
	chainReExecArgvFn               = func() []string { return os.Args }
	chainReExecFn                   = defaultChainReExec
	chainBoundaryRefreshLogFile     = loopchain.LogFile
	chainBoundaryFleetLaneFn        = defaultChainBoundaryFleetLaneActive
)

type (
	chainBoundaryRefreshLogEntry = loopchain.RefreshLogEntry
	chainResult                  = loopchain.Result
)

func defaultChainBoundaryAhead(projectRoot, runningCommit string) (ahead bool, err error) {
	return loopchain.GitAhead(projectRoot, runningCommit)
}

func defaultChainBoundaryRepinProvenance(projectRoot string) (string, phaseintegrity.ProvenanceVerified) {
	return loopchain.GitProvenance(chainRunningCommitFn)(projectRoot)
}

func defaultChainReExecTarget(projectRoot string) (string, error) {
	return loopchain.RebuiltBinary(projectRoot)
}

func defaultChainBoundaryFleetLaneActive(cfg loopConfig) (active bool, err error) {
	return loopchain.FleetLaneActive(cfg.EvolveDir)
}

func defaultChainRebuild(projectRoot string) error {
	cmd := sysexec.Command(context.Background(), "make", "-C", "go", "build")
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("make -C go build: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func defaultChainReExec(argv0 string, argv, envv []string) error {
	return syscall.Exec(argv0, argv, envv)
}

func wiredRefresher(cfg loopConfig, stderr io.Writer, signals *signalcenter.Center, extra ...loopchain.Option) *loopchain.Refresher {
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
	opts := append([]loopchain.Option{
		loopchain.WithSignals(func() *signalcenter.Center { return signals }),
		loopchain.WithMarkerFiles(chainBoundaryRefreshAttemptFile, chainBoundaryRefreshLogFile),
	}, extra...)
	return loopchain.NewRefresher(loopchain.Roots{ProjectRoot: cfg.ProjectRoot, EvolveDir: cfg.EvolveDir}, deps, stderr, opts...)
}

// Deprecated: production refreshes through wiredRefresher with its own Center.
func maybeRefreshChainBoundary(cfg loopConfig, batch int, stderr io.Writer) (refreshed bool) {
	signals := newRootSignalCenter(cfg.ProjectRoot, cfg.EvolveDir, stderr)
	defer signals.Flush()
	return wiredRefresher(cfg, stderr, signals).Refresh(context.Background(), batch)
}

func takeReexecHandoff(evolveDir string, stderr io.Writer) (int, bool) {
	done, handedOff, err := loopchain.TakeHandoff(filepath.Join(evolveDir, chainBoundaryRefreshAttemptFile), loopchain.Claim{PID: os.Getpid(), Commit: chainRunningCommitFn(), At: time.Now()})
	if err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: boundary re-exec handoff not honoured (%v) — starting at wave 0 with the full budget\n", err)
	}
	return done, handedOff
}

func lastChainBoundaryRefreshLogEntry(evolveDir string) (*chainBoundaryRefreshLogEntry, error) {
	return loopchain.LastRefreshLogEntry(filepath.Join(evolveDir, chainBoundaryRefreshLogFile))
}

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

func wiredChain(cfg loopConfig, cc policy.ChainConfig, stdin io.Reader, stdout, stderr io.Writer, signals *signalcenter.Center) *loopchain.Driver {
	next := cfg
	deps := loopchain.DriverDeps{
		Batch: func() int {
			signals.Flush()
			batchCfg := next
			next.ResumeWaves, next.HandedOff = 0, false
			return runLoopBatchFn(batchCfg, stdin, stdout, stderr)
		},
		Refresh:     func(batch int) bool { return wiredRefresher(cfg, stderr, signals).Refresh(context.Background(), batch) },
		LastRefresh: func() (*chainBoundaryRefreshLogEntry, error) { return lastChainBoundaryRefreshLogEntry(cfg.EvolveDir) },
		FleetWidth:  func() int { return loadFleetConfig(cfg.EvolveDir, stderr, signals).Count },
		QuotaPause: func() (loopchain.QuotaPause, bool) {
			qp, ok := detectQuotaPause(cfg.EvolveDir)
			return loopchain.QuotaPause{Cycle: qp.Cycle, WakeAt: qp.WakeAt, Source: qp.Source}, ok
		},
	}
	return loopchain.NewDriver(loopchain.Roots{ProjectRoot: cfg.ProjectRoot, EvolveDir: cfg.EvolveDir}, cc, deps, stderr,
		loopchain.WithSignals(func() *signalcenter.Center { return signals }))
}

func runLoopChain(cfg loopConfig, cc policy.ChainConfig, stdin io.Reader, stdout, stderr io.Writer) int {
	signals := newRootSignalCenter(cfg.ProjectRoot, cfg.EvolveDir, stderr)
	defer signals.Flush()
	res := wiredChain(cfg, cc, stdin, stdout, stderr, signals).Run()
	buf, _ := json.MarshalIndent(res, "", "  ")
	fmt.Fprintln(stdout, string(buf))
	return res.Exit
}
