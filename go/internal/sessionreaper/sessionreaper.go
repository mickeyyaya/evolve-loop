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

const DefaultReapTimeout = 15 * time.Second

const reapedSuffix = sessionrecord.ReapedSuffix

type Options struct {
	Now      func() time.Time
	LeaseTTL time.Duration
	Kill     swarm.TmuxKiller
}

type OrphanReap struct {
	RunDir string
	Report swarm.ReapRunReport
}

type Report struct {
	LiveRunsSkipped int
	Orphaned        []OrphanReap
}

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
			_ = os.Rename(registryPath, registryPath+reapedSuffix)
		}
		out.Orphaned = append(out.Orphaned, OrphanReap{RunDir: runDir, Report: rep})
	}
	return out, nil
}
