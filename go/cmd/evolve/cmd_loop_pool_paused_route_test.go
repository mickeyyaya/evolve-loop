package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const routedGammaItem = `{"id":"gamma","title":"host sentinel","kind":"bug","weight":0.1,"route":"console-manual","files":["go/internal/gamma/c.go"]}`

func TestProductionPoolPlanFn_APausedScopeOfAConsoleRoutedItemLeadsNoLane(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	evolveDir := filepath.Join(root, ".evolve")
	seedPoolInbox(t, evolveDir, map[string]string{"2026-11-03T00-00-02Z-gamma.json": routedGammaItem})
	seedPausedRun(t, root, 41, 0, 3, "gamma")
	var stderr bytes.Buffer
	plan := poolPlanner{cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, storage: &erroringReadStorage{}, stderr: &stderr}.plan(2)

	todos, err := plan(context.Background(), 0)

	for _, td := range todos {
		if strings.Contains(td.ID, "gamma") {
			t.Fatalf("todos = %+v, want no lane led by the console-routed gamma", todos)
		}
	}
	for _, want := range []string{"paused cycle 41", "gamma", "console-manual"} {
		if err != nil || !strings.Contains(stderr.String(), want) {
			t.Fatalf("err %v stderr %q, want a warning that names %q", err, stderr.String(), want)
		}
	}
}

func TestProductionPoolPlanFn_AClaimedConsoleRoutedItemOfAPausedScopeLeadsNoLane(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	evolveDir := filepath.Join(root, ".evolve")
	seedPoolInbox(t, evolveDir, map[string]string{"2026-11-03T00-00-02Z-gamma.json": `{"id":"gamma","weight":0.1,"files":["go/internal/gamma/c.go"]}`})
	seedPausedRun(t, root, 41, 0, 3, "gamma")
	claimFor(t, evolveDir, 41, "gamma")
	loc, err := inboxmover.Locate(filepath.Join(evolveDir, "inbox"), "gamma")
	if err != nil {
		t.Fatal(err)
	}
	writeE2File(t, loc.Path, routedGammaItem)
	var stderr bytes.Buffer
	plan := poolPlanner{cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, storage: &erroringReadStorage{}, stderr: &stderr}.plan(2)

	todos, perr := plan(context.Background(), 0)

	for _, td := range todos {
		if strings.Contains(td.ID, "gamma") {
			t.Fatalf("todos = %+v, want no lane led by the claimed console-routed gamma", todos)
		}
	}
	if perr != nil || !strings.Contains(stderr.String(), "paused cycle 41") || !strings.Contains(stderr.String(), "gamma") {
		t.Fatalf("err %v stderr %q, want a warning that names cycle 41 and gamma", perr, stderr.String())
	}
}

func TestLaneResume_ALaneDoesNotResumeAScopeOfAConsoleRoutedItem(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2107)
	p.addItem(t, "item-a", map[string]any{"route": "console-manual"})
	var stderr bytes.Buffer
	var got startedCycle

	_, err := p.lane(&stderr, signalcenter.New(), "item-a").run(context.Background(), recordingStarter{&got}, laneRequest())

	if err != nil || got.resumed != nil {
		t.Fatalf("run err %v resumed %+v, want a fresh cycle", err, got.resumed)
	}
	if cp := readCheckpoint(t, p.statePath); cp.AutoResumeAttempts != 0 {
		t.Errorf("checkpoint = %+v, want no auto-resume attempt spent on a refused scope", cp)
	}
	if !strings.Contains(stderr.String(), "console-manual") || !strings.Contains(stderr.String(), "cycle 2107") {
		t.Errorf("stderr %q, want the cause and the cycle", stderr.String())
	}
}

func TestScopeHold_AdoptableByRefusesAProtectedSurfaceItem(t *testing.T) {
	t.Parallel()
	inbox := t.TempDir()
	writeE2File(t, filepath.Join(inbox, "2026-11-03T00-00-00Z-p.json"), `{"id":"p","files":["go/guarded.go"]}`)
	hold := scopeHold{inbox: inbox, scope: []string{"p"}, protected: func(path string) bool { return path == "go/guarded.go" }}

	err := hold.adoptableBy(0)

	if err == nil || !strings.Contains(err.Error(), "p is console-routed") || !strings.Contains(err.Error(), "go/guarded.go") {
		t.Fatalf("adoptableBy = %v, want a console-routed refusal that names p and the path", err)
	}
}

func TestScopeHold_AdoptableByLeavesAnItemThatDoesNotDecodeToTheClaimFloor(t *testing.T) {
	t.Parallel()
	inbox := t.TempDir()
	writeE2File(t, filepath.Join(inbox, "2026-11-03T00-00-00Z-p.json"), `{"id":"p","weight":"heavy"}`)
	hold := scopeHold{inbox: inbox, scope: []string{"p"}}

	if err := hold.adoptableBy(0); err != nil {
		t.Fatalf("adoptableBy = %v, want nil: the claim floor judges an item that does not decode", err)
	}
}
