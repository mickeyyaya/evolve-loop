package loopwave

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func (e *Engine) Freshen() func(fleet.CycleSpec) (fleet.CycleSpec, bool) {
	probe := e.probe()
	noRefill := func(map[string]bool) (fleet.CycleSpec, bool) { return fleet.CycleSpec{}, false }
	return func(spec fleet.CycleSpec) (fleet.CycleSpec, bool) {
		kept, _ := fleet.FreshenSpecs([]fleet.CycleSpec{spec}, probe, noRefill, e.stderr)
		if len(kept) == 0 {
			return fleet.CycleSpec{}, false
		}
		return kept[0], true
	}
}

type InboxClaims struct {
	opts inboxmover.Options
}

func (e *Engine) Claims() InboxClaims { return InboxClaims{opts: e.lifecycle()} }

func (c InboxClaims) Release(todo, reason string) error {
	_, err := inboxmover.ReleaseClaim(c.opts, todo, reason)
	return err
}

func (c InboxClaims) Park(todo, reason string) error {
	_, err := inboxmover.RouteConsole(c.opts, todo, reason, 0)
	return err
}
