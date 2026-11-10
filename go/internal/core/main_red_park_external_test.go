package core_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const parkedMainSHA = "9a2bbe0e1c0ffee"

var (
	parkDupKey   = baseattr.Key{Package: "internal/dup", Test: "TestCount"}
	parkSizeKey  = baseattr.Key{Package: "internal/size", Test: "TestSize"}
	parkOtherKey = baseattr.Key{Package: "internal/other", Test: "TestOther"}
	greenOnMain  = baseattr.Key{Package: "internal/stale", Test: "TestGreenOnMain"}
)

func parkErrorOf(v baseattr.Verdict) *core.ShipError {
	return core.NewShipError(shiperr.CodeMainRedPark, core.ShipClassPrecondition, core.StageAtomicShip,
		"the red tests of the lane are red on main", v.Debug()...)
}

func mainRedParkError() *core.ShipError {
	return parkErrorOf(baseattr.Verdict{
		BaseRed: []baseattr.Key{greenOnMain, parkDupKey, parkSizeKey}, MainRed: []baseattr.Key{parkDupKey, parkSizeKey},
		Base: "7aa5456e2", Main: parkedMainSHA,
	})
}

func seedParkedLane(t *testing.T, root string, cycleID int) core.CycleState {
	t.Helper()
	cycle := strconv.Itoa(cycleID)
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-"+cycle)
	worktree := filepath.Join(root, "worktree")
	for _, dir := range []string{workspace, worktree} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	statePath := filepath.Join(workspace, "cycle-state.json")
	for path, body := range map[string]string{
		statePath: `{"cycle_id":` + cycle + `,"phase":"ship"}`,
		filepath.Join(workspace, core.LaneScopeFile): `{"todo_ids":["item-dup"],"goal_hash":"wave-goal"}`,
	} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(ipcenv.CycleStateFileKey, statePath)
	return core.CycleState{CycleID: cycleID, RunID: "run-" + cycle, Phase: "ship", WorkspacePath: workspace, ActiveWorktree: worktree}
}

func TestRecoverShipError_MainRedParkWritesAMainRedCheckpointAndAStopRow(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	cs := seedParkedLane(t, root, 2249)
	center := signalcenter.New()
	var events []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) { events = append(events, e) })
	o := core.NewOrchestrator(&recStorage{}, &fakeLedger{}, newRunners(nil), core.WithSignalCenter(center))
	se := mainRedParkError()

	next, recovering := o.RecoverFromShipErrorForTest(context.Background(), root, 2249, &cs, se, 99, 1)

	if recovering || next != "" {
		t.Errorf("a park routed to %q (recovering=%v), want the cycle to end with no re-audit", next, recovering)
	}
	if !errors.Is(se, core.ErrMainRedParked) || errors.Is(core.NewShipError(core.CodeEGPSRedCount, core.ShipClassIntegrity, core.StageAtomicShip, "red"), core.ErrMainRedParked) {
		t.Error("only a ship error with the park code is ErrMainRedParked")
	}
	cp, _ := readCheckpointBlock(t, filepath.Join(cs.WorkspacePath, "cycle-state.json"))
	if cp["reason"] != core.MainRedReason || cp["enabled"] != true || cp["resumeFromPhase"] != "ship" {
		t.Errorf("checkpoint = %v, want an enabled main-red checkpoint that resumes at ship", cp)
	}
	stop, found, err := mainstop.Read(evolveDir)
	wantKeys := []baseattr.Key{parkDupKey, parkSizeKey}
	if err != nil || !found || !stop.Holds() || !slices.Equal(stop.Keys, wantKeys) || stop.MainSHA != parkedMainSHA || stop.FixItem != mainstop.FixItemID(parkedMainSHA) {
		t.Fatalf("stop = %+v, %v, %v, want a held stop with only the two keys that are red on main", stop, found, err)
	}
	if len(stop.Parked) != 1 || stop.Parked[0].Cycle != 2249 || !slices.Equal(stop.Parked[0].Scope, []string{"item-dup"}) || stop.Parked[0].Workspace != cs.WorkspacePath {
		t.Errorf("parked = %+v, want cycle 2249 with its scope and its workspace", stop.Parked)
	}
	paused, err := core.PausedLaneScopes(root, evolveDir, core.ResumeOptions{})
	if err != nil || len(paused) != 1 || paused[0].Cycle != 2249 || paused[0].Reason != core.MainRedReason || paused[0].Phase != "ship" {
		t.Errorf("paused lanes = %+v, %v, want cycle 2249 paused at ship for main-red", paused, err)
	}
	var parked []signalcenter.Event
	for _, e := range events {
		if e.Code == core.CodeMainRedParked {
			parked = append(parked, e)
		}
	}
	if len(parked) != 1 {
		t.Fatalf("GATE_REPO_MAIN_RED_PARKED signals = %d, want 1 in %+v", len(parked), events)
	}
	e := parked[0]
	if e.Severity != signalcenter.SeverityWarn || e.Module != signalcenter.ModuleGateRepo || e.Cycle != 2249 || e.Fields["parked"] != "2249" || e.Fields["fix_item"] != stop.FixItem || e.Fields["checkpoint_error"] != "" || e.Fields["stop_error"] != "" {
		t.Errorf("signal = %+v, want a gate warning of cycle 2249 with the fix item and no write error", e)
	}
	for _, want := range []string{"internal/dup.TestCount", parkedMainSHA, "not blamed", "next:", "green on main"} {
		if !strings.Contains(e.Reason, want) {
			t.Errorf("signal reason = %q, want it to hold %q", e.Reason, want)
		}
	}
}

func TestRecoverShipError_ASecondParkRefreshesTheStopAndKeepsItsOpenTime(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	cs := seedParkedLane(t, root, 2249)
	o := core.NewOrchestrator(&recStorage{}, &fakeLedger{}, newRunners(nil))
	o.RecoverFromShipErrorForTest(context.Background(), root, 2249, &cs, mainRedParkError(), 0, 1)
	first, _, err := mainstop.Read(evolveDir)
	if err != nil {
		t.Fatal(err)
	}
	later := parkErrorOf(baseattr.Verdict{BaseRed: []baseattr.Key{parkOtherKey}, MainRed: []baseattr.Key{parkOtherKey}, Main: parkedMainSHA})
	unjudged := parkErrorOf(baseattr.Verdict{BaseRed: []baseattr.Key{greenOnMain}})

	second, third := cs, cs
	second.CycleID, third.CycleID = 2251, 2252
	o.RecoverFromShipErrorForTest(context.Background(), root, 2251, &second, later, 0, 1)
	o.RecoverFromShipErrorForTest(context.Background(), root, 2252, &third, unjudged, 0, 1)

	stop, _, err := mainstop.Read(evolveDir)
	if err != nil || !slices.Equal(stop.Keys, []baseattr.Key{parkDupKey, parkSizeKey, parkOtherKey}) || len(stop.Parked) != 3 || stop.MainSHA != parkedMainSHA || !stop.OpenedAt.Equal(first.OpenedAt) || stop.FixItem != first.FixItem {
		t.Errorf("stop = %+v, %v, want the 3 main-red keys, 3 parked cycles, and the tip, open time and fix item of the first park", stop, err)
	}
}

func TestRecoverShipError_AParkOnAnUnjudgedMainOpensNoStopAndStaysResumable(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	cs := seedParkedLane(t, root, 2249)
	center := signalcenter.New()
	var parked []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) {
		if e.Code == core.CodeMainRedParked {
			parked = append(parked, e)
		}
	})
	o := core.NewOrchestrator(&recStorage{}, &fakeLedger{}, newRunners(nil), core.WithSignalCenter(center))

	o.RecoverFromShipErrorForTest(context.Background(), root, 2249, &cs, parkErrorOf(baseattr.Verdict{BaseRed: []baseattr.Key{parkDupKey}, Base: "7aa5456e2"}), 0, 1)

	stop, found, err := mainstop.Read(evolveDir)
	if err != nil || !found || stop.Holds() || len(stop.Keys) != 0 || stop.FixItem != "" || stop.ParkedCycles() != "2249" {
		t.Fatalf("stop = %+v, %v, %v, want no held stop, no key, no fix item and the parked row of cycle 2249", stop, found, err)
	}
	paused, err := core.PausedLaneScopes(root, evolveDir, core.ResumeOptions{})
	if err != nil || len(paused) != 1 || paused[0].Reason != core.MainRedReason {
		t.Errorf("paused lanes = %+v, %v, want cycle 2249 paused for main-red", paused, err)
	}
	if len(parked) != 1 || parked[0].Severity != signalcenter.SeverityWarn || parked[0].Fields["keys"] != "" || parked[0].Fields["fix_item"] != "" {
		t.Fatalf("signals = %+v, want one warning with no key and no fix item", parked)
	}
	for _, want := range []string{"not blamed", "ship did not judge main", "next:", "judges main again"} {
		if !strings.Contains(parked[0].Reason, want) {
			t.Errorf("signal reason = %q, want it to hold %q", parked[0].Reason, want)
		}
	}
}
