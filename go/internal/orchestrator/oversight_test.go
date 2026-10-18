package orchestrator

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/reader"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/wake"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type callLog struct {
	mu    sync.Mutex
	calls []string
}

func (c *callLog) note(call string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, call)
}

func (c *callLog) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.calls...)
}

func (c *callLog) count(call string) int {
	n := 0
	for _, got := range c.snapshot() {
		if got == call {
			n++
		}
	}
	return n
}

type scriptWaiter struct {
	log     *callLog
	wakes   chan wake.Wake
	waiting chan struct{}
}

func (w *scriptWaiter) Arm(wake.Targets) error {
	w.log.note("arm")
	return nil
}

func (w *scriptWaiter) Wait(ctx context.Context, _ time.Time) (wake.Wake, error) {
	w.log.note("wait")
	w.waiting <- struct{}{}
	select {
	case wk := <-w.wakes:
		return wk, nil
	case <-ctx.Done():
		return wake.Wake{}, ctx.Err()
	}
}

var _ LandingObserver = (*landingLog)(nil)

type landingLog struct {
	mu              sync.Mutex
	clean, conflict int
}

func (l *landingLog) CleanLanding() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.clean++
}

func (l *landingLog) Conflict() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.conflict++
}

func (l *landingLog) counts() (clean, conflict int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.clean, l.conflict
}

type fixture struct {
	t       *testing.T
	root    string
	log     *callLog
	waiter  *scriptWaiter
	clock   *fakeClock
	events  *eventLog
	leases  *Leases
	intake  *Intake
	landing *landingLog
	launch  *gatedLaunch
	graph   *admission.ModuleGraph
	over    *Oversight
	cancel  context.CancelFunc
	done    chan error
	logs    map[string]*channel.Log
	seq     uint64
	locates atomic.Int64

	mu     sync.Mutex
	cycles map[string]int
	sizing func() (int, []string)
	lanes  map[string]chan fleet.Result
}

func newFixture(t *testing.T, stage string) *fixture {
	t.Helper()
	f := &fixture{t: t, root: t.TempDir(), log: &callLog{}, clock: newFakeClock(), events: &eventLog{}, intake: NewIntake(4),
		landing: &landingLog{}, launch: newGatedLaunch(), graph: cellGraph(t, "inboxrank", "quotastate"),
		logs: map[string]*channel.Log{}, cycles: map[string]int{}, lanes: map[string]chan fleet.Result{},
		sizing: func() (int, []string) { return 4, nil }}
	f.waiter = &scriptWaiter{log: f.log, wakes: make(chan wake.Wake, 16), waiting: make(chan struct{}, 64)}
	publish := func(e signalcenter.Event) {
		f.log.note("emit:" + string(e.Code))
		f.events.publish(e)
	}
	f.leases = NewLeases(LeaseConfig{Stage: stage, Deadline: 5 * time.Hour, Stall: 20 * time.Minute}, f.clock, silentHeartbeat, publish)
	f.over = &Oversight{Stage: stage, Leases: f.leases, Intake: f.intake, Landing: f.landing, Publish: publish,
		Locate: func(l Lane) (int, bool) {
			f.locates.Add(1)
			f.mu.Lock()
			defer f.mu.Unlock()
			c, ok := f.cycles[l.Key]
			return c, ok
		},
		Resize: func() (int, []string) {
			f.mu.Lock()
			size := f.sizing
			f.mu.Unlock()
			return size()
		}}
	return f
}

func (f *fixture) setCycle(lane string, cycle int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cycles[lane] = cycle
}

func (f *fixture) setSizing(size func() (int, []string)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sizing = size
}

func (f *fixture) addLane(id string, cycle int) {
	f.t.Helper()
	if cycle > 0 {
		f.setCycle(id, cycle)
	}
	f.lanes[id] = runLane(f.leases.Watch(f.launch.launch, nil), id)
	waitStart(f.t, f.launch)
}

func (f *fixture) endLane(id string) {
	f.t.Helper()
	done, ok := f.lanes[id]
	if !ok {
		return
	}
	delete(f.lanes, id)
	f.launch.finish(id, 0)
	<-done
}

func (f *fixture) start() {
	f.t.Helper()
	rd, err := Subscribe(f.root, epoch.Format(time.RFC3339Nano), reader.Ports{Waiter: f.waiter, StartOf: func(int) (string, error) { return "", syscall.ESRCH }, Now: time.Now})
	if err != nil {
		f.t.Fatal(err)
	}
	f.over.Events = rd
	ctx, cancel := context.WithCancel(context.Background())
	f.cancel, f.done = cancel, make(chan error, 1)
	go func() { f.done <- f.over.Run(ctx) }()
	f.t.Cleanup(func() {
		for id := range f.lanes {
			f.endLane(id)
		}
		f.stop()
	})
	<-f.waiter.waiting
}

func (f *fixture) stop() error {
	if f.cancel == nil {
		return nil
	}
	f.cancel()
	err := <-f.done
	f.cancel = nil
	return err
}

func (f *fixture) logFor(name string) *channel.Log {
	f.t.Helper()
	if l, ok := f.logs[name]; ok {
		return l
	}
	l, err := channel.New(f.root, name, channel.Config{LockDeadline: time.Minute})
	if err != nil {
		f.t.Fatal(err)
	}
	f.logs[name] = l
	return l
}

func (f *fixture) stamp(e signalcenter.Event) signalcenter.Event {
	f.seq++
	e.SchemaVersion, e.Seq, e.PID, e.Origin = signalcenter.SchemaVersion, f.seq, 4242, "test"
	if e.TS == "" {
		e.TS = epoch.Add(time.Duration(f.seq) * time.Second).Format(time.RFC3339Nano)
	}
	if e.Module == "" {
		e.Module = signalcenter.ModuleLoop
	}
	if e.Severity == "" {
		e.Severity = signalcenter.SeverityInfo
	}
	e.Reason = "r"
	return e
}

func (f *fixture) put(name string, e signalcenter.Event) {
	f.t.Helper()
	f.putIn(e, name)
}

func (f *fixture) putIn(e signalcenter.Event, names ...string) {
	f.t.Helper()
	e = f.stamp(e)
	for _, name := range names {
		if _, err := f.logFor(name).Append([]channel.Record{{Source: "loop", Signal: &e}}); err != nil {
			f.t.Fatal(err)
		}
	}
}

func (f *fixture) settle() {
	f.t.Helper()
	f.waiter.wakes <- wake.Wake{Changed: true}
	<-f.waiter.waiting
}

func (f *fixture) send(name string, e signalcenter.Event) {
	f.t.Helper()
	f.put(name, e)
	f.settle()
}

func (f *fixture) actions() []signalcenter.Event { return f.events.withCode(CodeOversightAction) }

func (f *fixture) admits(inFlight []fleet.Todo, pending ...fleet.Todo) bool {
	return f.intake.Gate(AdmitByCell(admission.Admitter{Graph: f.graph}))(inFlight, pending) == 0
}

func sealedEvent(cycle int, verdict string) signalcenter.Event {
	return signalcenter.Event{Cycle: cycle, Module: signalcenter.ModuleOrchestrator, Kind: signalcenter.KindCycleSealed,
		Fields: map[string]string{cyclestate.SealedVerdictField: verdict}}
}

func landedEvent(cycle int) signalcenter.Event {
	return signalcenter.Event{Cycle: cycle, Module: signalcenter.ModuleShip, Kind: signalcenter.KindShipLanded}
}

func phaseEvent(cycle int, kind signalcenter.Kind, at time.Time) signalcenter.Event {
	return signalcenter.Event{Cycle: cycle, Module: signalcenter.ModuleOrchestrator, Kind: kind, TS: at.Format(time.RFC3339Nano)}
}

func rebaseEvent(cycle int, code shiperr.ShipErrorCode) signalcenter.Event {
	return signalcenter.Event{Cycle: cycle, Module: signalcenter.ModuleShip, Kind: signalcenter.KindShipError,
		Severity: signalcenter.SeverityWarn, Code: shiperr.SignalCode(code)}
}

func stalledEvent(cycle int) signalcenter.Event {
	return signalcenter.Event{Cycle: cycle, Module: signalcenter.ModuleLiveness, Kind: signalcenter.KindPaneLiveness,
		Severity: signalcenter.SeverityWarn, Code: CodePhaseStalled}
}

func benchEvent(code signalcenter.Code) signalcenter.Event {
	return signalcenter.Event{Module: signalcenter.ModuleBridge, Kind: signalcenter.KindBridgeWarning, Severity: signalcenter.SeverityWarn, Code: code}
}

func haltEvent() signalcenter.Event {
	return signalcenter.Event{Module: signalcenter.ModuleLoop, Kind: signalcenter.KindLoopHalt, Severity: signalcenter.SeverityIncident,
		Code: "LOOP_HALT", Fields: map[string]string{"stop_reason": "loop_operator_brake"}}
}

func TestOrchestrator_WakesOnlyOnEvents(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.addLane("A", 41)
	f.addLane("B", 42)
	f.put("cycle", sealedEvent(41, cyclestate.VerdictFAIL))
	act := "emit:" + string(CodeOversightAction)

	f.start()
	if got, want := f.log.snapshot(), []string{"arm", act, "wait"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("first catch-up = %v, want arm, then the read of the record that was already there, then wait", got)
	}

	f.settle()
	if got, want := f.log.snapshot(), []string{"arm", act, "wait", "arm", "wait"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("after a wake with no record = %v, want arm and wait again with no action: a wake with no event acts on nothing", got)
	}

	f.send("ship", landedEvent(42))
	want := []string{"arm", act, "wait", "arm", "wait", "arm", act, "wait"}
	if got := f.log.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("after a landing = %v, want %v: arm, read, wait in that order for every wake", got, want)
	}
	if n := f.clock.armed(); n != 1 {
		t.Errorf("armed timers = %d, want the one deadline timer of the leases and none of the oversight", n)
	}
	if err := f.stop(); !errors.Is(err, context.Canceled) {
		t.Errorf("Run = %v, want the cancellation of its context", err)
	}
}

func refusals(admit Admit) (Admit, chan struct{}) {
	refused := make(chan struct{}, 16)
	return func(inFlight, pending []fleet.Todo) int {
		got := admit(inFlight, pending)
		if got < 0 {
			refused <- struct{}{}
		}
		return got
	}, refused
}

func TestOrchestrator_ShipLandedFreesTheCells(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.setCycle("a", 41)
	f.setCycle("b", 42)
	admit, refused := refusals(AdmitByCell(admission.Admitter{Graph: f.graph}))
	pool := Pool{Target: 3, Admit: admit, Launch: f.launch.launch, Publish: f.events.publish, Leases: f.leases, Intake: f.intake}
	done := runPool(pool, []fleet.Todo{todoIn("a", "inboxrank"), todoIn("b", "quotastate"), todoIn("d", "inboxrank")})
	first := []string{waitStart(t, f.launch), waitStart(t, f.launch)}
	if !sameSet(first, []string{"a", "b"}) {
		t.Fatalf("first fill = %v, want a and b: d waits for the inboxrank cell of a", first)
	}
	<-refused
	f.start()

	f.send("cycle", landedEvent(41))

	if next := waitStart(t, f.launch); next != "d" {
		t.Fatalf("after the landing of a the open slot took %q, want d while a still runs its post-ship phases", next)
	}
	got := f.actions()
	if len(got) != 1 || got[0].Fields["lane"] != "a" || got[0].Fields["actions"] != "free_cells,count_clean_landing,backfill" || got[0].Fields["applied"] != "true" {
		t.Errorf("action records = %+v, want one applied record for lane a that frees its cells, counts a clean landing and backfills", got)
	}
	if clean, conflict := f.landing.counts(); clean != 1 || conflict != 0 {
		t.Errorf("landing counts = %d clean, %d conflict, want 1 clean", clean, conflict)
	}
	if n := len(f.events.withCode(CodeLaneBackfilled)); n != 1 {
		t.Errorf("backfilled events = %d, want 1 (d)", n)
	}
	for _, id := range []string{"a", "b", "d"} {
		f.launch.finish(id, 0)
	}
	<-done
}

func TestOversight_ASealedFAILFreesTheCellsOfItsLaneAndAPASSOnlyRecordsTheVerdict(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.addLane("A", 41)
	f.addLane("B", 42)
	f.start()
	inFlight := []fleet.Todo{todoIn("A", "inboxrank"), todoIn("B", "quotastate")}

	f.send("cycle", sealedEvent(41, cyclestate.VerdictPASS))
	if f.admits(inFlight, todoIn("d", "inboxrank")) {
		t.Fatal("a PASS seal freed the cells of A, want them held until the landing")
	}
	f.send("cycle", sealedEvent(42, cyclestate.VerdictFAIL))

	if f.admits(inFlight, todoIn("d", "inboxrank")) || !f.admits(inFlight, todoIn("e", "quotastate")) {
		t.Error("after the FAIL seal of B, want the cells of A held and the cells of B free")
	}
	got := f.actions()
	if len(got) != 2 || got[0].Fields["actions"] != "record_verdict" || got[0].Fields["verdict"] != cyclestate.VerdictPASS ||
		got[1].Fields["actions"] != "record_verdict,free_cells" || got[1].Fields["verdict"] != cyclestate.VerdictFAIL || got[1].Fields["lane"] != "B" {
		t.Errorf("action records = %+v, want the PASS verdict recorded alone and the FAIL verdict recorded with a free of B", got)
	}
}

func TestOversight_AShadowComputesEveryActionAndChangesNothing(t *testing.T) {
	f := newFixture(t, StageShadow)
	f.addLane("A", 41)
	f.setSizing(func() (int, []string) { return 2, []string{"claude"} })
	f.start()
	inFlight := []fleet.Todo{todoIn("A", "inboxrank"), {ID: "x"}, {ID: "y"}}

	f.send("cycle", sealedEvent(41, cyclestate.VerdictFAIL))
	f.send("ship", rebaseEvent(41, shiperr.CodeGitFleetRebaseNeeded))
	f.send("cycle", landedEvent(41))
	f.send("errors", benchEvent(CodeUsageEvidence))
	f.send("loop", haltEvent())

	if f.admits(inFlight[:1], todoIn("d", "inboxrank")) {
		t.Error("the shadow freed the cells of A, want every cell held")
	}
	if f.intake.Gate(firstOf)(inFlight, []fleet.Todo{{ID: "z"}}) != 0 {
		t.Error("the shadow lowered the limit, want 3 lanes in flight under the limit of 4 to admit")
	}
	if f.intake.Stopped() {
		t.Error("the shadow stopped admission, want it open")
	}
	select {
	case <-f.intake.Wake():
		t.Error("the shadow kicked the pool, want no kick")
	default:
	}
	if clean, conflict := f.landing.counts(); clean+conflict != 0 {
		t.Errorf("landing counts = %d clean, %d conflict, want none: the window is fed at enforce only", clean, conflict)
	}
	for _, a := range f.actions() {
		if a.Fields["applied"] != "false" || a.Fields["stage"] != StageShadow {
			t.Errorf("action record %+v, want applied false at stage shadow", a.Fields)
		}
	}
	if got := len(f.actions()); got != 4 {
		t.Errorf("action records = %d, want one for each of the seal, the landing, the bench and the halt: the rebase code only marks its lane", got)
	}
	if paused := f.events.withCode(CodeFamilyPaused); len(paused) != 1 || paused[0].Fields["family"] != "claude" || paused[0].Fields["applied"] != "false" {
		t.Errorf("family paused events = %+v, want one record for claude with applied false", paused)
	}
}

func TestOversight_ALandingIsAConflictWhenItsLaneStreamHeldARebaseCode(t *testing.T) {
	for _, code := range []shiperr.ShipErrorCode{shiperr.CodeGitFleetRebaseNeeded, shiperr.CodeGitFleetRebaseConflict, shiperr.CodeAuditBindingHeadMoved} {
		f := newFixture(t, StageEnforce)
		f.addLane("A", 41)
		f.addLane("B", 42)
		f.start()

		f.send("ship", rebaseEvent(41, code))
		f.send("ship", rebaseEvent(99, code))
		f.send("cycle", landedEvent(41))
		f.send("cycle", landedEvent(42))
		f.send("cycle", landedEvent(99))

		if clean, conflict := f.landing.counts(); clean != 1 || conflict != 1 {
			t.Errorf("%s: landing counts = %d clean, %d conflict, want 1 and 1 (A rebased, B did not, cycle 99 is not ours)", code, clean, conflict)
		}
		got := f.actions()
		if len(got) != 2 || got[0].Fields["actions"] != "free_cells,count_conflict,backfill" || got[1].Fields["actions"] != "free_cells,count_clean_landing,backfill" {
			t.Errorf("%s: action records = %+v, want a conflict record for A and a clean record for B", code, got)
		}
	}
}

func TestOversight_ALandingWithNoWindowObserverIsStillHandled(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.over.Landing = nil
	f.addLane("A", 41)
	f.addLane("B", 42)
	f.start()

	f.send("ship", rebaseEvent(41, shiperr.CodeGitFleetRebaseNeeded))
	f.send("cycle", landedEvent(41))
	f.send("cycle", landedEvent(42))

	got := f.actions()
	if len(got) != 2 || got[0].Fields["actions"] != "free_cells,count_conflict,backfill" || got[1].Fields["actions"] != "free_cells,count_clean_landing,backfill" {
		t.Errorf("action records = %+v, want both landings handled with no observer set", got)
	}
}

func TestOversight_PhaseEventsAndObserverReportsMoveTheLaneHeartbeat(t *testing.T) {
	for name, e := range map[string]signalcenter.Event{
		"phase.dispatched": phaseEvent(41, signalcenter.KindPhaseDispatched, epoch.Add(15*time.Minute)),
		"phase.outcome":    phaseEvent(41, signalcenter.KindPhaseOutcome, epoch.Add(15*time.Minute)),
		"phase.aborted":    phaseEvent(41, signalcenter.KindPhaseAborted, epoch.Add(15*time.Minute)),
		"observer report":  {Cycle: 41, Module: signalcenter.ModuleObserver, Kind: signalcenter.KindObserverWarning, TS: epoch.Add(15 * time.Minute).Format(time.RFC3339Nano)},
	} {
		f := newFixture(t, StageEnforce)
		f.addLane("A", 41)
		f.start()
		f.clock.Advance(15 * time.Minute)

		f.send("cycle", e)
		f.clock.Advance(6 * time.Minute)
		if got := f.events.withCode(CodeLaneStalled); len(got) != 0 {
			t.Fatalf("%s: stall events at 21m with a signal at 15m = %+v, want none", name, got)
		}
		f.clock.Advance(15 * time.Minute)
		if got := f.events.withCode(CodeLaneStalled); len(got) != 1 {
			t.Errorf("%s: stall events at 36m = %+v, want one: 20m past the signal at 15m", name, got)
		}
		if got := f.actions(); len(got) != 0 {
			t.Errorf("%s: action records = %+v, want none: a heartbeat is an observation, not an action", name, got)
		}
	}
}

func TestOversight_AHeartbeatWithNoTimeOrNoLaneMovesNothing(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.addLane("A", 41)
	f.start()

	bad := phaseEvent(41, signalcenter.KindPhaseOutcome, epoch)
	bad.TS = "not a time"
	f.put("cycle", bad)
	f.put("cycle", phaseEvent(99, signalcenter.KindPhaseOutcome, epoch.Add(15*time.Minute)))
	f.put("cycle", phaseEvent(0, signalcenter.KindPhaseOutcome, epoch.Add(15*time.Minute)))
	f.settle()
	f.clock.Advance(21 * time.Minute)

	if got := f.events.withCode(CodeLaneStalled); len(got) != 1 {
		t.Errorf("stall events at 21m = %+v, want one: no event moved the heartbeat of A (bad time, another cycle, no cycle)", got)
	}
}

func TestOversight_APhaseStalledEventPublishesTheLaneStallOnce(t *testing.T) {
	for name, e := range map[string]signalcenter.Event{
		"a tmux phase":     stalledEvent(41),
		"a headless phase": {Cycle: 41, Module: signalcenter.ModuleObserver, Kind: signalcenter.KindObserverWarning, Severity: signalcenter.SeverityWarn, Code: CodePhaseStalled},
	} {
		f := newFixture(t, StageEnforce)
		f.addLane("A", 41)
		f.start()

		f.send("errors", e)
		if got := f.events.withCode(CodeLaneStalled); len(got) != 1 || got[0].Fields["todo"] != "A" {
			t.Fatalf("%s: stall events at once = %+v, want one for A: a stall report is not a heartbeat even from the observer module", name, got)
		}
		f.send("errors", e)
		f.send("errors", stalledEvent(99))
		f.clock.Advance(25 * time.Minute)

		if got := f.events.withCode(CodeLaneStalled); len(got) != 1 {
			t.Errorf("%s: stall events = %+v, want exactly one: the event, a repeat and the lease timer share one mark", name, got)
		}
	}
}

func TestOversight_ABenchWithNoFallbackPausesTheFamilyOnceAndShrinksTheLimit(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.start()
	two := []fleet.Todo{{ID: "x"}, {ID: "y"}}
	next := f.intake.Gate(firstOf)

	f.setSizing(func() (int, []string) { return 2, []string{"claude"} })
	f.send("errors", benchEvent(CodeUsageEvidence))
	if next(two, []fleet.Todo{{ID: "z"}}) != -1 {
		t.Fatal("2 lanes in flight under a limit of 2 still admit, want the bench to stop new admission")
	}
	f.send("errors", benchEvent(CodePaneExhausted))
	f.setSizing(func() (int, []string) { return 4, nil })
	f.send("errors", benchEvent(CodeUsageEvidence))

	if next(two, []fleet.Todo{{ID: "z"}}) != 0 {
		t.Error("after the family came back the limit stayed low, want admission open again")
	}
	paused := f.events.withCode(CodeFamilyPaused)
	if len(paused) != 1 || paused[0].Fields["family"] != "claude" || paused[0].Fields["target"] != "2" || paused[0].Fields["applied"] != "true" {
		t.Errorf("family paused events = %+v, want exactly one for claude: a second bench event of a paused family adds none", paused)
	}
	if got := f.actions(); len(got) != 3 || got[0].Fields["actions"] != "resize" || got[0].Fields["target"] != "2" || got[2].Fields["target"] != "4" {
		t.Errorf("action records = %+v, want a resize record for each of the 3 bench events", got)
	}
	f.setSizing(func() (int, []string) { return 3, []string{"agy-claude"} })
	f.send("errors", benchEvent(CodeUsageEvidence))
	if again := f.events.withCode(CodeFamilyPaused); len(again) != 2 || again[1].Fields["family"] != "agy-claude" {
		t.Errorf("family paused events after a new family = %+v, want agy-claude as the second", again)
	}
}

func TestOversight_AHaltOnTheLoopChannelStopsAllAdmissionAndLanesInFlightFinish(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.addLane("A", 41)
	f.start()

	f.send("loop", haltEvent())

	if !f.intake.Stopped() {
		t.Fatal("the halt did not stop admission")
	}
	if got := f.actions(); len(got) != 1 || got[0].Fields["actions"] != "stop_admission" || got[0].Fields["applied"] != "true" {
		t.Errorf("action records = %+v, want one applied stop_admission record", got)
	}
	f.endLane("A")
}

func TestOversight_EventsOfOtherCyclesGapsAndUnmappedKindsActOnNothing(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.addLane("A", 41)
	f.start()

	f.put("cycle", sealedEvent(99, cyclestate.VerdictFAIL))
	f.put("cycle", landedEvent(99))
	f.put("cycle", signalcenter.Event{Cycle: 41, Module: signalcenter.ModuleLoop, Kind: signalcenter.KindLoopWave})
	if _, err := f.logFor("errors").Append([]channel.Record{{Source: "loop", Gap: &channel.Gap{Reason: channel.ReasonQueueFull, Severity: signalcenter.SeverityWarn, PID: 1, FirstSeq: 1, LastSeq: 1, Dropped: 1}}}); err != nil {
		t.Fatal(err)
	}
	f.settle()

	if got := f.actions(); len(got) != 0 {
		t.Errorf("action records = %+v, want none for another loop's cycle, a gap and a kind with no mapping", got)
	}
	if f.log.count("emit:"+string(CodeLaneStalled)) != 0 {
		t.Error("an event of another cycle reported a stall")
	}
}

func TestOversight_ALateEventOfAnEarlierLifeOfTheSameTodoIsCountedAndFreesNothingOfTheNewLife(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.addLane("A", 41)
	f.start()
	f.send("cycle", phaseEvent(41, signalcenter.KindPhaseDispatched, epoch))

	f.endLane("A")
	f.clock.Advance(time.Minute)
	f.addLane("A", 45)
	f.send("cycle", landedEvent(41))

	got := f.actions()
	if len(got) != 1 || got[0].Fields["actions"] != "count_clean_landing" || got[0].Fields["lane"] != "A" || got[0].Fields["cycle"] != "41" {
		t.Fatalf("action records = %+v, want the landing of cycle 41 counted for its own life and nothing freed", got)
	}
	if f.admits([]fleet.Todo{todoIn("A", "inboxrank")}, todoIn("d", "inboxrank")) {
		t.Error("the late landing of cycle 41 freed the cells of the new life of A (cycle 45)")
	}
	select {
	case <-f.intake.Wake():
		t.Error("the late landing kicked the pool, want no kick: the old life already left the pool")
	default:
	}
	f.send("cycle", landedEvent(45))
	if got := f.actions(); len(got) != 2 || got[1].Fields["actions"] != "free_cells,count_clean_landing,backfill" || got[1].Fields["cycle"] != "45" {
		t.Errorf("action records = %+v, want the landing of cycle 45 handled in full for the new life of A", got)
	}
}

func TestOversight_AStallReportAndAHeartbeatOfAnEarlierLifeNeverTouchTheNewLife(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.addLane("A", 41)
	f.start()
	f.send("cycle", phaseEvent(41, signalcenter.KindPhaseDispatched, epoch))
	f.endLane("A")
	f.clock.Advance(time.Minute)
	f.addLane("A", 45)

	f.send("errors", stalledEvent(41))
	if got := f.events.withCode(CodeLaneStalled); len(got) != 0 {
		t.Fatalf("stall events = %+v, want none: the report names the first life of A, and the new life is not stalled", got)
	}
	f.clock.Advance(19 * time.Minute)
	f.send("cycle", phaseEvent(41, signalcenter.KindPhaseOutcome, epoch.Add(20*time.Minute)))
	f.clock.Advance(2 * time.Minute)

	if got := f.events.withCode(CodeLaneStalled); len(got) != 1 || got[0].Fields["todo"] != "A" {
		t.Errorf("stall events at 22m = %+v, want the stall of the new life at 21m: a signal of the first life must not move its heartbeat", got)
	}
}

func TestOversight_ASealAndALandingAfterTheirLaneLeftAreCountedAndRecorded(t *testing.T) {
	for name, tc := range map[string]struct {
		rebased bool
		want    string
		clean   int
		conflct int
	}{
		"a clean landing":         {want: "count_clean_landing", clean: 1},
		"a landing in a conflict": {rebased: true, want: "count_conflict", conflct: 1},
	} {
		f := newFixture(t, StageEnforce)
		f.addLane("A", 41)
		f.start()
		f.send("cycle", phaseEvent(41, signalcenter.KindPhaseDispatched, epoch))
		if tc.rebased {
			f.send("ship", rebaseEvent(41, shiperr.CodeGitFleetRebaseNeeded))
		}

		f.endLane("A")
		f.send("cycle", landedEvent(41))
		f.send("cycle", sealedEvent(41, cyclestate.VerdictFAIL))

		got := f.actions()
		if len(got) != 2 || got[0].Fields["actions"] != tc.want || got[1].Fields["actions"] != "record_verdict" || got[1].Fields["verdict"] != cyclestate.VerdictFAIL {
			t.Fatalf("%s: action records = %+v, want the landing counted and the verdict recorded, with no cells to free", name, got)
		}
		if clean, conflict := f.landing.counts(); clean != tc.clean || conflict != tc.conflct {
			t.Errorf("%s: landing counts = %d clean, %d conflict, want %d and %d: the last events of a lane arrive after it left", name, clean, conflict, tc.clean, tc.conflct)
		}
		select {
		case <-f.intake.Wake():
			t.Errorf("%s: an event of a lane that left kicked the pool", name)
		default:
		}
	}
}

func TestOversight_ASealedCycleIsForgottenSoItsMappingDoesNotOutliveItsLastEvent(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.addLane("A", 41)
	f.start()

	f.send("cycle", phaseEvent(41, signalcenter.KindPhaseDispatched, epoch))
	if _, kept := f.over.cycles[41]; !kept {
		t.Fatal("the cycle of A is not mapped after its first event")
	}
	f.send("cycle", sealedEvent(41, cyclestate.VerdictPASS))

	if _, kept := f.over.cycles[41]; kept {
		t.Error("the mapping of cycle 41 outlived its sealed event")
	}
}

func TestOversight_AForeignCycleIsLocatedOncePerChangeOfTheLiveSet(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.addLane("A", 41)
	f.addLane("B", 42)
	f.start()
	f.send("cycle", phaseEvent(41, signalcenter.KindPhaseDispatched, epoch))
	base := f.locates.Load()

	for range 3 {
		f.send("cycle", phaseEvent(98, signalcenter.KindPhaseDispatched, epoch))
	}
	if got := f.locates.Load() - base; got != 2 {
		t.Fatalf("Locate calls for 3 events of a foreign cycle = %d, want 2: one scan of the 2 lanes, then the negative cache", got)
	}

	f.endLane("B")
	f.send("cycle", phaseEvent(98, signalcenter.KindPhaseDispatched, epoch))
	f.send("cycle", phaseEvent(98, signalcenter.KindPhaseDispatched, epoch))
	if got := f.locates.Load() - base; got != 3 {
		t.Errorf("Locate calls after a lane left = %d, want 3: one more scan of the 1 lane that stays", got)
	}
}

func TestOversight_AnUnlocatedLaneIsLookedForAgainSoItsFirstEventIsNeverLostToTheNegativeCache(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.addLane("A", 0)
	f.start()

	f.send("cycle", phaseEvent(99, signalcenter.KindPhaseDispatched, epoch))
	f.send("cycle", phaseEvent(99, signalcenter.KindPhaseDispatched, epoch))
	if got := f.locates.Load(); got != 2 {
		t.Fatalf("Locate calls while A has no cycle = %d, want 2: a miss is not cached while a lane might own the cycle", got)
	}
	f.setCycle("A", 41)
	f.send("cycle", sealedEvent(41, cyclestate.VerdictPASS))

	if got := f.actions(); len(got) != 1 || got[0].Fields["lane"] != "A" {
		t.Errorf("action records = %+v, want the first event of cycle 41 found for A once its pin exists", got)
	}
}

func TestOversight_StampedNeverMutatesItsInput(t *testing.T) {
	o := &Oversight{Stage: StageEnforce}
	in := map[string]string{"family": "claude"}

	out := o.stamped(in)
	out["family"] = "changed"

	if len(in) != 1 || in["family"] != "claude" {
		t.Errorf("input = %v, want it untouched: stamped must clone", in)
	}
	if out["stage"] != StageEnforce || out["applied"] != "true" {
		t.Errorf("output = %v, want the stage and applied keys added", out)
	}
	if shadow := (&Oversight{Stage: StageShadow}).stamped(nil); shadow["applied"] != "false" || len(shadow) != 2 {
		t.Errorf("stamped(nil) at shadow = %v, want stage and applied false", shadow)
	}
}

func TestOversight_OneLandingEventInTwoChannelsIsOneActionAndOneCount(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.addLane("A", 41)
	f.start()

	f.putIn(landedEvent(41), "cycle", "ship")
	f.settle()

	if got := f.actions(); len(got) != 1 {
		t.Errorf("action records = %+v, want 1: the reader delivers an event once, whatever the number of channels that hold it", got)
	}
	if clean, conflict := f.landing.counts(); clean != 1 || conflict != 0 {
		t.Errorf("landing counts = %d clean, %d conflict, want 1 clean", clean, conflict)
	}
}

func TestChannelRoutes_ListOnlyTheKindsThatTheOversightMaps(t *testing.T) {
	mapped := []string{
		string(signalcenter.KindPhaseDispatched), string(signalcenter.KindPhaseOutcome), string(signalcenter.KindPhaseAborted),
		string(signalcenter.KindCycleSealed), string(signalcenter.KindShipLanded),
	}
	listed := strings.Split(strings.TrimPrefix(channelRoutes["cycle"], "kind="), ",")
	slices.Sort(mapped)
	slices.Sort(listed)

	if !reflect.DeepEqual(listed, mapped) {
		t.Errorf("the cycle route lists %v, want the kinds the oversight maps %v", listed, mapped)
	}
}

func TestOversight_RunReturnsTheErrorOfItsEventSource(t *testing.T) {
	f := newFixture(t, StageEnforce)
	boom := errors.New("the reader is gone")
	f.over.Events = failingSource{boom}

	err := f.over.Run(context.Background())
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "oversight") {
		t.Errorf("Run = %v, want the error of the source, wrapped with the oversight context", err)
	}
}

func TestOversight_RunRefusesToStartWithAMissingPort(t *testing.T) {
	for name, missing := range map[string]func(o *Oversight){
		"events": func(o *Oversight) { o.Events = nil },
		"leases": func(o *Oversight) { o.Leases = nil },
		"intake": func(o *Oversight) { o.Intake = nil },
		"locate": func(o *Oversight) { o.Locate = nil },
		"resize": func(o *Oversight) { o.Resize = nil },
	} {
		f := newFixture(t, StageEnforce)
		f.over.Events = failingSource{errors.New("not reached")}
		missing(f.over)

		if err := f.over.Run(context.Background()); !errors.Is(err, errOversightPorts) {
			t.Errorf("without %s: Run = %v, want the error that names the ports before any event is read", name, err)
		}
	}
}

type failingSource struct{ err error }

func (s failingSource) Next(context.Context, time.Time) ([]reader.Item, error) { return nil, s.err }

func TestSubscribe_FollowsTheFourChannelsAndRefusesABadStartOrRoute(t *testing.T) {
	ports := reader.Ports{Waiter: &scriptWaiter{log: &callLog{}, wakes: make(chan wake.Wake), waiting: make(chan struct{}, 1)}, Now: time.Now}

	if _, err := Subscribe(t.TempDir(), "yesterday", ports); err == nil {
		t.Error("Subscribe accepted a start that is not all, new, last, a time or a cursor")
	}
	rd, err := Subscribe(t.TempDir(), "new", ports)
	if err != nil || rd == nil {
		t.Fatalf("Subscribe = %v, %v, want a reader", rd, err)
	}
	if !reflect.DeepEqual(oversightChannels, []string{"loop", "cycle", "errors", "ship"}) {
		t.Errorf("channels = %v, want loop, cycle, errors and ship (ADR-0139 decision 11)", oversightChannels)
	}
	saved := channelRoutes["errors"]
	channelRoutes["errors"] = "severity>=LOUD"
	t.Cleanup(func() { channelRoutes["errors"] = saved })
	if _, err := Subscribe(t.TempDir(), "new", ports); err == nil {
		t.Error("Subscribe accepted a route that does not parse")
	}
}

func TestCodes_TheOversightRegistersItsCodesUnderTheLoopModule(t *testing.T) {
	for _, code := range []signalcenter.Code{CodeOversightAction, CodeFamilyPaused, CodeSupply, CodeSupplyShort} {
		if m, ok := signalcenter.IsRegistered(code); !ok || m != signalcenter.ModuleLoop {
			t.Errorf("%s is registered under %q, %v, want the loop module", code, m, ok)
		}
	}
}
