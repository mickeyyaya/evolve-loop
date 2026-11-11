package main

import (
	"context"
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
)

const (
	remoteSweepBudget = 5 * time.Minute
	gcRemoteName      = "origin"
)

func (r gcRun) remoteSweep(projectRoot, evolveDir string, pol gc.Policy) bool {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.ctx), remoteSweepBudget)
	defer cancel()
	stage := gc.RemoteStage(pol.RemoteStage())
	if r.dryRun {
		stage = gc.RemoteStageShadow
	}
	opts := gc.RemoteSweepOptions{
		ProjectRoot: projectRoot,
		EvolveDir:   evolveDir,
		Remote:      gcRemoteName,
		Stage:       stage,
		Exec:        r.exec,
		API:         gc.GhRemoteAPI{Exec: r.exec, Dir: projectRoot},
		Log:         r.stdout,
	}
	manifest, err := gc.PlanRemoteSweep(ctx, opts)
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: WARN: remote branch sweep skipped: %v\n", err)
		return false
	}
	planned := gcRemoteDeletes(manifest)
	r.summary("%d remote branch(es) would be deleted at stage %s (%d kept)", "remote branch sweep — %d branch(es) to delete at stage %s (%d kept)", planned, stage, len(manifest.Items)-planned)
	if err := gc.ApplyRemoteSweep(ctx, opts, manifest); err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: remote branch sweep partial: %v\n", err)
		return true
	}
	return false
}

func gcRemoteDeletes(m gc.RemoteManifest) int {
	n := 0
	for _, it := range m.Items {
		if it.Action == gc.RemoteActionDelete {
			n++
		}
	}
	return n
}
