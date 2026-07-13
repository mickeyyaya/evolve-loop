// Package sessionreaper removes tmux sessions orphaned by runs whose lease is
// no longer fresh. It discovers ownership exclusively through each run's
// session registry, so a sweep cannot target sessions belonging to another run.
package sessionreaper

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/sessionrecord"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

// DefaultReapTimeout bounds one orphan sweep so a wedged tmux socket (a
// corrupted server, not the common "no server" case) can never hang its
// caller — loop-boot preflight, soak checks, and the per-cycle GC all share
// this discipline.
const DefaultReapTimeout = 15 * time.Second

// reapedSuffix tombstones a fully-reaped registry. Renaming (rather than
// deleting) preserves the session audit trail, and the existing
// "no registry → skip" walk check makes every later sweep skip the run for
// free — the fix for the unbounded boot re-reap class (cycle-769: 4,388
// sessions re-swept serially on every boot). A relaunched run recreates the
// registry, so it becomes sweepable again. The suffix value lives in the
// sessionrecord SSOT (ReapedSuffix) so the reader that resolves the tombstone
// for attribution (sessionrecord.ReadAllResolving) shares one definition.
const reapedSuffix = sessionrecord.ReapedSuffix

// Options configures an orphan sweep.
type Options struct {
	Now      func() time.Time
	LeaseTTL time.Duration
	Kill     swarm.TmuxKiller
}

// OrphanReap reports the result of reaping one stale run.
type OrphanReap struct {
	RunDir string
	Report swarm.ReapRunReport
}

// Report summarizes an orphan sweep.
type Report struct {
	LiveRunsSkipped int
	Orphaned        []OrphanReap
}

// ReapOrphans walks <evolveDir>/runs for registry-backed runs. Fresh leases
// prove liveness and are never reaped; absent leases are stale by definition.
func ReapOrphans(ctx context.Context, evolveDir string, o Options) (Report, error) {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	kill := o.Kill
	if kill == nil {
		kill = swarm.ExecTmuxKill
	}

	runsDir := filepath.Join(evolveDir, "runs")
	entries, err := os.ReadDir(runsDir)
	if errors.Is(err, os.ErrNotExist) {
		return Report{}, nil
	}
	if err != nil {
		return Report{}, fmt.Errorf("sessionreaper: read runs dir: %w", err)
	}

	var out Report
	for _, entry := range entries {
		runDir := filepath.Join(runsDir, entry.Name())
		info, err := os.Stat(runDir)
		if err != nil || !info.IsDir() {
			continue
		}
		registryPath := sessionrecord.PathIn(runDir)
		if info, err := os.Stat(registryPath); err != nil || !info.Mode().IsRegular() {
			continue
		}

		lease, ok, _ := runlease.Read(runDir)
		if ok && runlease.Fresh(lease, now(), o.LeaseTTL) {
			out.LiveRunsSkipped++
			continue
		}
		rep := swarm.ReapRunSessions(ctx, registryPath, kill)
		if rep.Errors == 0 {
			// Tombstone only a FULLY successful reap; any killer error leaves
			// the registry in place so the next sweep retries this run.
			// Best-effort: a failed rename costs one redundant re-sweep.
			_ = os.Rename(registryPath, registryPath+reapedSuffix)
		}
		out.Orphaned = append(out.Orphaned, OrphanReap{RunDir: runDir, Report: rep})
	}
	return out, nil
}
