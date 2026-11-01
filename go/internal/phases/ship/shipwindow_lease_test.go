package ship

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindow"
)

func TestEnterShipWindow_OnlyALiveCycleShipTakesTheLease(t *testing.T) {
	t.Parallel()
	for name, opts := range map[string]Options{
		"manual":       {Class: ClassManual},
		"cycle-dryrun": {Class: ClassCycle, DryRun: true},
	} {
		t.Run(name, func(t *testing.T) {
			opts.ProjectRoot = t.TempDir()
			release := enterShipWindow(context.Background(), &opts, &RunResult{})
			if _, err := os.Stat(filepath.Join(opts.ProjectRoot, ".evolve")); !os.IsNotExist(err) {
				t.Fatalf("a %s ship touched the ship window (stat err=%v)", name, err)
			}
			release()
		})
	}
}

func TestEnterShipWindow_AWaiterOutlastsAHealthyHolderWithoutAWarn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sibling, err := shipwindow.Acquire(context.Background(), filepath.Join(root, ".evolve"), shipwindow.Options{OwnerPID: 1})
	if err != nil {
		t.Fatal(err)
	}
	released := make(chan struct{})
	go func() {
		time.Sleep(300 * time.Millisecond)
		if err := sibling.Release(); err != nil {
			t.Error(err)
		}
		close(released)
	}()
	res := &RunResult{}
	release := enterShipWindow(context.Background(), &Options{Class: ClassCycle, ProjectRoot: root}, res)
	select {
	case <-released:
	default:
		t.Fatal("the lane entered the ship window while a healthy sibling still held it")
	}
	if !leaseHeld(root) {
		t.Fatal("the lane waited out the sibling but holds no lease")
	}
	if containsLog(*res, "ship-window lease not acquired") {
		t.Fatalf("a waiter behind a healthy holder logged an unleased ship: %v", res.Logs)
	}
	release()
	if leaseHeld(root) {
		t.Fatal("release left the lease on disk")
	}
}

func TestEnterShipWindow_ACanceledWaitShipsUnleasedWithAWarn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sibling, err := shipwindow.Acquire(context.Background(), filepath.Join(root, ".evolve"), shipwindow.Options{OwnerPID: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sibling.Release() }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := &RunResult{}
	release := enterShipWindow(ctx, &Options{Class: ClassCycle, ProjectRoot: root}, res)
	release()
	if !containsLog(*res, "[ship] WARN: ship-window lease not acquired (context canceled); proceeding unleased") {
		t.Fatalf("logs=%v", res.Logs)
	}
	if !leaseHeld(root) {
		t.Fatal("the unleased lane's release removed the sibling's lease")
	}
}

func TestEnterShipWindow_AReleaseFailureIsLogged(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	res := &RunResult{}
	release := enterShipWindow(context.Background(), &Options{Class: ClassCycle, ProjectRoot: root}, res)
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.RemoveAll(evolveDir); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, evolveDir, "now a file")
	release()
	if !containsLog(*res, "[ship] WARN: ship-window lease release: ") {
		t.Fatalf("logs=%v", res.Logs)
	}
}

func TestEnterShipWindow_AHeldContextReentersWithoutWaitingOrReleasing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outer, err := shipwindow.Acquire(context.Background(), filepath.Join(root, ".evolve"), shipwindow.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = outer.Release() }()
	ctx, cancel := context.WithTimeout(shipwindow.WithHeld(context.Background(), outer), 2*time.Second)
	defer cancel()
	res := &RunResult{}
	release := enterShipWindow(ctx, &Options{Class: ClassCycle, ProjectRoot: root}, res)
	release()
	if ctx.Err() != nil || len(res.Logs) != 0 {
		t.Fatalf("a ship inside the orchestrator's held window waited or warned: ctxErr=%v logs=%v", ctx.Err(), res.Logs)
	}
	if !leaseHeld(root) {
		t.Fatal("the inner release removed the orchestrator's lease")
	}
}

func TestEnterShipWindow_BreakingAHungHolderWarnsAndSignals(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, shipwindow.PathIn(filepath.Join(root, ".evolve")),
		fmt.Sprintf(`{"owner_pid":%d,"heartbeat_at":"2026-10-19T01:13:10Z","token":"hung"}`+"\n", os.Getpid()))
	center, events := recordingCenter()
	res := &RunResult{}
	release := enterShipWindow(context.Background(), &Options{Class: ClassCycle, ProjectRoot: root, CycleID: 7, Signals: center}, res)
	defer release()
	center.Flush()
	want := fmt.Sprintf("[ship] WARN: ship-window lease of pid %d broken at the TTL (heartbeat 2026-10-19T01:13:10Z)", os.Getpid())
	if !containsLog(*res, want) {
		t.Fatalf("logs=%v, want %q", res.Logs, want)
	}
	found := false
	for _, e := range *events {
		found = found || (e.Code == codeShipWindowLeaseBroken && e.Fields["owner_pid"] == strconv.Itoa(os.Getpid()))
	}
	if !found {
		t.Fatalf("signals=%+v, want %s naming the broken holder", *events, codeShipWindowLeaseBroken)
	}
}
