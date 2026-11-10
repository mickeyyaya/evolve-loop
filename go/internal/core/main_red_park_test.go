package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func parkError() *ShipError {
	dup := []baseattr.Key{{Package: "internal/dup", Test: "TestCount"}}
	return NewShipError(shiperr.CodeMainRedPark, ShipClassPrecondition, StageAtomicShip, "red on main",
		baseattr.Verdict{BaseRed: dup, MainRed: dup, Main: "9a2bbe0e1c0ffee"}.Debug()...)
}

func swapMainRedCheckpointer(t *testing.T, hook func(CycleState, string, time.Time) error) {
	t.Helper()
	prior := MainRedBoundaryCheckpointer
	MainRedBoundaryCheckpointer = hook
	t.Cleanup(func() { MainRedBoundaryCheckpointer = prior })
}

func parkWith(t *testing.T, root string) (signalcenter.Event, string) {
	t.Helper()
	center, events := captureSignals()
	var stderr bytes.Buffer
	o := NewOrchestrator(&fakeStorage{state: State{}}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(center), WithStderr(&stderr))
	o.parkOnMainRed(root, CycleState{CycleID: 2249, RunID: "run-2249", WorkspacePath: t.TempDir()}, baseattr.VerdictOfDebug(parkError().Debug))
	if len(*events) != 1 || (*events)[0].Code != CodeMainRedParked {
		t.Fatalf("signals = %+v, want one GATE_REPO_MAIN_RED_PARKED", *events)
	}
	return (*events)[0], stderr.String()
}

func TestParkOnMainRed_AFailedCheckpointIsAnIncidentAndTheStopRowIsStillWritten(t *testing.T) {
	cases := map[string]struct {
		hook func(CycleState, string, time.Time) error
		want string
	}{
		"no checkpointer is wired": {nil, errNoMainRedCheckpointer.Error()},
		"the checkpointer fails":   {func(CycleState, string, time.Time) error { return errors.New("disk is full") }, "disk is full"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			swapMainRedCheckpointer(t, c.hook)
			root := t.TempDir()
			e, stderr := parkWith(t, root)
			if e.Severity != signalcenter.SeverityIncident || e.Fields["checkpoint_error"] != c.want || e.Fields["stop_error"] != "" || !strings.Contains(e.Reason, c.want) || !strings.Contains(e.Reason, "next:") {
				t.Errorf("signal = %+v, want an incident that names %q and a next step", e, c.want)
			}
			if !strings.Contains(stderr, "main-red park of cycle 2249 is not whole") || !strings.Contains(stderr, c.want) {
				t.Errorf("stderr = %q, want the failed park and its cause", stderr)
			}
			if stop, found, err := mainstop.Read(filepath.Join(root, ".evolve")); err != nil || !found || len(stop.Parked) != 1 {
				t.Errorf("stop = %+v, %v, %v, want the parked row", stop, found, err)
			}
		})
	}
}

func TestParkOnMainRed_AFailedStopWriteIsAnIncident(t *testing.T) {
	swapMainRedCheckpointer(t, func(CycleState, string, time.Time) error { return nil })
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".evolve"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	e, stderr := parkWith(t, root)
	if e.Severity != signalcenter.SeverityIncident || e.Fields["stop_error"] == "" || e.Fields["checkpoint_error"] != "" || strings.Contains(e.Reason, "\n") {
		t.Errorf("signal = %+v, want a one-line incident with the stop error", e)
	}
	if !strings.Contains(stderr, "is not whole") {
		t.Errorf("stderr = %q, want the failed park", stderr)
	}
}

func TestParkOnMainRed_TwoFailedWritesGiveAOneLineIncident(t *testing.T) {
	swapMainRedCheckpointer(t, func(CycleState, string, time.Time) error { return errors.New("disk is full") })
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".evolve"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	e, stderr := parkWith(t, root)
	if e.Severity != signalcenter.SeverityIncident || !strings.Contains(e.Reason, "disk is full; ") || strings.Contains(e.Reason, "\n") {
		t.Errorf("signal reason = %q, want the two causes on one line", e.Reason)
	}
	if strings.Count(stderr, "\n") != 1 || !strings.Contains(stderr, "disk is full; ") {
		t.Errorf("stderr = %q, want one line with the two causes", stderr)
	}
}

func TestRunCycle_AFirstShipThatParksOnARedMainIsNotAFailedCycle(t *testing.T) {
	root := t.TempDir()
	swapMainRedCheckpointer(t, func(CycleState, string, time.Time) error { return nil })
	st, runners, o := resumeRigE4(t, t.TempDir(), nil)
	runners[PhaseShip] = &fakeRunner{name: "ship", failErr: parkError(), failUntil: 99}

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root})

	if !isParked(err) || !errors.Is(err, ErrMainRedParked) {
		t.Fatalf("err = %v, want the park of the first ship", err)
	}
	if res.FinalVerdict == VerdictFAIL || len(st.state.FailedAt) != 0 {
		t.Errorf("a parked cycle has verdict %q and failed approaches %v, want no failure of the lane", res.FinalVerdict, st.state.FailedAt)
	}
	if stop, found, err := mainstop.Read(filepath.Join(root, ".evolve")); err != nil || !found || !stop.Holds() || len(stop.Parked) != 1 || stop.Parked[0].Cycle != res.Cycle {
		t.Errorf("stop = %+v, %v, %v, want a held stop with cycle %d parked", stop, found, err, res.Cycle)
	}
}

func TestParkOnMainRed_AWholeParkIsAWarningAndPrintsNothing(t *testing.T) {
	swapMainRedCheckpointer(t, func(CycleState, string, time.Time) error { return nil })
	e, stderr := parkWith(t, t.TempDir())
	if e.Severity != signalcenter.SeverityWarn || stderr != "" || e.Fields["keys"] != "internal/dup.TestCount" || e.Fields["main"] != "9a2bbe0e1c0ffee" {
		t.Errorf("signal = %+v, stderr = %q, want a warning with the key and the main commit, and no stderr line", e, stderr)
	}
}

func TestIsParked_IsTrueForAWallAndForAMainRedPark(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		err    error
		parked bool
		cause  string
	}{
		{"a wall", fmt.Errorf("dispatch: %w", ErrAllFamiliesExhausted), true, wallPauseCause},
		{"a main-red park", fmt.Errorf("ship: %w", parkError()), true, mainRedParkCause},
		{"another ship error", NewShipError(CodeEGPSRedCount, ShipClassIntegrity, StageAtomicShip, "red"), false, wallPauseCause},
		{"no error", nil, false, wallPauseCause},
	}
	for _, c := range cases {
		if got := isParked(c.err); got != c.parked {
			t.Errorf("%s: isParked = %v, want %v", c.name, got, c.parked)
		}
		if got := parkCause(c.err); got != c.cause {
			t.Errorf("%s: parkCause = %q, want %q", c.name, got, c.cause)
		}
	}
}

func TestAbnormalEpilogue_AMainRedParkKeepsTheCycleResumable(t *testing.T) {
	t.Parallel()
	cr, root := epilogueRun(t, false)
	cr.abnormalEpilogue(fmt.Errorf("phase ship: %w", parkError()))
	if _, err := os.Stat(filepath.Join(root, "knowledge-base", "cycles", "cycle-77.json")); err == nil {
		t.Error("a parked cycle got a failure dossier")
	}
	if cr.cs.Phase != "retro" || cr.cs.ActiveAgent != "retro" {
		t.Errorf("a parked cycle changed its state to phase %q, agent %q; an aborted phase cannot resume", cr.cs.Phase, cr.cs.ActiveAgent)
	}
}

func TestCheckpointInterruptedPhase_AMainRedParkKeepsItsCheckpoint(t *testing.T) {
	prior := ResumeBoundaryCheckpointer
	t.Cleanup(func() { ResumeBoundaryCheckpointer = prior })
	calls := 0
	ResumeBoundaryCheckpointer = func(CycleState, string, time.Time) error { calls++; return nil }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cr := &cycleRun{ctx: ctx, o: &Orchestrator{now: time.Now}}
	cr.checkpointInterruptedPhase(fmt.Errorf("phase ship: %w", parkError()))
	if calls != 0 {
		t.Errorf("the interrupt checkpoint ran %d times over a main-red park, want 0: it replaces the main-red reason", calls)
	}
	cr.checkpointInterruptedPhase(context.Canceled)
	if calls != 1 {
		t.Errorf("the interrupt checkpoint ran %d times for a canceled phase, want 1", calls)
	}
}

func TestResumeExecution_AShipThatParksOnARedMainIsNotAFailedCycle(t *testing.T) {
	root := t.TempDir()
	swapMainRedCheckpointer(t, func(CycleState, string, time.Time) error { return nil })
	st, runners, o := resumeRigE4(t, t.TempDir(), nil)
	runners[PhaseShip] = &fakeRunner{name: "ship", failErr: parkError(), failUntil: 99}

	res, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: root}, &ResumePoint{Phase: string(PhaseShip), CycleID: 61})

	if !errors.Is(err, ErrMainRedParked) {
		t.Fatalf("err = %v, want the park of the resumed ship", err)
	}
	if res.FinalVerdict == VerdictFAIL || len(st.state.FailedAt) != 0 {
		t.Errorf("a parked resume has verdict %q and failed approaches %v, want no failure of the lane", res.FinalVerdict, st.state.FailedAt)
	}
	if stop, found, err := mainstop.Read(filepath.Join(root, ".evolve")); err != nil || !found || !stop.Holds() || stop.ParkedCycles() != "61" {
		t.Errorf("stop = %+v, %v, %v, want a held stop with cycle 61 parked", stop, found, err)
	}
}

func TestIsResumableReason_AMainRedParkIsResumable(t *testing.T) {
	t.Parallel()
	if !IsResumableReason(MainRedReason) {
		t.Errorf("IsResumableReason(%q) = false, so resume discovery and the inbox holder drop a parked lane", MainRedReason)
	}
}
