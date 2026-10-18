package ship

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestLanding_EmitsShipLandedAtIntentComplete(t *testing.T) {
	t.Parallel()
	opts, _, _ := pinOptions(t, ClassCycle)
	opts.RunID = "run-7"
	var landed []signalcenter.Event
	var statusAtEmit []landing.IntentStatus
	center := signalcenter.New()
	center.Subscribe(func(e signalcenter.Event) {
		if e.Kind != signalcenter.KindShipLanded {
			return
		}
		landed = append(landed, e)
		in, _, _ := landing.ReadIntent(intentPath(opts, 7))
		statusAtEmit = append(statusAtEmit, in.Status)
	})
	opts.Signals = center
	wt := filepath.Join(opts.ProjectRoot, "wt")
	r := newArgvRecorder(opts)
	scriptGreenLanding(r, wt)
	s := newWorktreeShip(context.Background(), opts, &RunResult{}, pinBranch, wt)
	s.cycleBranch = pinCycleBranch

	if err := s.land(""); err != nil {
		t.Fatal(err)
	}

	if len(landed) != 1 {
		t.Fatalf("ship.landed events = %d, want exactly 1 for the landing", len(landed))
	}
	e := landed[0]
	if e.Module != signalcenter.ModuleShip || e.Severity != signalcenter.SeverityInfo || e.Phase != phaseName || e.Cycle != 7 || e.RunID != "run-7" || e.Code != "" {
		t.Errorf("event = %+v, want an INFO ship.landed of the ship phase for cycle 7 and run run-7, with no code", e)
	}
	if e.Fields["commit"] != pinHead || e.Fields["cycle"] != "7" || e.Fields["branch"] != pinBranch {
		t.Errorf("fields = %v, want the landed commit %s, cycle 7 and branch %s", e.Fields, pinHead, pinBranch)
	}
	if statusAtEmit[0] != landing.IntentComplete {
		t.Errorf("the intent said %q when ship.landed was emitted, want %q: the event marks a durable landing", statusAtEmit[0], landing.IntentComplete)
	}
}
