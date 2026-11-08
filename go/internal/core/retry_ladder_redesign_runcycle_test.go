package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestRunCycle_AFailedLadderRoutesOnceToTheRedesignRung(t *testing.T) {
	t.Parallel()
	events := runLadderCycle(t, policy.RetryLadderEnforce, newLadderProbe(0))
	routes := eventsWithCode(events, CodeLadderRedesign)
	if len(routes) != 1 {
		t.Fatalf("a cycle whose build never passes emitted %d %s events, want exactly 1: %+v", len(routes), CodeLadderRedesign, events)
	}
	route := routes[0]
	if route.Module != signalcenter.ModuleLadder || route.Severity != signalcenter.SeverityWarn || route.Origin != originContractCorrection {
		t.Fatalf("the redesign route = %+v, want a ladder warning from the contract correction path", route)
	}
	if route.Fields["fingerprint"] == "" {
		t.Fatalf("the redesign route names no failed check: %+v", route)
	}
	if n := len(eventsWithCode(events, CodeLadderStopSpec)); n != 1 {
		t.Fatalf("the redesign route follows %d spec stops, want 1: %+v", n, events)
	}
}

func TestRunCycle_APassingBuildNeverRoutesToTheRedesignRung(t *testing.T) {
	t.Parallel()
	events := runLadderCycle(t, policy.RetryLadderEnforce, newLadderProbe(1))
	if n := len(eventsWithCode(events, CodeLadderRedesign)); n != 0 {
		t.Fatalf("a build that passes after one correction emitted %d %s events, want 0: %+v", n, CodeLadderRedesign, events)
	}
	if n := len(eventsWithCode(events, signalcenter.Code(retryladder.SignalStopSpec))); n != 0 {
		t.Fatalf("a build that passes after one correction stopped as spec: %+v", events)
	}
}
