package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// A true halt means the result was already emitted with exitCode.
func prepareFreshBatch(
	ctx context.Context,
	cfg loopConfig,
	deps orchDeps,
	lr *loopResult,
	stdout, stderr io.Writer,
) (exitCode int, halt bool) {
	// A stale binary must not run recovery logic; a successful refresh re-execs
	// and never returns.
	bootBinaryRefreshFn(cfg, stderr)
	if br := bootRecoverFn(ctx, cfg, deps.Ledger, stderr); br.HaltSelfSHA {
		lr.StopReason = "self_sha_boot_halt"
		lr.emit(stdout)
		return 2, true
	}

	if !cfg.ForceFresh {
		cs, csErr := deps.Storage.ReadCycleState(context.Background())
		last, _ := readLastCycleNumber(context.Background(), deps.Storage)
		if csErr != nil || unfinishedCycle(cs, last) {
			if csErr == nil && cs.WorkspacePath != "" {
				if lease, ok, _ := runlease.Read(cs.WorkspacePath); ok && runlease.OwnerLive(lease, time.Now(), 0, pidAlive) {
					fmt.Fprintf(stderr, "[loop] cycle %d is owned by a LIVE run (pid %d, lease heartbeat fresh) — another evolve loop is already running it.\n", cs.CycleID, lease.OwnerPID)
					fmt.Fprintln(stderr, "[loop]   • continue/attach:  evolve loop --resume")
					fmt.Fprintln(stderr, "[loop]   • or let it finish — do NOT `evolve cycle reset` or `pkill` a live run (Ctrl-C lets it checkpoint).")
					lr.StopReason = "owned_by_live_run"
					lr.emit(stdout)
					return 2, true
				}
			}
			if csErr != nil {
				fmt.Fprintf(stderr, "[loop] cycle-state.json is unreadable (%v) — treating as an unfinished cycle to avoid clobbering history.\n", csErr)
			} else {
				fmt.Fprintf(stderr, "[loop] unfinished cycle %d detected at phase %q (lastCycleNumber=%d).\n", cs.CycleID, cs.Phase, last)
			}
			fmt.Fprintln(stderr, "[loop]   • continue it:    evolve loop --resume")
			fmt.Fprintln(stderr, "[loop]   • seal & move on: evolve cycle reset   (archives the cycle for analysis, advances the number)")
			fmt.Fprintln(stderr, "[loop]   (or pass --force-fresh to start fresh and overwrite — history NOT sealed)")
			lr.StopReason = "unfinished_cycle"
			lr.emit(stdout)
			return 2, true
		}
	}

	if bootGateHalts(ctx, cfg, deps, lr, stdout, stderr) {
		return 2, true
	}

	// Drains worktrees a prior crashed batch left finalized; this batch's own
	// finalized worktrees are handled by the end sweep.
	gcHookFn(cfg, filepath.Join(gcManifestDir(cfg.EvolveDir), "pre-batch"), stderr)
	return 0, false
}

func bootGateHalts(ctx context.Context, cfg loopConfig, deps orchDeps, lr *loopResult, stdout, stderr io.Writer) bool {
	if bootCLIUpdateHalts(ctx, cfg, deps, lr, stdout, stderr) {
		return true
	}
	if loopPreflightHalts(cfg, stderr) {
		lr.StopReason = "preflight_failed"
		lr.emit(stdout)
		return true
	}
	return false
}
