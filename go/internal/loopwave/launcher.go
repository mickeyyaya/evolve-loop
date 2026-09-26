package loopwave

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

// Launcher builds one wave's production launcher: a fleet.Supervisor over launch,
// wrapped in the freshness gate so the wave and the min-width repair share one gate.
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

// gatedLauncher runs the freshness gate at the last moment before launch,
// because the plan was made earlier and may be stale.
type gatedLauncher struct {
	inner  Launcher
	probe  fleet.FreshnessProbeFn
	refill fleet.RefillFn
	warn   io.Writer
	wave   int
	e      *Engine
}

// Run launches the specs that stay fresh. When none do, it launches nothing and
// emits one LOOP_WAVE_ALL_LANES_STALE: a shorter wave, never a doomed lane.
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

// probe resolves a task id's freshness from the inbox lifecycle at dispatch time.
// An id with no lifecycle evidence is fresh, because not every planned id is inbox-backed.
func (e *Engine) probe() fleet.FreshnessProbeFn {
	opts := inboxmover.Options{ProjectRoot: e.roots.ProjectRoot, Stderr: io.Discard, Signals: e.center()}
	return func(taskID string) fleet.TaskFreshness {
		ds := inboxmover.ResolveDispatchState(opts, taskID)
		switch ds.State {
		case inboxmover.StatePending:
			for _, dep := range ds.Deps {
				switch inboxmover.ResolveDispatchState(opts, dep).State {
				case inboxmover.StatePending, inboxmover.StateProcessing, inboxmover.StateRetry:
					return fleet.TaskFreshness{Fresh: false, Reason: "deps unmet: needs " + dep}
				}
			}
			return fleet.TaskFreshness{Fresh: true}
		case inboxmover.StateUnknown:
			return fleet.TaskFreshness{Fresh: true}
		default:
			reason := "consumed: " + ds.State
			if ds.Detail != "" {
				reason += " " + ds.Detail
			}
			return fleet.TaskFreshness{Fresh: false, Reason: reason}
		}
	}
}

// refill fills a freed slot with the highest-weight pending todo the wave does not
// own. It picks by weight alone and does not re-check file-disjointness with kept lanes.
func (e *Engine) refill() fleet.RefillFn {
	evolveDir := paths.EvolveDirOf(e.roots.ProjectRoot)
	return func(exclude map[string]bool) (fleet.CycleSpec, bool) {
		backlog := triagecap.ReadInboxBacklog(evolveDir, e.ports.Protected)
		sort.SliceStable(backlog, func(i, j int) bool { return backlog[i].Weight > backlog[j].Weight })
		for _, c := range backlog {
			if exclude[c.ID] {
				continue
			}
			return fleet.CycleSpec{
				Scope: []string{c.ID},
				Env:   map[string]string{ipcenv.FleetScopeKey: c.ID},
			}, true
		}
		return fleet.CycleSpec{}, false
	}
}
