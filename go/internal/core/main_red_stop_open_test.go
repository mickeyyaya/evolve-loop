package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var dupRedOnMain = baseattr.Verdict{
	MainRed: []baseattr.Key{{Package: "internal/dup", Test: "TestCount"}},
	Main:    "9a2bbe0e1c0ffee",
}

var threeRedOnMain = baseattr.Verdict{
	Main: "0123456789abcdef",
	MainRed: []baseattr.Key{
		{Package: "example.com/m/pkg/a", Test: "TestOne"},
		{Package: "example.com/m/pkg/a", Test: "TestTwo"},
		{Package: "example.com/m/pkg/b", Test: "TestThree"},
	},
}

var stopOpenedAt = time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)

func fixedOpener() mainRedStopOpener {
	return mainRedStopOpener{now: func() time.Time { return stopOpenedAt }}
}

func pendingFixItems(t *testing.T, root string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func inboxThatIsAFile(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox"), []byte("a file"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestOpenMainRedStop_TheFirstCallWritesTheStopAndFilesOneFixItem(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	opened, err := fixedOpener().open(root, threeRedOnMain, 7)

	if !opened || err != nil {
		t.Fatalf("opened = %v err = %v, want the first call to open the stop", opened, err)
	}
	stop, found, err := mainstop.Read(filepath.Join(root, ".evolve"))
	if err != nil || !found || !stop.Holds() || stop.FixItem != "fix-main-red-01234567" || len(stop.Keys) != 3 || !stop.OpenedAt.Equal(stopOpenedAt) {
		t.Fatalf("stop = %+v found = %v err = %v, want a held stop of three keys that opened at the clock time", stop, found, err)
	}
	paths := pendingFixItems(t, root)
	if len(paths) != 1 {
		t.Fatalf("pending items = %v, want one fix item", paths)
	}
	raw, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	var item mainRedFixItem
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatal(err)
	}
	if item.ID != stop.FixItem || item.Kind != "fix" || item.PriorityClass != "correctness" || item.Weight != 1.0 {
		t.Errorf("item = %+v, want the fix item of the stop as a correctness fix of weight 1, which a lane can take", item)
	}
	if item.Priority != "critical" || item.Source != "main-red-stop" {
		t.Errorf("priority = %q source = %q, want a critical item from the main-red stop", item.Priority, item.Source)
	}
	var filed map[string]any
	if err := json.Unmarshal(raw, &filed); err != nil {
		t.Fatal(err)
	}
	if filed["origin"] != "cycle" || filed["created_at"] != stopOpenedAt.Format(time.RFC3339) {
		t.Errorf("origin = %v created_at = %v, want a cycle item that was filed at the clock time", filed["origin"], filed["created_at"])
	}
	if got := strings.Join(item.Files, ","); got != "example.com/m/pkg/a,example.com/m/pkg/b" {
		t.Errorf("files = %q, want each key package once", got)
	}
	if len(item.Acceptance) == 0 || !strings.Contains(item.Acceptance[0], "TestThree") {
		t.Errorf("acceptance = %q, want each key named green on main", item.Acceptance)
	}
}

func TestOpenMainRedStop_ASecondCallOnAHeldStopOpensNothingAndFilesNoSecondItem(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := OpenMainRedStop(root, threeRedOnMain, 7); err != nil {
		t.Fatal(err)
	}
	later := baseattr.Verdict{Main: "fedcba9876543210", MainRed: []baseattr.Key{{Package: "example.com/m/pkg/c", Test: "TestFour"}}}

	opened, err := OpenMainRedStop(root, later, 8)

	if opened || err != nil {
		t.Fatalf("opened = %v err = %v, want a held stop to stay the same stop", opened, err)
	}
	stop, _, err := mainstop.Read(filepath.Join(root, ".evolve"))
	if err != nil || stop.FixItem != "fix-main-red-01234567" || len(stop.Keys) != 4 {
		t.Errorf("stop = %+v err = %v, want the first fix item and the fourth key", stop, err)
	}
	if paths := pendingFixItems(t, root); len(paths) != 1 {
		t.Errorf("pending items = %v, want still one fix item", paths)
	}
}

func TestOpenMainRedStop_AReleasedStopOpensAgainOnTheNextRedMain(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := OpenMainRedStop(root, threeRedOnMain, 7); err != nil {
		t.Fatal(err)
	}
	released, err := mainstop.Update(filepath.Join(root, ".evolve"), func(s mainstop.Stop, _ bool) (mainstop.Stop, error) {
		return s.Released(s.Keys, "landing", stopOpenedAt), nil
	})
	if err != nil || released.Holds() {
		t.Fatalf("released = %+v err = %v, want a stop that does not hold", released, err)
	}

	opened, err := OpenMainRedStop(root, dupRedOnMain, 9)

	if !opened || err != nil {
		t.Errorf("opened = %v err = %v, want a red main after a release to open a stop", opened, err)
	}
}

func TestOpenMainRedStop_AFixItemThatWasLostIsFiledAgainOnTheNextCall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := OpenMainRedStop(root, threeRedOnMain, 7); err != nil {
		t.Fatal(err)
	}
	for _, path := range pendingFixItems(t, root) {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}

	opened, err := OpenMainRedStop(root, threeRedOnMain, 8)

	if opened || err != nil || len(pendingFixItems(t, root)) != 1 {
		t.Errorf("opened = %v err = %v items = %v, want the held stop to get its fix item back", opened, err, pendingFixItems(t, root))
	}
}

func TestOpenMainRedStop_AStopOrAnInboxThatCannotBeWrittenIsAnError(t *testing.T) {
	t.Parallel()
	blocked := t.TempDir()
	if err := os.WriteFile(filepath.Join(blocked, ".evolve"), []byte("a file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if opened, err := OpenMainRedStop(blocked, threeRedOnMain, 7); opened || err == nil || !strings.Contains(err.Error(), "write the stop") {
		t.Errorf("opened = %v err = %v, want an unwritable stop to be an error", opened, err)
	}

	readOnly := filepath.Join(t.TempDir(), ".evolve")
	if err := os.MkdirAll(readOnly, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(readOnly, "main-red-stop.json.lock"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(readOnly, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0o755) })
	if opened, err := OpenMainRedStop(filepath.Dir(readOnly), threeRedOnMain, 7); opened || err == nil || !strings.Contains(err.Error(), "write the stop") {
		t.Errorf("opened = %v err = %v, want a stop whose write fails after the lock to be not opened", opened, err)
	}

	opened, err := OpenMainRedStop(inboxThatIsAFile(t), threeRedOnMain, 7)
	if !opened || err == nil || !strings.Contains(err.Error(), "fix-main-red-01234567") {
		t.Errorf("opened = %v err = %v, want the stop open and the lost fix item named", opened, err)
	}
}

func TestStopOpening_AStopThatOpensEmitsOneSignalWithTheKeysAndTheFixItem(t *testing.T) {
	t.Parallel()
	center, events := captureSignals()
	var root string
	var cycle int
	sink := func(r string, _ baseattr.Verdict, c int) (bool, error) { root, cycle = r, c; return true, nil }

	err := stopOpening{projectRoot: "/repo", runID: "run-7", phase: "build", origin: "test", cycle: 7, verdict: dupRedOnMain}.through(sink, center)

	if err != nil || root != "/repo" || cycle != 7 || len(*events) != 1 {
		t.Fatalf("err = %v root = %q cycle = %d events = %+v, want one signal", err, root, cycle, *events)
	}
	e := (*events)[0]
	if e.Code != CodeMainRedStopOpened || e.Module != signalcenter.ModuleGateRepo || e.Severity != signalcenter.SeverityWarn || e.Cycle != 7 || e.RunID != "run-7" || e.Phase != "build" {
		t.Errorf("signal = %+v, want a gate.repo warning of cycle 7", e)
	}
	if e.Fields["keys"] != "internal/dup.TestCount" || e.Fields["main"] != "9a2bbe0e1c0ffee" || e.Fields["fix_item"] != "fix-main-red-9a2bbe0e" || e.Fields["fix_item_error"] != "" || !strings.Contains(e.Reason, "next:") {
		t.Errorf("signal = %+v, want the key, the main commit, the fix item and a next step", e)
	}
}

func TestStopOpening_AStopThatOpensWithNoFixItemIsAnIncidentThatNamesTheCause(t *testing.T) {
	t.Parallel()
	center, events := captureSignals()
	failed := errors.New("inbox is read-only")
	sink := func(string, baseattr.Verdict, int) (bool, error) { return true, failed }

	err := stopOpening{verdict: dupRedOnMain}.through(sink, center)

	if !errors.Is(err, failed) || len(*events) != 1 {
		t.Fatalf("err = %v events = %+v, want the error and one signal", err, *events)
	}
	e := (*events)[0]
	if e.Severity != signalcenter.SeverityIncident || e.Fields["fix_item_error"] != "inbox is read-only" || !strings.Contains(e.Reason, "is not in the inbox") || !strings.Contains(e.Reason, "next:") {
		t.Errorf("signal = %+v, want an incident that names the missing fix item and a next step", e)
	}
}

func TestStopOpening_AHeldStopOrAGreenMainEmitsNothing(t *testing.T) {
	t.Parallel()
	center, events := captureSignals()
	failed := errors.New("record locked")
	calls := 0
	held := func(string, baseattr.Verdict, int) (bool, error) { calls++; return false, failed }

	if err := (stopOpening{verdict: dupRedOnMain}).through(held, center); !errors.Is(err, failed) || calls != 1 {
		t.Errorf("err = %v calls = %d, want the error of the sink", err, calls)
	}
	if err := (stopOpening{verdict: baseattr.Verdict{Main: "abc"}}).through(held, center); err != nil || calls != 1 {
		t.Errorf("err = %v calls = %d, want a green main to call no sink", err, calls)
	}
	if len(*events) != 0 {
		t.Errorf("events = %+v, want none", *events)
	}
}

func TestOpenMainRedStop_TheBuildFloorSignalsTheOpeningWithItsRunAndTheBuildPhase(t *testing.T) {
	t.Parallel()
	center, events := captureSignals()
	sink := func(string, baseattr.Verdict, int) (bool, error) { return true, nil }

	ReviewInput{RunID: "run-9", Cycle: 9, Signals: center, MainRedStop: sink}.openMainRedStop(dupRedOnMain)

	if len(*events) != 1 {
		t.Fatalf("events = %+v, want one signal", *events)
	}
	if e := (*events)[0]; e.RunID != "run-9" || e.Cycle != 9 || e.Phase != string(PhaseBuild) || e.Code != CodeMainRedStopOpened {
		t.Errorf("signal = %+v, want the opening of run-9, cycle 9, in the build phase", e)
	}
}

func TestMainRedSink_TheInputHookWinsAndTheRealOpenerIsTheDefault(t *testing.T) {
	t.Parallel()
	center, _ := captureSignals()
	byDefault, byInput := t.TempDir(), t.TempDir()
	calls := 0
	own := func(string, baseattr.Verdict, int) (bool, error) { calls++; return false, nil }

	ReviewInput{ProjectRoot: byDefault, Signals: center}.openMainRedStop(dupRedOnMain)
	ReviewInput{ProjectRoot: byInput, Signals: center, MainRedStop: own}.openMainRedStop(dupRedOnMain)

	if stop, _, err := mainstop.Read(filepath.Join(byDefault, ".evolve")); err != nil || !stop.Holds() {
		t.Errorf("stop = %+v err = %v, want the real opener to write the stop by default", stop, err)
	}
	if _, found, err := mainstop.Read(filepath.Join(byInput, ".evolve")); err != nil || found || calls != 1 {
		t.Errorf("found = %v err = %v calls = %d, want only the input hook to run", found, err, calls)
	}
}

func parkOnRedMainAt(t *testing.T, root string) []signalcenter.Event {
	t.Helper()
	swapMainRedCheckpointer(t, func(CycleState, string, time.Time) error { return nil })
	center, events := captureSignals()
	o := NewOrchestrator(&fakeStorage{state: State{}}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(center), WithStderr(&bytes.Buffer{}))
	o.now = func() time.Time { return stopOpenedAt }
	o.parkOnMainRed(root, CycleState{CycleID: 2249, RunID: "run-2249", WorkspacePath: t.TempDir()}, baseattr.VerdictOfDebug(parkError().Debug))
	return *events
}

func TestParkOnMainRed_OpensTheStopAndFilesTheFixItemBeforeItAddsTheParkedRow(t *testing.T) {
	root := t.TempDir()

	events := parkOnRedMainAt(t, root)

	if len(events) != 2 || events[0].Code != CodeMainRedStopOpened || events[1].Code != CodeMainRedParked {
		t.Fatalf("events = %+v, want the stop opened and then the park", events)
	}
	if events[0].Phase != string(PhaseShip) || events[0].Cycle != 2249 || events[0].RunID != "run-2249" || events[1].Severity != signalcenter.SeverityWarn {
		t.Errorf("events = %+v, want a ship-phase opening of cycle 2249 and a whole park", events)
	}
	stop, _, err := mainstop.Read(filepath.Join(root, ".evolve"))
	if err != nil || !stop.Holds() || !stop.OpenedAt.Equal(stopOpenedAt) || stop.ParkedCycles() != "2249" || len(pendingFixItems(t, root)) != 1 {
		t.Errorf("stop = %+v err = %v items = %v, want a stop held since the clock time with the parked cycle and one fix item", stop, err, pendingFixItems(t, root))
	}
}

func TestParkOnMainRed_AFixItemThatCannotBeFiledMakesTheParkAnIncident(t *testing.T) {
	events := parkOnRedMainAt(t, inboxThatIsAFile(t))

	if len(events) != 2 || events[0].Code != CodeMainRedStopOpened || events[0].Severity != signalcenter.SeverityIncident {
		t.Fatalf("events = %+v, want an opening incident and then the park", events)
	}
	if events[1].Severity != signalcenter.SeverityIncident || !strings.Contains(events[1].Fields["stop_error"], "fix-main-red-9a2bbe0e") {
		t.Errorf("park = %+v, want an incident that names the fix item that was not filed", events[1])
	}
}
