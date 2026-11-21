package ship

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindow"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindowtest"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type pushRecorder struct {
	t      *testing.T
	pushes int
}

func (p *pushRecorder) run(_ context.Context, _, _ string, args []string, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
	if slices.Contains(args, "push") {
		p.pushes++
	}
	return 0, nil
}

func windowOptions(clock *shipwindowtest.Clock, beats *shipwindowtest.Beats, probe *shipwindowtest.Probe, pid int) shipwindow.Options {
	alive := func(int) bool { return true }
	if probe != nil {
		alive = probe.Alive
	}
	return shipwindow.Options{Now: clock.Now, Ticker: beats.Ticker, Alive: alive, OwnerPID: pid, Poll: time.Millisecond}
}

func TestEnterShipWindow_ALostLeaseAbortsTheLandingWithoutAPush(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	clock, beats := shipwindowtest.NewClock(), shipwindowtest.NewBeats()
	center := signalcenter.New()
	emitted := make(chan signalcenter.Event, 4)
	center.Subscribe(func(e signalcenter.Event) { emitted <- e })
	pushes := &pushRecorder{t: t}
	opts := &Options{Class: ClassCycle, ProjectRoot: root, CycleID: 7, RunID: "run-7", Signals: center, Runner: pushes.run,
		ShipWindow: windowOptions(clock, beats, nil, 101)}
	res := &RunResult{}
	ctx, release := enterShipWindow(context.Background(), opts, res)
	defer release()

	clock.Advance(shipwindow.DefaultMaxHold)
	beats.Tick(t, clock.Now())
	var lostSignal signalcenter.Event
	select {
	case lostSignal = <-emitted:
	case <-time.After(5 * time.Second):
		t.Fatal("the holder was not told that its lease passed the hard cap")
	}

	err := pushWithRepair(ctx, opts, res, landing.PushRequest{Branch: "main", Site: landing.SiteDirect})
	se, ok := core.AsShipError(err)
	if !ok || se.Code != core.CodeGitFleetRebaseNeeded || se.Class != core.ShipClassTransient {
		t.Fatalf("pushWithRepair err = %v, want a transient %s", err, core.CodeGitFleetRebaseNeeded)
	}
	if pushes.pushes != 0 {
		t.Fatalf("a holder without its lease pushed %d time(s)", pushes.pushes)
	}
	if lostSignal.Code != CodeShipWindowLeaseBroken || lostSignal.Cycle != 7 || lostSignal.Fields["role"] != "holder" {
		t.Fatalf("signal = %+v, want %s from cycle 7 with role=holder", lostSignal, CodeShipWindowLeaseBroken)
	}
	if !containsLog(*res, "lease lost: the hold passed the hard cap") {
		t.Fatalf("logs = %v, want the abort named", res.Logs)
	}
}

func TestPushWithRepair_AHeldLeaseStillPushes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	clock, beats := shipwindowtest.NewClock(), shipwindowtest.NewBeats()
	pushes := &pushRecorder{t: t}
	opts := &Options{Class: ClassCycle, ProjectRoot: root, Runner: pushes.run, ShipWindow: windowOptions(clock, beats, nil, 101)}
	res := &RunResult{}
	ctx, release := enterShipWindow(context.Background(), opts, res)
	defer release()
	if err := pushWithRepair(ctx, opts, res, landing.PushRequest{Branch: "main", Site: landing.SiteDirect}); err != nil {
		t.Fatalf("a holder with its lease: %v", err)
	}
	if pushes.pushes != 1 {
		t.Fatalf("pushes = %d, want 1", pushes.pushes)
	}
}

func TestEnterShipWindow_TwoLanesWithSectionsLongerThanTTLNeverOverlap(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	clock, beatsA, beatsB, probeB := shipwindowtest.NewClock(), shipwindowtest.NewBeats(), shipwindowtest.NewBeats(), shipwindowtest.NewProbe()
	laneA := &Options{Class: ClassCycle, ProjectRoot: root, CycleID: 1, ShipWindow: windowOptions(clock, beatsA, nil, 101)}
	laneB := &Options{Class: ClassCycle, ProjectRoot: root, CycleID: 2, ShipWindow: windowOptions(clock, beatsB, probeB, 202)}
	_, releaseA := enterShipWindow(context.Background(), laneA, &RunResult{})

	enteredB := make(chan func())
	resB := &RunResult{}
	probeB.Drain()
	go func() {
		_, releaseB := enterShipWindow(context.Background(), laneB, resB)
		enteredB <- releaseB
	}()

	lanes := shipwindowtest.Lanes{Clock: clock, Holder: beatsA, Waiter: probeB}
	for range 4 {
		lanes.Step(t, 5*time.Minute)
	}
	probeB.Drain()
	probeB.WaitPolls(t, 3)
	select {
	case <-enteredB:
		t.Fatal("lane B entered the ship window while lane A was still 20 minutes into its section")
	default:
	}
	releaseA()
	select {
	case releaseB := <-enteredB:
		releaseB()
	case <-time.After(5 * time.Second):
		t.Fatal("lane B never entered after lane A released")
	}
	broken, _, _ := strings.Cut(shipwindow.StaleHolder{OwnerPID: 101}.Reason(), " (heartbeat")
	if strings.Contains(strings.Join(resB.Logs, "\n"), broken) {
		t.Fatalf("lane B broke a lease it should have waited for: %v", resB.Logs)
	}
	if _, err := os.Stat(shipwindow.PathIn(filepath.Join(root, ".evolve"))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the lease outlived both lanes (stat err=%v)", err)
	}
}

func TestWorktreeShipLand_ALostShipWindowKeepsTheIntentResumable(t *testing.T) {
	t.Parallel()
	opts, r, s, _ := scriptedLanding(t, true)
	clock, beats := shipwindowtest.NewClock(), shipwindowtest.NewBeats()
	lost := make(chan struct{})
	w := windowOptions(clock, beats, nil, 101)
	w.OnLost = func(*shipwindow.LostError) { close(lost) }
	lease, err := shipwindow.Acquire(context.Background(), filepath.Join(opts.ProjectRoot, ".evolve"), w)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lease.Release() }()
	clock.Advance(shipwindow.DefaultMaxHold)
	beats.Tick(t, clock.Now())
	<-lost
	s.ctx = shipwindow.WithHeld(s.ctx, lease)

	landErr := s.land("")

	wantShipErr(t, landErr, core.CodeGitFleetRebaseNeeded, core.ShipClassTransient, "ship: the ship-window lease was lost before the push")
	for _, a := range r.argvs() {
		if strings.HasPrefix(a, "push ") {
			t.Fatalf("%s ran without the lease", a)
		}
	}
	if in := landingIntentOf(t, opts); in.Status != landing.IntentPrepared {
		t.Fatalf("intent %+v, want prepared so a retry resumes at the push", in)
	}
}
