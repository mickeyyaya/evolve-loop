package main

import (
	"context"
	"errors"
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
	opts := gc.BranchSweepOptions{
		ProjectRoot: projectRoot,
		EvolveDir:   evolveDir,
		Remote:      gcRemoteName,
		Policy:      pol,
		Stage:       stage,
		Exec:        r.exec,
		API:         gc.GhRemoteAPI{Exec: r.exec, Dir: projectRoot},
		Log:         r.stdout,
	}
	manifest, err := gc.PlanBranchSweep(ctx, opts)
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: WARN: remote branch sweep skipped: %v\n", err)
		return false
	}
	planned, kept := manifest.RemoteCounts()
	r.summary("%d remote branch(es) would be deleted at stage %s (%d kept)", "remote branch sweep — %d branch(es) to delete at stage %s (%d kept)", planned, stage, kept)
	if err := gc.ApplyBranchSweep(ctx, opts, manifest); err != nil {
		return r.reportBranchSweepErrors(err)
	}
	return false
}

func (r gcRun) reportBranchSweepErrors(err error) bool {
	var hard []error
	for _, e := range flattenErrors(err) {
		if errors.Is(e, gc.ErrRemoteDeleteFailed) {
			fmt.Fprintf(r.stderr, "evolve gc: WARN: %v\n", e)
			continue
		}
		hard = append(hard, e)
	}
	if len(hard) > 0 {
		fmt.Fprintf(r.stderr, "evolve gc: remote branch sweep partial: %v\n", errors.Join(hard...))
		return true
	}
	return false
}

func flattenErrors(err error) []error {
	if uw, ok := err.(interface{ Unwrap() []error }); ok {
		var out []error
		for _, sub := range uw.Unwrap() {
			out = append(out, flattenErrors(sub)...)
		}
		return out
	}
	return []error{err}
}
