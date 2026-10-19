package loopwave

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

func (e *Engine) Launcher(wave, concurrency int, launch fleet.LaunchFn) Launcher {
	return gatedLauncher{
		inner:  &fleet.Supervisor{Concurrency: concurrency, Launch: launch},
		probe:  e.probe(),
		refill: e.refill(),
		warn:   e.stderr,
		wave:   wave,
		e:      e,
	}
}

type gatedLauncher struct {
	inner  Launcher
	probe  fleet.FreshnessProbeFn
	refill fleet.RefillFn
	warn   io.Writer
	wave   int
	e      *Engine
}

func (l gatedLauncher) Run(ctx context.Context, specs []fleet.CycleSpec) []fleet.Result {
	kept, skipped := fleet.FreshenSpecs(specs, l.probe, l.refill, l.warn)
	if len(kept) == 0 {
		l.e.warn("gatedLauncher.Run", l.wave, CodeWaveAllLanesStale,
			fmt.Sprintf("freshness gate: all %d planned lane(s) stale (%d skip(s)), nothing to launch", len(specs), len(skipped)),
			map[string]string{"planned": strconv.Itoa(len(specs)), "skipped": strconv.Itoa(len(skipped))})
		return nil
	}
	return l.inner.Run(ctx, kept)
}

func (e *Engine) lifecycle() inboxmover.Options {
	return inboxmover.Options{ProjectRoot: e.roots.ProjectRoot, InboxDir: filepath.Join(e.roots.EvolveDir, "inbox"), LedgerPath: filepath.Join(e.roots.EvolveDir, "ledger.jsonl"), Stderr: io.Discard, Signals: e.center()}
}

func (e *Engine) probe() fleet.FreshnessProbeFn {
	lifecycle := e.lifecycle()
	return func(taskID string) fleet.TaskFreshness {
		d := inboxmover.ResolveDispatchability(lifecycle, taskID)
		return fleet.TaskFreshness{Fresh: d.Dispatchable, Reason: d.Reason}
	}
}

func (e *Engine) refill() fleet.RefillFn {
	return func(exclude map[string]bool) (fleet.CycleSpec, bool) {
		for _, c := range triagecap.ReadInboxBacklog(e.roots.EvolveDir, e.ports.Protected) {
			if !exclude[c.ID] {
				return fleet.CycleSpec{Scope: []string{c.ID}, Env: map[string]string{ipcenv.FleetScopeKey: c.ID}}, true
			}
		}
		return fleet.CycleSpec{}, false
	}
}
