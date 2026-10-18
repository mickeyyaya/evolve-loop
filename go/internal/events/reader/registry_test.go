package reader

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/filter"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestRegistry_NewKindsAndCodesAreRegistered(t *testing.T) {
	t.Parallel()
	cat := filter.RegisteredCatalog()

	for _, kind := range []signalcenter.Kind{"phase.dispatched", "ship.landed", "loop.started", "loop.exit", "loop.lost"} {
		if !slices.Contains(cat.Kinds, kind) {
			t.Errorf("kind %q is not in the filter vocabulary, so a route or --until cannot name it", kind)
		}
	}
	if !slices.Contains(cat.Codes, signalcenter.Code("LOOP_LOST")) {
		t.Errorf("code LOOP_LOST is not in the filter vocabulary")
	}
	if owner, ok := signalcenter.IsRegistered("LOOP_LOST"); !ok || owner != signalcenter.ModuleLoop {
		t.Errorf("LOOP_LOST owner = %q (registered %v), want the loop module", owner, ok)
	}
}

func TestRegistry_TheLastWillKindsAreTheRegisteredOnes(t *testing.T) {
	t.Parallel()

	if kindLoopStarted != signalcenter.KindLoopStarted || kindLoopExit != signalcenter.KindLoopExit || kindLoopLost != signalcenter.KindLoopLost {
		t.Errorf("the last will reads %q, %q and %q, which are not the registered loop lifecycle kinds", kindLoopStarted, kindLoopExit, kindLoopLost)
	}
}
