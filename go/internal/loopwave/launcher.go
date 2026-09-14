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

// Launcher builds the production launcher for one wave: a fleet.Supervisor
// over launch (the same exec launcher `evolve fleet` uses, so lanes inherit
// EVOLVE_FLEET=1 + EVOLVE_FLEET_SCOPE) wrapped in the dispatch freshness gate
// (cycle 767): immediately before launch every spec's scope ids are
// re-resolved against the CURRENT inbox lifecycle + deps, stale ids are
// skipped with a logged reason and freed slots are refilled from the pending
// backlog — a lane slot is never burned on known-dead work. Decorating here
// gates BOTH the wave path and the min-width repair at one seam; the wave
// index rides into the gate so its one WARN is wave-indexed.
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

// gatedLauncher decorates a Launcher with fleet.FreshenSpecs so the gate runs
// at the last moment before lanes launch (planning happened earlier and may
// be stale — the postmortem's whole failure class).
type gatedLauncher struct {
	inner  Launcher
	probe  fleet.FreshnessProbeFn
	refill fleet.RefillFn
	warn   io.Writer
	wave   int
	e      *Engine
}

// Run freshens the specs and launches the kept ones. A whole wave stale with
// the backlog exhausted launches nothing — that IS the fix (a shorter wave,
// never a doomed lane) — and reports ONE LOOP_WAVE_ALL_LANES_STALE; the
// per-lane skips are fleet's own WARN lines on the warn writer.
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

// probe re-resolves one task id against the inbox lifecycle at dispatch
// time. Pending → fresh unless a declared dep is still undone
// (pending/processing/retry); any consumed lifecycle state → stale with the
// state as reason; no lifecycle evidence at all → fresh (fail-open: not every
// planned id is inbox-backed, and a missing file must never false-skip a
// lane — Q-W6, acs/cycle1180).
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

// refill pulls the highest-weight pending inbox todo not already owned by
// this wave into a freed slot, shaped exactly like a planned lane spec
// (Scope + EVOLVE_FLEET_SCOPE; EVOLVE_FLEET is forced by the supervisor).
// Minimal: picks by weight only, no re-check of file-disjointness against the
// kept lanes. It reads <ProjectRoot>/.evolve, as the production launcher
// always did (Q-W8 — not EvolveDir).
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
