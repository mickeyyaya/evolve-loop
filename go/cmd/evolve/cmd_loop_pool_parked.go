package main

import (
	"context"
	"fmt"
	"io"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type parkedLanes struct {
	cfg  loopConfig
	warn io.Writer
}

func mainRedParked(p core.PausedScope) bool { return p.Reason == core.MainRedReason }

func (pl parkedLanes) resume(ctx context.Context, paused []core.PausedScope) bool {
	if !slices.ContainsFunc(paused, mainRedParked) {
		return false
	}
	stop, _, err := mainstop.Read(pl.cfg.EvolveDir)
	if err != nil {
		fmt.Fprintf(pl.warn, "[pool] WARN main-red stop: the record could not be read (%v), so the parked lanes resume\n", err)
		return true
	}
	if !stop.Holds() {
		return true
	}
	attr := baseattr.New(gitexec.Git{Dir: pl.cfg.ProjectRoot, Exec: sysexec.DefaultRunner}, nil, 0, nil)
	tip, err := attr.Resolve(ctx, attr.MainRef)
	if err != nil {
		fmt.Fprintf(pl.warn, "[pool] WARN main-red stop: %s could not be resolved (%v), so the parked lanes stay parked\n", attr.MainRef, err)
		return false
	}
	return stop.Resumes(tip)
}
