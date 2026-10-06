package main

import (
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
)

// maintainBatchState's failures are reported, never fatal, so batch-startup
// maintenance cannot block a cycle.
func maintainBatchState(cfg loopConfig, autoPrune bool, stderr io.Writer) {
	statePath := filepath.Join(cfg.EvolveDir, "state.json")
	if autoPrune {
		pruneBatchState(statePath, stderr)
	}
	if cfg.Reset {
		resetBatchState(cfg, statePath, stderr)
	}
}

type expiryOutcome struct {
	failed       failurelog.PruneResult
	failedErr    error
	carryover    failurelog.PruneResult
	carryoverErr error
}

func pruneExpiredState(statePath string, now time.Time) expiryOutcome {
	failed, failedErr := failurelog.PruneExpired(statePath, now)
	carryover, carryoverErr := failurelog.PruneExpiredCarryoverTodos(statePath, now)
	return expiryOutcome{failed: failed, failedErr: failedErr, carryover: carryover, carryoverErr: carryoverErr}
}

func pruneBatchState(statePath string, stderr io.Writer) {
	now := time.Now().UTC()
	out := pruneExpiredState(statePath, now)
	if out.failedErr != nil {
		fmt.Fprintf(stderr, "[loop] auto-prune: %v\n", out.failedErr)
	} else if out.failed.Removed > 0 {
		fmt.Fprintf(stderr, "[loop] auto-prune: removed %d expired failedApproaches (%d→%d)\n", out.failed.Removed, out.failed.Before, out.failed.After)
	}
	if out.carryoverErr != nil {
		fmt.Fprintf(stderr, "[loop] auto-prune: carryover: %v\n", out.carryoverErr)
	} else if out.carryover.Removed > 0 {
		fmt.Fprintf(stderr, "[loop] auto-prune: removed %d expired carryoverTodos (%d→%d)\n", out.carryover.Removed, out.carryover.Before, out.carryover.After)
	}
	if stamped, err := failurelog.BackfillLegacyCarryoverExpiry(statePath, failurelog.DefaultCarryoverBackfillTTL, now); err != nil {
		fmt.Fprintf(stderr, "[loop] auto-prune: carryover backfill: %v\n", err)
	} else if stamped > 0 {
		fmt.Fprintf(stderr, "[loop] auto-prune: backfilled expiresAt on %d legacy carryoverTodos\n", stamped)
	}
	if inc, err := failurelog.IncrementCarryoverUnpicked(statePath); err != nil {
		fmt.Fprintf(stderr, "[loop] auto-prune: carryover unpicked: %v\n", err)
	} else if inc > 0 {
		fmt.Fprintf(stderr, "[loop] auto-prune: incremented cycles_unpicked on %d carryoverTodos\n", inc)
	}
}

var resetClasses = []failurelog.Classification{
	failurelog.InfrastructureSystemic,
	failurelog.InfrastructureTransient,
	failurelog.ShipGateConfig,
}

type resetOutcome struct {
	pruned   failurelog.PruneResult
	pruneErr error
	acked    bool
	ackErr   error
}

func resetFailures(statePath, evolveDir, fingerprint string) resetOutcome {
	pr, pruneErr := failurelog.PruneByClassification(statePath, resetClasses)
	out := resetOutcome{pruned: pr, pruneErr: pruneErr}
	if fingerprint == "" {
		return out
	}
	out.ackErr = core.AppendResolvedFingerprint(evolveDir, fingerprint, "operator-reset", time.Now().UTC())
	out.acked = out.ackErr == nil
	return out
}

func resetBatchState(cfg loopConfig, statePath string, stderr io.Writer) {
	out := resetFailures(statePath, cfg.EvolveDir, cfg.Fingerprint)
	if out.pruneErr != nil {
		fmt.Fprintf(stderr, "[loop] --reset: %v\n", out.pruneErr)
	} else if out.pruned.Removed > 0 {
		fmt.Fprintf(stderr, "[loop] --reset: pruned %d failedApproaches (infrastructure-{systemic,transient} + ship-gate-config) (%d→%d)\n", out.pruned.Removed, out.pruned.Before, out.pruned.After)
	}
	if out.ackErr != nil {
		fmt.Fprintf(stderr, "[loop] --reset --fingerprint: %v\n", out.ackErr)
	} else if out.acked {
		fmt.Fprintf(stderr, "[loop] --reset --fingerprint: acknowledged %q in resolved-fingerprints.json — blocker-breaker will exclude it going forward\n", cfg.Fingerprint)
	}
}
