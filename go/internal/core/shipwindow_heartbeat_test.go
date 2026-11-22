package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindow"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindowtest"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func windowFor(clock *shipwindowtest.Clock, beats *shipwindowtest.Beats, pid int) shipwindow.Options {
	return shipwindow.Options{Now: clock.Now, Ticker: beats.Ticker, Alive: func(int) bool { return true }, OwnerPID: pid, Poll: time.Millisecond}
}

func TestShipWindow_TwoLanesWithSectionsLongerThanTTLNeverOverlap(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	clock, beatsA, beatsB, probeB := shipwindowtest.NewClock(), shipwindowtest.NewBeats(), shipwindowtest.NewBeats(), shipwindowtest.NewProbe()
	laneA := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithShipWindowOptions(windowFor(clock, beatsA, 101)))
	optsB := windowFor(clock, beatsB, 202)
	optsB.Alive = probeB.Alive
	laneB := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithShipWindowOptions(optsB))
	laneA.holdShipWindow(context.Background(), root, 1, "run-1")

	enteredB := make(chan struct{})
	probeB.Drain()
	go func() {
		laneB.holdShipWindow(context.Background(), root, 2, "run-2")
		close(enteredB)
	}()

	lanes := shipwindowtest.Lanes{Clock: clock, Holder: beatsA, Waiter: probeB}
	for range 4 {
		lanes.Step(t, 5*time.Minute)
	}
	probeB.Drain()
	probeB.WaitPolls(t, 3)
	select {
	case <-enteredB:
		t.Fatal("lane B took the ship window while lane A was still 20 minutes into its recovery")
	default:
	}
	laneA.releaseShipWindow()
	select {
	case <-enteredB:
	case <-time.After(30 * time.Second):
		t.Fatal("lane B never took the ship window after lane A released")
	}
	laneB.releaseShipWindow()
	if _, err := os.Stat(shipwindow.PathIn(filepath.Join(root, ".evolve"))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the lease outlived both lanes (stat err=%v)", err)
	}
}

func TestHoldShipWindow_ALostLeaseSignalsTheHolderAndTheReShipSeesIt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	clock, beats := shipwindowtest.NewClock(), shipwindowtest.NewBeats()
	center := signalcenter.New()
	emitted := make(chan signalcenter.Event, 4)
	center.Subscribe(func(e signalcenter.Event) { emitted <- e })
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(center), WithShipWindowOptions(windowFor(clock, beats, 101)))
	o.holdShipWindow(context.Background(), root, 9, "run-9")
	defer o.releaseShipWindow()

	clock.Advance(shipwindow.DefaultMaxHold)
	beats.Tick(t, clock.Now())
	var e signalcenter.Event
	select {
	case e = <-emitted:
	case <-time.After(30 * time.Second):
		t.Fatal("the holder of the recovery was not told that its lease passed the hard cap")
	}

	if e.Code != CodeShipWindowLeaseBroken || e.Cycle != 9 || e.RunID != "run-9" || e.Fields["role"] != "holder" {
		t.Fatalf("signal = %+v, want %s from cycle 9 with role=holder", e, CodeShipWindowLeaseBroken)
	}
	if shipwindow.HeldErr(o.shipWindowContext(context.Background())) == nil {
		t.Fatal("the re-ship context carries a lease that is already lost without saying so")
	}
}
