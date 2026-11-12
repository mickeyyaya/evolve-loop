package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type parkedLanes struct {
	cfg  loopConfig
	warn io.Writer
	now  time.Time
	emit func(signalcenter.Event)
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
	pl.escalateOverdue()
	attr := baseattr.New(gitexec.Git{Dir: pl.cfg.ProjectRoot, Exec: sysexec.DefaultRunner}, nil, 0, nil)
	tip, err := attr.Resolve(ctx, attr.MainRef)
	if err != nil {
		fmt.Fprintf(pl.warn, "[pool] WARN main-red stop: %s could not be resolved (%v), so the parked lanes stay parked\n", attr.MainRef, err)
		return false
	}
	return stop.Resumes(tip)
}

func (pl parkedLanes) escalateOverdue() {
	gates := policy.LoadOrDefaults(filepath.Join(pl.cfg.EvolveDir, "policy.json"), pl.warn).GatesConfig()
	bound := selfheal.StopBound{EvolveDir: pl.cfg.EvolveDir, Bound: gates.MainRedStopBound, Now: pl.now, Emit: pl.emit}
	if err := bound.Check(); err != nil {
		fmt.Fprintf(pl.warn, "[pool] WARN main-red stop: %v\n", err)
	}
}
