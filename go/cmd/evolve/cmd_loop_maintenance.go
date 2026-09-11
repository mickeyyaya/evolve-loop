package main

import (
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
)

// maintainBatchState applies the bounded state maintenance requested at batch
// startup. Failures are reported and remain non-fatal, matching the original
// dispatcher contract.
func maintainBatchState(cfg loopConfig, autoPrune bool, stderr io.Writer) {
	statePath := filepath.Join(cfg.EvolveDir, "state.json")
	if autoPrune {
		pruneBatchState(statePath, stderr)
	}
	if cfg.Reset {
		resetBatchState(cfg, statePath, stderr)
	}
}

func pruneBatchState(statePath string, stderr io.Writer) {
	if pr, err := failurelog.PruneExpired(statePath, time.Now().UTC()); err != nil {
		fmt.Fprintf(stderr, "[loop] auto-prune: %v\n", err)
	} else if pr.Removed > 0 {
		fmt.Fprintf(stderr, "[loop] auto-prune: removed %d expired failedApproaches (%d→%d)\n", pr.Removed, pr.Before, pr.After)
	}
	if pr, err := failurelog.PruneExpiredCarryoverTodos(statePath, time.Now().UTC()); err != nil {
		fmt.Fprintf(stderr, "[loop] auto-prune: carryover: %v\n", err)
	} else if pr.Removed > 0 {
		fmt.Fprintf(stderr, "[loop] auto-prune: removed %d expired carryoverTodos (%d→%d)\n", pr.Removed, pr.Before, pr.After)
	}
	if stamped, err := failurelog.BackfillLegacyCarryoverExpiry(statePath, failurelog.DefaultCarryoverBackfillTTL, time.Now().UTC()); err != nil {
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

func resetBatchState(cfg loopConfig, statePath string, stderr io.Writer) {
	resetClasses := []failurelog.Classification{
		failurelog.InfrastructureSystemic,
		failurelog.InfrastructureTransient,
		failurelog.ShipGateConfig,
	}
	if pr, err := failurelog.PruneByClassification(statePath, resetClasses); err != nil {
		fmt.Fprintf(stderr, "[loop] --reset: %v\n", err)
	} else if pr.Removed > 0 {
		fmt.Fprintf(stderr, "[loop] --reset: pruned %d failedApproaches (infrastructure-{systemic,transient} + ship-gate-config) (%d→%d)\n", pr.Removed, pr.Before, pr.After)
	}
	if cfg.Fingerprint == "" {
		return
	}
	if err := core.AppendResolvedFingerprint(cfg.EvolveDir, cfg.Fingerprint, "operator-reset", time.Now().UTC()); err != nil {
		fmt.Fprintf(stderr, "[loop] --reset --fingerprint: %v\n", err)
	} else {
		fmt.Fprintf(stderr, "[loop] --reset --fingerprint: acknowledged %q in resolved-fingerprints.json — blocker-breaker will exclude it going forward\n", cfg.Fingerprint)
	}
}
