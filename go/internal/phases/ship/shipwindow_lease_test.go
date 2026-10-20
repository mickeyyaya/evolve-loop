package ship

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindow"
)

func leaseHeld(root string) bool {
	_, err := os.Stat(shipwindow.PathIn(filepath.Join(root, ".evolve")))
	return err == nil
}

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
