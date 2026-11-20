package orchestrator

import (
	"errors"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestWindow_SetCeilingEmitsWindowSignalWithHostPressureCause(t *testing.T) {
	w, events := newWindow(8, 4, 12)
	w.SetCeiling(6)
	if got := w.Size(); got != 6 {
		t.Fatalf("Size after SetCeiling(6) = %d, want 6", got)
	}
	got := events.withCode(CodeWindow)
	if len(got) != 1 {
		t.Fatalf("window events = %d, want 1", len(got))
	}
	want := map[string]string{"old": "8", "new": "6", "sized": "12", "cause": "host_pressure"}
	if !reflect.DeepEqual(got[0].Fields, want) {
		t.Errorf("fields = %v, want %v", got[0].Fields, want)
	}
}

func TestWindow_CeilingBoundsLandingGrowthAndARaiseNeverExceedsTheLandingSize(t *testing.T) {
	w, _ := newWindow(8, 4, 12)
	w.SetCeiling(6)
	landings(w, 5)
	if got := w.Size(); got != 6 {
		t.Fatalf("Size after 5 landings under a ceiling of 6 = %d, want 6", got)
	}
	w.SetCeiling(7)
	if got := w.Size(); got != 7 {
		t.Fatalf("Size after the ceiling rose to 7 = %d, want 7: landings earned 12", got)
	}
	w.SetCeiling(12)
	if got := w.Size(); got != 12 {
		t.Fatalf("Size after the ceiling rose to 12 = %d, want 12", got)
	}
}

func TestWindow_AHostRaiseDoesNotUndoTheConflictBackOff(t *testing.T) {
	w, _ := newWindow(12, 1, 12)
	for range 3 {
		w.Conflict()
	}
	w.SetCeiling(6)
	for ceiling := 7; ceiling <= 12; ceiling++ {
		w.SetCeiling(ceiling)
	}
	if got := w.Size(); got != 1 {
		t.Fatalf("Size = %d after conflicts and a full host raise with no clean landing, want 1", got)
	}
	w.CleanLanding()
	if got := w.Size(); got != 2 {
		t.Fatalf("Size = %d after one clean landing, want 2: only landings grow the window", got)
	}
}

func TestWindow_SetCeilingNeverGoesBelowTheFloorOfTheWindowItself(t *testing.T) {
	w, _ := newWindow(8, 4, 12)
	w.SetCeiling(6)
	w.SetCeiling(2)
	if got := w.Size(); got != 2 {
		t.Fatalf("Size = %d: the controller holds its own floor at window.min, the window obeys its ceiling", got)
	}
}

func TestWindow_HostCappedFollowsTheCeilingAgainstTheSizedCount(t *testing.T) {
	w, _ := newWindow(8, 4, 12)
	if w.HostCapped() {
		t.Fatal("HostCapped before any ceiling, want false")
	}
	w.SetCeiling(5)
	if !w.HostCapped() {
		t.Fatal("HostCapped = false under a ceiling of 5, want true")
	}
	w.SetCeiling(12)
	if w.HostCapped() {
		t.Error("HostCapped = true at a ceiling equal to sized, want false")
	}
}

func TestWindow_ResizeAndTheCeilingTakeTheLowerLimit(t *testing.T) {
	w, _ := newWindow(8, 1, 12)
	w.SetCeiling(6)
	w.Resize(5)
	if got := w.Size(); got != 5 {
		t.Fatalf("Size after Resize(5) under a ceiling of 6 = %d, want 5", got)
	}
	w.Resize(12)
	landings(w, 4)
	if got := w.Size(); got != 6 {
		t.Fatalf("Size after 4 landings and Resize(12) under a ceiling of 6 = %d, want 6", got)
	}
}

func TestIntake_KickWakesThePoolAndLeavesTheLimitAlone(t *testing.T) {
	in := NewIntake(2)
	in.Kick()
	select {
	case <-in.Wake():
	default:
		t.Fatal("Kick did not wake the pool")
	}
	admitted := in.Gate(func(_, _ []fleet.Todo) int { return 0 })
	inFlight := []fleet.Todo{{ID: "a"}, {ID: "b"}}
	if got := admitted(inFlight, todosNamed("c")); got != -1 {
		t.Fatalf("admit with 2 in flight under a limit of 2 = %d, want -1: Kick must not change the limit", got)
	}
}

func TestPool_HighPressureCeilingStopsBackfillButKeepsRunningCycles(t *testing.T) {
	g, events := newGatedLaunch(), &eventLog{}
	w, _ := newWindow(4, 1, 4)
	done := runPool(Pool{Target: 4, Stage: StageEnforce, Window: w, Launch: g.launch, Publish: events.publish}, todosNamed("a", "b", "c", "d", "e", "f"))
	startN(t, g, 4)

	w.SetCeiling(2)
	g.finish("a", 0)
	g.finish("b", 0)
	select {
	case id := <-g.started:
		t.Fatalf("launched %q while 2 lanes ran under a ceiling of 2", id)
	default:
	}
	g.finish("c", 0)
	if next := waitStart(t, g); next != "e" {
		t.Fatalf("next launch = %q, want e once live fell under the ceiling", next)
	}
	g.finish("d", 0)
	if next := waitStart(t, g); next != "f" {
		t.Fatalf("next launch = %q, want f", next)
	}
	finishAll(t, g, done, "e", "f")
}

func TestPool_SupplyEightAdmissibleOfTwelveCapOpensEightLanesAndRaisesNoUnderfillWarn(t *testing.T) {
	g, events := newGatedLaunch(), &eventLog{}
	w, _ := newWindow(12, 4, 12)
	ids := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	done := runPool(Pool{Target: 12, MinLanes: 4, Stage: StageEnforce, Window: w, Launch: g.launch, Publish: events.publish}, todosNamed(ids...))
	if got := startN(t, g, 8); !sameSet(got, ids) {
		t.Fatalf("launched %v, want exactly the 8 admissible items", got)
	}
	select {
	case id := <-g.started:
		t.Fatalf("launched a ninth lane %q for 8 items", id)
	default:
	}
	if got := len(events.withCode(CodePoolUnderfilled)); got != 0 {
		t.Errorf("underfill WARNs while 8 lanes ran = %d, want 0", got)
	}
	finishAll(t, g, done, ids...)
}

func TestIdleCause_HostPressureNamesAnIdleSlotHeldByTheCeiling(t *testing.T) {
	w, _ := newWindow(8, 4, 12)
	w.SetCeiling(6)
	idle := fleet.PoolTransition{Kind: fleet.PoolIdle, Live: 6, Target: 12, Pending: 3}
	empty := fleet.PoolTransition{Kind: fleet.PoolIdle, Live: 6, Target: 12, Pending: 0}
	tests := []struct {
		name string
		pool Pool
		tr   fleet.PoolTransition
		want IdleCause
	}{
		{"enforce with a ceiling", Pool{Window: w, Stage: StageEnforce}, idle, CauseHostPressure},
		{"supply empty beats host pressure", Pool{Window: w, Stage: StageEnforce}, empty, CauseSupplyEmpty},
		{"bench beats nothing: host pressure comes first", Pool{Window: w, Stage: StageEnforce, Benched: func() bool { return true }}, idle, CauseHostPressure},
		{"shadow stage ignores the window", Pool{Window: w, Stage: StageShadow}, idle, CauseAdmissionRefused},
		{"no window", Pool{Stage: StageEnforce}, idle, CauseAdmissionRefused},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.pool.idleCause(tc.tr); got != tc.want {
				t.Errorf("idleCause = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestUnderfill_HostPressureIdleKeepsTheIdleEventAndSkipsTheWarn(t *testing.T) {
	w, _ := newWindow(8, 4, 12)
	w.SetCeiling(6)
	events := &eventLog{}
	p := Pool{MinLanes: 8, Window: w, Stage: StageEnforce, Publish: events.publish}
	p.transition(fleet.PoolTransition{Kind: fleet.PoolIdle, Live: 6, Target: 12, Pending: 3})
	if got := len(events.withCode(CodePoolUnderfilled)); got != 0 {
		t.Errorf("underfill WARNs = %d, want 0: a pressure cap below min_lanes is by design", got)
	}
	if got := fieldsOf(events, CodePoolIdle, "cause"); !reflect.DeepEqual(got, []string{"host_pressure"}) {
		t.Errorf("idle causes = %v, want [host_pressure]", got)
	}
}

func TestHostPressureCodesAreRegisteredToTheLoopModule(t *testing.T) {
	for _, c := range []signalcenter.Code{CodeHostPressure, CodeHostBlind} {
		if m, ok := signalcenter.IsRegistered(c); !ok || m != signalcenter.ModuleLoop {
			t.Errorf("IsRegistered(%q) = %q, %v, want the loop module", c, m, ok)
		}
	}
	if CodeHostPressure != "LOOP_ORCH_HOST_PRESSURE" || CodeHostBlind != "LOOP_HOST_PRESSURE_BLIND" {
		t.Errorf("codes = %q, %q", CodeHostPressure, CodeHostBlind)
	}
}

func TestPublish_HostPressureCarriesTheDecisionAndTheReading(t *testing.T) {
	events := &eventLog{}
	Publish(events.publish).HostPressure(HostPressureEvent{Old: 12, New: 6, Sized: 12, Cause: "pressure_high", Stage: "enforce", Applied: true,
		Load1: 28, CPUs: 14, LoadPerCore: 2, MemAvailablePct: 80, SwapUsedMB: 10, Score: 1.5})
	got := events.withCode(CodeHostPressure)
	if len(got) != 1 || got[0].Severity != signalcenter.SeverityInfo || got[0].Module != signalcenter.ModuleLoop {
		t.Fatalf("events = %+v, want one loop INFO", got)
	}
	want := map[string]string{"old": "12", "new": "6", "sized": "12", "cause": "pressure_high", "stage": "enforce", "applied": "true",
		"load1": "28", "cpus": "14", "load_per_core": "2", "mem_available_pct": "80", "swap_used_mb": "10", "score": "1.5"}
	if !reflect.DeepEqual(got[0].Fields, want) {
		t.Errorf("fields = %v, want %v", got[0].Fields, want)
	}
	Publish(nil).HostPressure(HostPressureEvent{})
}

func TestPublish_HostBlindWarnsWithTheErrorAndTheOS(t *testing.T) {
	events := &eventLog{}
	Publish(events.publish).HostBlind(errors.New("vm_stat failed"), "darwin")
	got := events.withCode(CodeHostBlind)
	if len(got) != 1 || got[0].Severity != signalcenter.SeverityWarn {
		t.Fatalf("events = %+v, want one WARN", got)
	}
	if want := map[string]string{"error": "vm_stat failed", "os": "darwin"}; !reflect.DeepEqual(got[0].Fields, want) {
		t.Errorf("fields = %v, want %v", got[0].Fields, want)
	}
}

func TestWindow_LandingAndConflictUnderACeilingPublishNothingWhileTheAdmittedWidthHolds(t *testing.T) {
	w, events := newWindow(12, 1, 12)
	w.SetCeiling(6)
	w.CleanLanding()
	w.Conflict()
	if got := w.Size(); got != 6 {
		t.Fatalf("Size = %d, want 6: the ceiling holds the admitted width", got)
	}
	if got := len(events.withCode(CodeWindow)); got != 1 {
		t.Fatalf("window events = %d, want 1 (the ceiling only): a landing or a conflict that leaves the admitted width unchanged publishes nothing", got)
	}
}

func TestPublish_HostPressureChangeEmitsOnlyWhenTheCapMoves(t *testing.T) {
	events := &eventLog{}
	publish := Publish(events.publish)
	if publish.HostPressureChange(HostPressureEvent{Old: 8, New: 8, Cause: "hold"}) {
		t.Fatal("an unchanged cap reported a change")
	}
	if got := events.withCode(CodeHostPressure); len(got) != 0 {
		t.Fatalf("an unchanged cap emitted %d events, want 0", len(got))
	}
	if !publish.HostPressureChange(HostPressureEvent{Old: 8, New: 4, Cause: "pressure_high"}) {
		t.Fatal("a moved cap reported no change")
	}
	got := events.withCode(CodeHostPressure)
	if len(got) != 1 || got[0].Fields["new"] != "4" {
		t.Fatalf("events = %+v, want one event with new=4", got)
	}
}
