//go:build integration

package ship

import (
	"context"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type landedWitness struct {
	events        []signalcenter.Event
	statusAtEmit  []string
	readStatusNow func() string
}

func witnessLandings(t *testing.T, l landingLane, opts *Options) *landedWitness {
	t.Helper()
	w := &landedWitness{readStatusNow: func() string {
		status, _ := l.intent(t)["status"].(string)
		return status
	}}
	center := signalcenter.New()
	center.Subscribe(func(e signalcenter.Event) {
		if e.Kind == signalcenter.KindShipLanded {
			w.events = append(w.events, e)
			w.statusAtEmit = append(w.statusAtEmit, w.readStatusNow())
		}
	})
	opts.Signals = center
	return w
}

func TestSettleLanding_EmitsShipLandedOnceTheRealIntentIsComplete(t *testing.T) {
	t.Parallel()
	l := laneReadyToLand(t)
	in := l.preparedIntent(t)
	_, opts, res := l.worktreeShip(execRunner)
	w := witnessLandings(t, l, opts)

	if err := settleLanding(context.Background(), opts, res, in); err != nil {
		t.Fatalf("settleLanding: %v", err)
	}

	if len(w.events) != 1 {
		t.Fatalf("ship.landed events = %d, want exactly 1 for the landing", len(w.events))
	}
	e := w.events[0]
	if e.Module != signalcenter.ModuleShip || e.Severity != signalcenter.SeverityInfo || e.Phase != "ship" || e.Cycle != in.Cycle || e.RunID != in.RunID {
		t.Errorf("event = %+v, want module ship, INFO, phase ship, cycle %d, run %q", e, in.Cycle, in.RunID)
	}
	if e.Fields["commit"] != in.CommitSHA || e.Fields["cycle"] != strconv.Itoa(in.Cycle) || e.Fields["branch"] != in.Branch {
		t.Errorf("fields = %v, want the landed commit %s, cycle %d and branch %s", e.Fields, in.CommitSHA, in.Cycle, in.Branch)
	}
	if w.statusAtEmit[0] != "complete" {
		t.Errorf("the intent said %q when ship.landed was emitted, want complete: the event marks a durable landing", w.statusAtEmit[0])
	}
}

func TestSettleLanding_NoShipLandedWhileTheRealIntentCannotBeMarkedComplete(t *testing.T) {
	t.Parallel()
	skipWhenRoot(t)
	l := laneReadyToLand(t)
	in := l.preparedIntent(t)
	_, opts, res := l.worktreeShip(execRunner)
	w := witnessLandings(t, l, opts)
	l.lockTheLandingDir(t)

	if err := settleLanding(context.Background(), opts, res, in); err != nil {
		t.Fatalf("settleLanding: %v", err)
	}

	if len(w.events) != 0 {
		t.Errorf("ship.landed events = %+v, want none: a landing whose intent is still prepared settles again on a retry, and that retry emits the event once", w.events)
	}
	if !containsLog(*res, "[ship] WARN: ship cannot mark the landing intent complete") {
		t.Errorf("logs %q, want the WARN that the intent stayed prepared", res.Logs)
	}
}
