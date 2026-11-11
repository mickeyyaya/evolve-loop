package main

import (
	"context"
	"fmt"
	"io"
	"syscall"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func sweepSealedCycleGC(cfg loopConfig, cycle int, stderr io.Writer) {
	gcPol, mode, ok := loopGCMode(cfg, stderr)
	if !ok || mode != "enforce" || cfg.ProjectRoot == "" {
		return
	}
	host := gcProcesses()
	opts := worktreeGCOptions(cfg.ProjectRoot, cfg.EvolveDir, gcPol.Worktrees, stderr)
	opts.PidAlive = host.alive
	opts.LeaseTTL = runlease.DefaultTTL
	rep := gc.SweepSealedCycle(context.Background(), opts, cycle, func(pid int) error { return host.signal(pid, syscall.SIGTERM) })
	fmt.Fprintf(stderr, "[gc] sealed cycle %d: %s\n", cycle, rep.Summary())
}
