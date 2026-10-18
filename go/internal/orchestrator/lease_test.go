package orchestrator

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func leaseConfig(stage string) LeaseConfig {
	return LeaseConfig{Stage: stage, Deadline: 60 * time.Minute, Stall: 20 * time.Minute}
}

func runLane(launch fleet.LaunchFn, id string) chan fleet.Result {
	done := make(chan fleet.Result, 1)
	go func() {
		rc, err := launch(context.Background(), fleet.CycleSpec{Scope: []string{id}})
		done <- fleet.Result{ExitCode: rc, Err: err}
	}()
	return done
}

func TestLease_AShadowDeadlineRecordsTheEventAndStopsNothing(t *testing.T) {
	clock, events, g := newFakeClock(), &eventLog{}, newGatedLaunch()
	leases := NewLeases(leaseConfig(StageShadow), clock, noHeartbeat, events.publish)
	var settled []Return
	var settle Settle = func(_ Lane, r Return) { settled = append(settled, r) }
	done := runLane(leases.Watch(g.launch, settle), "A")
	waitStart(t, g)

	clock.Advance(61 * time.Minute)
	got := events.withCode(CodeLaneDeadline)
	if len(got) != 1 || got[0].Fields["stage"] != StageShadow || got[0].Fields["todo"] != "A" || got[0].Fields["action"] != "recorded" {
		t.Fatalf("deadline events = %+v, want one shadow event for A with action recorded", got)
	}
	select {
	case r := <-done:
		t.Fatalf("the shadow lease stopped the lane (result %+v), want it to keep running", r)
	default:
	}

	g.finish("A", 0)
	if r := <-done; r.ExitCode != 0 {
		t.Errorf("lane result = %+v, want its own exit 0", r)
	}
	if len(settled) != 1 || settled[0].Stopped || !settled[0].Late {
		t.Errorf("settled = %+v, want one late, unstopped return", settled)
	}
	late := events.withCode(CodeLaneLateExit)
	if len(late) != 1 || late[0].Fields["status"] != string(fleet.LaneOK) || late[0].Fields["stage"] != StageShadow {
		t.Errorf("late-exit events = %+v, want one ok shadow event: the S14 exit measure", late)
	}
}

func TestLease_AnEnforcedDeadlineStopsOnlyThatLaneThroughItsInjectedStop(t *testing.T) {
	clock, events, g := newFakeClock(), &eventLog{}, newGatedLaunch()
	leases := NewLeases(leaseConfig(StageEnforce), clock, noHeartbeat, events.publish)
	launch := leases.Watch(g.launch, nil)
	early := runLane(launch, "early")
	waitStart(t, g)
	clock.Advance(30 * time.Minute)
	late := runLane(launch, "late")
	waitStart(t, g)

	clock.Advance(31 * time.Minute)
	if r := <-early; r.ExitCode != 143 {
		t.Errorf("early lane result = %+v, want the stop of its own launch context (143)", r)
	}
	select {
	case r := <-late:
		t.Fatalf("the lane inside its deadline stopped too (result %+v)", r)
	default:
	}
	got := events.withCode(CodeLaneDeadline)
	if len(got) != 1 || got[0].Fields["todo"] != "early" || got[0].Fields["action"] != "stopped" || got[0].Fields["stage"] != StageEnforce {
		t.Errorf("deadline events = %+v, want one enforce stop of early", got)
	}
	g.finish("late", 0)
	<-late
}

func TestLease_ASilentLaneReportsItsStallOnce(t *testing.T) {
	clock, events, g := newFakeClock(), &eventLog{}, newGatedLaunch()
	leases := NewLeases(LeaseConfig{Stage: StageEnforce, Deadline: 5 * time.Hour, Stall: 20 * time.Minute}, clock, silentHeartbeat, events.publish)
	done := runLane(leases.Watch(g.launch, nil), "A")
	waitStart(t, g)

	clock.Advance(21 * time.Minute)
	clock.Advance(21 * time.Minute)
	clock.Advance(21 * time.Minute)
	got := events.withCode(CodeLaneStalled)
	if len(got) != 1 || got[0].Fields["todo"] != "A" {
		t.Errorf("stall events = %+v, want exactly one for A", got)
	}
	g.finish("A", 0)
	<-done
}

func TestLease_AFreshHeartbeatPostponesTheStall(t *testing.T) {
	clock, events, g := newFakeClock(), &eventLog{}, newGatedLaunch()
	beat := epoch
	var heartbeat Heartbeat = func(Lane) (time.Time, bool) { return beat, true }
	leases := NewLeases(LeaseConfig{Stage: StageEnforce, Deadline: 5 * time.Hour, Stall: 20 * time.Minute}, clock, heartbeat, events.publish)
	done := runLane(leases.Watch(g.launch, nil), "A")
	waitStart(t, g)

	beat = epoch.Add(15 * time.Minute)
	clock.Advance(21 * time.Minute)
	if got := events.withCode(CodeLaneStalled); len(got) != 0 {
		t.Fatalf("stall events = %+v at 21m with a heartbeat at 15m, want none", got)
	}
	clock.Advance(15 * time.Minute)
	if got := events.withCode(CodeLaneStalled); len(got) != 1 {
		t.Errorf("stall events = %+v at 36m with the last heartbeat at 15m, want one", got)
	}
	g.finish("A", 0)
	<-done
}

func TestLease_ManyLanesArmOneTimer(t *testing.T) {
	clock, events, g := newFakeClock(), &eventLog{}, newGatedLaunch()
	leases := NewLeases(leaseConfig(StageShadow), clock, noHeartbeat, events.publish)
	launch := leases.Watch(g.launch, nil)
	var lanes []chan fleet.Result
	for _, id := range []string{"A", "B", "C"} {
		lanes = append(lanes, runLane(launch, id))
		waitStart(t, g)
		clock.Advance(time.Minute)
		if n := clock.armed(); n != 1 {
			t.Fatalf("armed timers after %s = %d, want 1 (one timer for the nearest deadline)", id, n)
		}
	}
	for i, id := range []string{"A", "B", "C"} {
		g.finish(id, 0)
		<-lanes[i]
	}
	if n := clock.armed(); n != 0 {
		t.Errorf("armed timers with no lane = %d, want 0", n)
	}
}

func TestLease_ALaneThatExitsBeforeItsTimerFiresIsForgotten(t *testing.T) {
	clock, events, g := newFakeClock(), &eventLog{}, newGatedLaunch()
	leases := NewLeases(leaseConfig(StageEnforce), clock, noHeartbeat, events.publish)
	done := runLane(leases.Watch(g.launch, nil), "A")
	waitStart(t, g)
	g.finish("A", 0)
	<-done
	clock.Advance(2 * time.Hour)
	leases.fire()
	if n := len(events.withCode(CodeLaneDeadline)) + len(events.withCode(CodeLaneStalled)); n != 0 {
		t.Errorf("a fire after the lane exit published %d lease event(s), want none", n)
	}
}

func TestLease_AnExitedLaneDeepInTheHeapIsSkippedWhenItsTimeComes(t *testing.T) {
	clock, events, g := newFakeClock(), &eventLog{}, newGatedLaunch()
	leases := NewLeases(leaseConfig(StageShadow), clock, silentHeartbeat, events.publish)
	launch := leases.Watch(g.launch, nil)
	b := runLane(launch, "B")
	waitStart(t, g)
	clock.Advance(time.Minute)
	a := runLane(launch, "A")
	waitStart(t, g)
	g.finish("A", 0)
	<-a

	clock.Advance(25 * time.Minute)
	got := events.withCode(CodeLaneStalled)
	if len(got) != 1 || got[0].Fields["todo"] != "B" {
		t.Errorf("stall events = %+v, want only B (A exited before its stall time)", got)
	}
	g.finish("B", 0)
	<-b
}

func TestSystemClock_RunsTheFunctionAfterTheDelay(t *testing.T) {
	fired := make(chan time.Time, 1)
	var clock Clock = SystemClock{}
	start := clock.Now()
	clock.AfterFunc(time.Millisecond, func() { fired <- time.Now() })
	select {
	case at := <-fired:
		if at.Before(start) {
			t.Errorf("fired at %v, before the start %v", at, start)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the system clock never ran the function")
	}
	if clock.AfterFunc(time.Hour, func() {}).Stop() != true {
		t.Error("Stop of a pending timer returned false")
	}
}

func TestLease_AnUnknownHeartbeatReportsNoStall(t *testing.T) {
	clock, events, g := newFakeClock(), &eventLog{}, newGatedLaunch()
	leases := NewLeases(LeaseConfig{Stage: StageEnforce, Deadline: 5 * time.Hour, Stall: 20 * time.Minute}, clock, noHeartbeat, events.publish)
	done := runLane(leases.Watch(g.launch, nil), "A")
	waitStart(t, g)

	for range 4 {
		clock.Advance(21 * time.Minute)
	}
	if got := events.withCode(CodeLaneStalled); len(got) != 0 {
		t.Errorf("stall events = %+v, want none: a lane whose cycle is not found has an unknown heartbeat, not the launch time", got)
	}
	if n := clock.armed(); n != 1 {
		t.Errorf("armed timers = %d, want 1: the lease probes the unknown heartbeat again", n)
	}
	g.finish("A", 0)
	<-done
}

func TestLease_AGiveWithNoLeaseIsAnErrorNotAPanic(t *testing.T) {
	leases := NewLeases(leaseConfig(StageEnforce), newFakeClock(), noHeartbeat, nil)
	ret, err := leases.give(Lane{Key: "dup"}, fleet.Result{ExitCode: 0})
	if err == nil || !strings.Contains(err.Error(), "dup") {
		t.Errorf("give of a lane with no lease = %v, want an error that names the key", err)
	}
	if ret.Stopped || ret.Late || ret.Stops != 0 {
		t.Errorf("return = %+v, want a zero return", ret)
	}
}

func TestLease_ADuplicateKeyFailsTheSecondLaneLoudly(t *testing.T) {
	clock, g := newFakeClock(), newGatedLaunch()
	launch := NewLeases(leaseConfig(StageEnforce), clock, noHeartbeat, nil).Watch(g.launch, nil)
	first := runLane(launch, "dup")
	waitStart(t, g)
	second := runLane(launch, "dup")
	waitStart(t, g)
	g.finish("dup", 0)
	g.finish("dup", 0)
	results := []fleet.Result{<-first, <-second}
	failed := 0
	for _, r := range results {
		if r.Err != nil && strings.Contains(r.Err.Error(), "no lease") {
			failed++
		}
	}
	if failed != 1 {
		t.Errorf("results = %+v, want exactly one lane to return the no-lease error", results)
	}
}

func TestLease_AZeroStallChecksNoStallAndArmsOnlyTheDeadline(t *testing.T) {
	for name, beat := range map[string]Heartbeat{"unknown": noHeartbeat, "silent": silentHeartbeat} {
		clock, events, g := newFakeClock(), &eventLog{}, newGatedLaunch()
		leases := NewLeases(LeaseConfig{Stage: StageShadow, Deadline: time.Hour}, clock, beat, events.publish)
		done := runLane(leases.Watch(g.launch, nil), "A")
		waitStart(t, g)
		clock.mu.Lock()
		timers := len(clock.timers)
		clock.mu.Unlock()
		if timers != 1 {
			t.Errorf("%s: timers at launch = %d, want 1: the deadline only", name, timers)
		}
		clock.Advance(2 * time.Hour)
		if got := events.withCode(CodeLaneStalled); len(got) != 0 {
			t.Errorf("%s: stall events = %+v, want none with stall_minutes 0", name, got)
		}
		if got := events.withCode(CodeLaneDeadline); len(got) != 1 {
			t.Errorf("%s: deadline events = %d, want 1", name, len(got))
		}
		g.finish("A", 0)
		<-done
	}
}

func TestLease_ABeatMakesAnUnknownHeartbeatKnownAndMovesTheStall(t *testing.T) {
	clock, events, g := newFakeClock(), &eventLog{}, newGatedLaunch()
	leases := NewLeases(LeaseConfig{Stage: StageEnforce, Deadline: 5 * time.Hour, Stall: 20 * time.Minute}, clock, noHeartbeat, events.publish)
	done := runLane(leases.Watch(g.launch, nil), "A")
	waitStart(t, g)

	clock.Advance(5 * time.Minute)
	leases.Beat("missing", epoch.Add(5*time.Minute))
	leases.Beat("A", epoch.Add(5*time.Minute))
	leases.Beat("A", epoch.Add(time.Minute))
	clock.Advance(16 * time.Minute)
	if got := events.withCode(CodeLaneStalled); len(got) != 0 {
		t.Fatalf("stall events at 21m with a beat at 5m = %+v, want none", got)
	}
	clock.Advance(5 * time.Minute)
	got := events.withCode(CodeLaneStalled)
	if len(got) != 1 || got[0].Fields["last_heartbeat"] != epoch.Add(5*time.Minute).Format(time.RFC3339) {
		t.Errorf("stall events at 26m = %+v, want one that names the beat at 5m: an older beat never moves it back", got)
	}
	g.finish("A", 0)
	<-done
}

func TestLease_ABeatFromTheFutureCountsAsNowAndNeverPostponesTheStall(t *testing.T) {
	clock, events, g := newFakeClock(), &eventLog{}, newGatedLaunch()
	leases := NewLeases(LeaseConfig{Stage: StageEnforce, Deadline: 5 * time.Hour, Stall: 20 * time.Minute}, clock, noHeartbeat, events.publish)
	done := runLane(leases.Watch(g.launch, nil), "A")
	waitStart(t, g)

	leases.Beat("A", epoch.Add(10*time.Hour))
	clock.Advance(21 * time.Minute)

	got := events.withCode(CodeLaneStalled)
	if len(got) != 1 || got[0].Fields["last_heartbeat"] != epoch.Format(time.RFC3339) {
		t.Errorf("stall events = %+v, want one that counts the beat from the future as the clock of that moment: a skewed producer must not hide a stall", got)
	}
	g.finish("A", 0)
	<-done
}

func TestLease_AZeroStallTurnsTheReportedStallOffToo(t *testing.T) {
	clock, events, g := newFakeClock(), &eventLog{}, newGatedLaunch()
	leases := NewLeases(LeaseConfig{Stage: StageEnforce, Deadline: 5 * time.Hour}, clock, noHeartbeat, events.publish)
	done := runLane(leases.Watch(g.launch, nil), "A")
	waitStart(t, g)

	leases.Stall("A")

	if got := events.withCode(CodeLaneStalled); len(got) != 0 {
		t.Errorf("stall events = %+v, want none: stall_minutes 0 turns the stall check off for the reported stall as for the timer", got)
	}
	g.finish("A", 0)
	<-done
}

func TestLease_EpochCountsEachGrantAndEachGive(t *testing.T) {
	clock, g := newFakeClock(), newGatedLaunch()
	leases := NewLeases(leaseConfig(StageEnforce), clock, noHeartbeat, nil)
	before := leases.Epoch()
	done := runLane(leases.Watch(g.launch, nil), "A")
	waitStart(t, g)
	granted := leases.Epoch()

	g.finish("A", 0)
	<-done

	if granted != before+1 || leases.Epoch() != granted+1 {
		t.Errorf("epoch = %d, %d, %d, want a step of 1 at the grant and a step of 1 at the give", before, granted, leases.Epoch())
	}
	if _, err := leases.give(Lane{Key: "none"}, fleet.Result{}); err == nil || leases.Epoch() != granted+1 {
		t.Errorf("a give with no lease moved the epoch to %d (err %v), want it unchanged", leases.Epoch(), err)
	}
}

func TestLease_AReportedStallPublishesOnceAndTheTimerNeverRepeatsIt(t *testing.T) {
	clock, events, g := newFakeClock(), &eventLog{}, newGatedLaunch()
	leases := NewLeases(LeaseConfig{Stage: StageShadow, Deadline: 5 * time.Hour, Stall: 20 * time.Minute}, clock, silentHeartbeat, events.publish)
	done := runLane(leases.Watch(g.launch, nil), "A")
	waitStart(t, g)

	leases.Stall("missing")
	leases.Stall("A")
	leases.Stall("A")
	clock.Advance(25 * time.Minute)
	got := events.withCode(CodeLaneStalled)
	if len(got) != 1 || got[0].Fields["todo"] != "A" || got[0].Fields["stage"] != StageShadow {
		t.Errorf("stall events = %+v, want exactly one for A: the reported stall and the timer share one mark", got)
	}
	g.finish("A", 0)
	<-done
}

func TestLease_LiveAndLaneListTheLanesInFlightWithTheirStart(t *testing.T) {
	clock, g := newFakeClock(), newGatedLaunch()
	leases := NewLeases(leaseConfig(StageEnforce), clock, noHeartbeat, nil)
	launch := leases.Watch(g.launch, nil)
	a := runLane(launch, "A")
	waitStart(t, g)
	clock.Advance(time.Minute)
	b := runLane(launch, "B")
	waitStart(t, g)

	live := leases.Live()
	if len(live) != 2 {
		t.Fatalf("Live = %+v, want 2 lanes", live)
	}
	lane, ok := leases.Lane("B")
	if !ok || !lane.Started.Equal(epoch.Add(time.Minute)) || lane.Key != "B" {
		t.Errorf("Lane(B) = %+v, %v, want B started at 1m", lane, ok)
	}
	g.finish("A", 0)
	<-a
	if _, ok := leases.Lane("A"); ok {
		t.Error("Lane(A) found a lane that exited")
	}
	g.finish("B", 0)
	<-b
}
