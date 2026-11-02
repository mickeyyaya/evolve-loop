package orchestrator

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestPoolHeld_BelowMinLanesWarnsUnderfilledAtOnce(t *testing.T) {
	t.Parallel()
	events := &eventLog{}
	Pool{MinLanes: 4, Publish: events.publish}.transition(fleet.PoolTransition{Kind: fleet.PoolHeld, Live: 1, Target: 4, Pending: 72})
	if len(events.withCode(CodePoolHeld)) != 1 {
		t.Fatalf("held events = %d, want 1", len(events.withCode(CodePoolHeld)))
	}
	warns := events.withCode(CodePoolUnderfilled)
	if len(warns) != 1 || warns[0].Fields["live"] != "1" || warns[0].Fields["ready"] != "72" {
		t.Fatalf("underfilled = %+v, want one WARN with live 1 and ready 72 at the hold, not at the next lane exit", warns)
	}
}

func TestPoolHeld_AtMinLanesDoesNotWarn(t *testing.T) {
	t.Parallel()
	events := &eventLog{}
	Pool{MinLanes: 2, Publish: events.publish}.transition(fleet.PoolTransition{Kind: fleet.PoolHeld, Live: 2, Target: 4, Pending: 5})
	if n := len(events.withCode(CodePoolUnderfilled)); n != 0 {
		t.Fatalf("underfilled = %d, want 0 at min_lanes", n)
	}
}

func TestPoolHeld_TheCodeIsRegisteredToTheLoopModule(t *testing.T) {
	t.Parallel()
	if m, ok := signalcenter.IsRegistered(CodePoolHeld); !ok || m != signalcenter.ModuleLoop {
		t.Fatalf("IsRegistered(%q) = %q, %v, want the loop module", CodePoolHeld, m, ok)
	}
}
