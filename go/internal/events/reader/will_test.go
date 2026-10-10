package reader

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/wake"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func exits(pids ...int) func() (wake.Wake, error) {
	return func() (wake.Wake, error) { return wake.Wake{Exited: pids}, nil }
}

func wantLoss(t *testing.T, items []Item, pid int, exit string, historical bool) {
	t.Helper()
	lost := losses(items)
	if len(lost) != 1 {
		t.Fatalf("losses = %s in %s, want one loop.lost", describe(lost), describe(items))
	}
	it := lost[0]
	ev := it.Record.Signal
	want := map[string]string{"pid": strconv.Itoa(pid), "run_id": "run-" + strconv.Itoa(pid), "exit": exit}
	if historical {
		want["historical"] = "true"
	}
	if !equalFields(ev.Fields, want) {
		t.Errorf("loop.lost fields = %v, want %v", ev.Fields, want)
	}
	if it.Record.Source != channel.SourceWatch || ev.Severity != signalcenter.SeverityIncident || ev.Code != "LOOP_LOST" || ev.Module != signalcenter.ModuleLoop || ev.PID != pid || ev.RunID != "run-"+strconv.Itoa(pid) {
		t.Errorf("loop.lost = %+v from %q, want an INCIDENT LOOP_LOST of module loop for pid %d from watch", *ev, it.Record.Source, pid)
	}
}

func equalFields(got, want map[string]string) bool {
	if len(got) != len(want) {
		return false
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

func TestLastWill_AWatchArmedMidLoopGetsALastWill(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("loop", loopStarted(loopPid, 1, procA))
	s.starts[loopPid] = []string{procA, procA, "gone"}
	r := f.reader(s, []string{"loop"}, Since{})
	s.then(exits(loopPid))

	items := drain(t, r)

	wantLoss(t, items, loopPid, "crash", false)
	if items[0].Channel != "loop" || len(items) != 1 {
		t.Errorf("items = %s, want one loop.lost on the loop channel", describe(items))
	}
	if !slices.Contains(s.arms[1].Pids, loopPid) {
		t.Errorf("arms = %v, want the exit watch of pid %d", s.arms, loopPid)
	}
	if s.clockN != 1 || items[0].Record.Signal.TS != "2026-10-09T18:00:01Z" {
		t.Errorf("loop.lost ts = %q after %d clock reads, want the reader's time", items[0].Record.Signal.TS, s.clockN)
	}
}

func TestLastWill_AStaleLoopStartedGivesNoAlarm(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("loop", loopStarted(loopPid, 1, procA))
	r := f.reader(s, []string{"loop"}, Since{})

	items := drain(t, r)

	if len(items) != 0 {
		t.Errorf("items = %s, want no alarm for a loop that was gone before the reader looked", describe(items))
	}
	for _, a := range s.arms {
		if len(a.Pids) != 0 {
			t.Errorf("armed pids %v, want no exit watch for a gone loop", a.Pids)
		}
	}
}

func TestLastWill_SinceAllReportsAStaleLoopOnceAsHistory(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("loop", loopStarted(loopPid, 1, procA))
	r := f.reader(s, []string{"loop"}, Since{mode: sinceAll})
	s.then(func() (wake.Wake, error) { return wake.Wake{Changed: true}, nil })

	items := drain(t, r)

	wantLoss(t, items, loopPid, "crash", true)
	if describe(items) != "loop:loop.started#1 loop:loop.lost#0" {
		t.Errorf("items = %s, want the record and one historical loss", describe(items))
	}
}

func TestLastWill_AStartCursorAfterTheStaleRecordGivesNoHistory(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("loop", loopStarted(loopPid, 1, procA), sig(1, 2, signalcenter.KindLoopWave, signalcenter.SeverityInfo))
	last, _ := f.log("loop").Last()
	r := f.reader(s, []string{"loop"}, since(t, "loop:"+strconv.FormatInt(last, 10)))

	items := drain(t, r)

	if describe(items) != "loop:loop.wave#2" {
		t.Errorf("items = %s, want no history for a loop.started before the start cursor", describe(items))
	}
}

func TestLastWill_ALoopThatDiesBetweenTheSeedScanAndTheArmIsLost(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("loop", loopStarted(loopPid, 1, procA))
	s.starts[loopPid] = []string{procA, "gone"}
	r := f.reader(s, []string{"loop"}, Since{})

	items := next(t, r)

	wantLoss(t, items, loopPid, "crash", false)
	if s.count("wait") != 0 {
		t.Errorf("waits = %d, want the loss before any wait", s.count("wait"))
	}
}

func TestLastWill_AReusedPidWithAnotherProcStartIsGone(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("loop", loopStarted(loopPid, 1, procA))
	s.starts[loopPid] = []string{procB}
	r := f.reader(s, []string{"loop"}, Since{})

	items := drain(t, r)

	if len(items) != 0 || slices.ContainsFunc(s.arms, func(a wake.Targets) bool { return len(a.Pids) > 0 }) {
		t.Errorf("items = %s, arms = %v, want no alarm and no exit watch for a reused pid", describe(items), s.arms)
	}
}

func TestLastWill_ASecondReadWithAnotherProcStartIsLost(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("loop", loopStarted(loopPid, 1, procA))
	s.starts[loopPid] = []string{procA, procB}
	r := f.reader(s, []string{"loop"}, Since{})

	items := next(t, r)

	wantLoss(t, items, loopPid, "crash", false)
}

func TestLastWill_ArmsTheExitWatchBeforeTheSecondProcStartRead(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("loop", loopStarted(loopPid, 1, procA))
	s.starts[loopPid] = []string{procA}
	r := f.reader(s, []string{"loop"}, Since{})

	drain(t, r)

	start := "start:" + strconv.Itoa(loopPid)
	want := []string{"arm", start, "arm", start}
	if got := s.only("arm", start); !slices.Equal(got[:4], want) {
		t.Errorf("calls = %v, want %v: the first read, the arm of the exit watch, then the second read", got, want)
	}
	if !slices.Contains(s.arms[1].Pids, loopPid) {
		t.Errorf("second arm = %+v, want the exit watch of pid %d", s.arms[1], loopPid)
	}
}

func TestLastWill_ALoopExitFoundOnCatchUpIsNoLoss(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("loop", loopStarted(loopPid, 1, procA))
	s.starts[loopPid] = []string{procA, procA, "gone"}
	r := f.reader(s, []string{"loop"}, Since{})
	s.then(func() (wake.Wake, error) {
		f.append("loop", loopExit(loopPid, 2))
		return wake.Wake{Exited: []int{loopPid}}, nil
	})

	items := drain(t, r)

	if describe(items) != "loop:loop.exit#2" {
		t.Errorf("items = %s, want the loop.exit and no loss", describe(items))
	}
}

func TestLastWill_AnIncidentGapFromThePidMakesTheExitUnknown(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		gap  channel.Record
		exit string
	}{
		{"lock deadline", incidentGap(loopPid, channel.ReasonLockDeadline, signalcenter.SeverityIncident), "unknown"},
		{"write error", incidentGap(loopPid, channel.ReasonWriteError, signalcenter.SeverityIncident), "unknown"},
		{"a warn gap", incidentGap(loopPid, channel.ReasonQueueFull, signalcenter.SeverityWarn), "crash"},
		{"another pid", incidentGap(loopPid2, channel.ReasonLockDeadline, signalcenter.SeverityIncident), "crash"},
		{"another source", func() channel.Record {
			g := incidentGap(loopPid, channel.ReasonLockDeadline, signalcenter.SeverityIncident)
			g.Source = "cycle"
			return g
		}(), "crash"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f, s := newFixture(t), newScript()
			f.append("loop", loopStarted(loopPid, 1, procA), tc.gap)
			s.starts[loopPid] = []string{procA, procA, "gone"}
			r := f.reader(s, []string{"errors"}, Since{})
			s.then(exits(loopPid))

			items := drain(t, r)

			wantLoss(t, items, loopPid, tc.exit, false)
		})
	}
}

func TestLastWill_AnIncidentGapBeforeTheLoopStartedLeavesTheExitACrash(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("loop", incidentGap(loopPid, channel.ReasonLockDeadline, signalcenter.SeverityIncident), loopStarted(loopPid, 1, procA))
	s.starts[loopPid] = []string{procA, procA, "gone"}
	r := f.reader(s, []string{"errors"}, Since{})
	s.then(exits(loopPid))

	items := drain(t, r)

	wantLoss(t, items, loopPid, "crash", false)
}

func TestLastWill_ASeedBeyondRetentionWritesAGap(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		selected []string
		since    Since
		want     string
	}{
		{"loop not selected, since all", []string{"errors"}, Since{mode: sinceAll}, "loop:gap(retention 0->BASE)"},
		{"loop selected, since all", []string{"loop"}, Since{mode: sinceAll}, "loop:gap(retention 0->BASE) loop:loop.wave#3"},
		{"loop not selected, since new", []string{"errors"}, Since{}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f, s := newFixture(t), newScript()
			f.seg = 200
			f.append("loop", sig(1, 1, signalcenter.KindLoopWave, signalcenter.SeverityInfo))
			f.append("loop", sig(1, 2, signalcenter.KindLoopWave, signalcenter.SeverityInfo))
			f.append("loop", sig(1, 3, signalcenter.KindLoopWave, signalcenter.SeverityInfo))
			segs := f.segments("loop")
			mustRemove(t, segs[0].Path)
			r := f.reader(s, tc.selected, tc.since)

			items := drain(t, r)

			want := replaceBase(tc.want, segs[1].Base)
			if describe(items) != want {
				t.Errorf("items = %s, want %s", describe(items), want)
			}
		})
	}
}

func replaceBase(s string, base int64) string {
	out := []byte{}
	for i := 0; i < len(s); i++ {
		if i+4 <= len(s) && s[i:i+4] == "BASE" {
			out = append(out, strconv.FormatInt(base, 10)...)
			i += 3
			continue
		}
		out = append(out, s[i])
	}
	return string(out)
}

func TestLastWill_ALoopThatStartsLaterIsWatched(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	s.starts[loopPid] = []string{procA, "gone"}
	r := f.reader(s, []string{"errors"}, Since{})
	s.then(func() (wake.Wake, error) {
		f.append("loop", loopStarted(loopPid, 1, procA))
		return wake.Wake{Changed: true}, nil
	})
	s.then(exits(loopPid))

	items := drain(t, r)

	wantLoss(t, items, loopPid, "crash", false)
	if items[0].Channel != "errors" {
		t.Errorf("loop.lost channel = %q, want errors: the route of a selected channel holds it", items[0].Channel)
	}
	armed := slices.IndexFunc(s.arms, func(a wake.Targets) bool { return slices.Contains(a.Pids, loopPid) })
	start := "start:" + strconv.Itoa(loopPid)
	calls := s.only("arm", start)
	if armed < 0 || slices.Index(calls, start) < armed+1 {
		t.Errorf("calls = %v, want the arm of the exit watch before the first read of a new loop", calls)
	}
}

func TestLastWill_ALossThatNoSelectedRouteHoldsIsNotPrinted(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("loop", loopStarted(loopPid, 1, procA))
	s.starts[loopPid] = []string{procA, procA, "gone"}
	r := f.reader(s, []string{"cycle"}, Since{})
	s.then(exits(loopPid))
	s.then(exits(loopPid))

	items := drain(t, r)

	if len(items) != 0 {
		t.Errorf("items = %s, want nothing: no selected route holds loop.lost", describe(items))
	}
	if got := s.lastArm().Pids; len(got) != 0 {
		t.Errorf("last arm pids = %v, want the lost loop dropped from the watch", got)
	}
}

func TestLastWill_TwoLiveLoopsAreWatchedApart(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("loop", loopStarted(loopPid, 1, procA), loopStarted(loopPid2, 2, procB))
	s.starts[loopPid] = []string{procA, procA, "gone"}
	s.starts[loopPid2] = []string{procB}
	r := f.reader(s, []string{"loop"}, Since{})
	s.then(exits(loopPid))

	items := drain(t, r)

	wantLoss(t, items, loopPid, "crash", false)
	if got := s.lastArm().Pids; !slices.Equal(got, []int{loopPid2}) {
		t.Errorf("last arm pids = %v, want only the live loop %d", got, loopPid2)
	}
}

func TestLastWill_ALaneExitIsNotALostLoop(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	lane := sig(lanePid, 1, signalcenter.KindLoopWave, signalcenter.SeverityInfo)
	lane.Source = "cycle"
	f.append("loop", lane)
	s.starts[lanePid] = []string{"1.000000"}
	r := f.reader(s, []string{"loop"}, Since{})
	s.then(exits(lanePid))

	items := drain(t, r)

	if len(losses(items)) != 0 || s.count("start:"+strconv.Itoa(lanePid)) != 0 {
		t.Errorf("items = %s, want no loss and no proc_start read for a lane", describe(items))
	}
	for _, a := range s.arms {
		if slices.Contains(a.Pids, lanePid) {
			t.Errorf("arm %+v watches the lane pid", a)
		}
	}
}

func TestLastWill_OneProcessWithTwoLoopStartedIsArmedOnce(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	s.onArm = func(n int) {
		if n == 1 {
			twin := loopStarted(loopPid, 2, procA)
			twin.Source = "loop-chain"
			f.append("loop", loopStarted(loopPid, 1, procA), twin)
		}
	}
	s.starts[loopPid] = []string{procA}
	r := f.reader(s, []string{"errors"}, Since{})
	s.then(exits(loopPid))

	items := drain(t, r)

	wantLoss(t, items, loopPid, "crash", false)
	if got := s.arms[1].Pids; !slices.Equal(got, []int{loopPid}) {
		t.Errorf("exit watch pids = %v, want pid %d once", got, loopPid)
	}
	if got := s.count("start:" + strconv.Itoa(loopPid)); got != 3 {
		t.Errorf("proc_start reads = %d, want 3: one first read for each record and one second read for the one live loop", got)
	}
}

func TestLastWill_AnExitWakeOfAnUnwatchedPidIsIgnored(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	r := f.reader(s, []string{"loop"}, Since{})
	s.then(exits(lanePid))

	items := drain(t, r)

	if len(items) != 0 {
		t.Errorf("items = %s, want nothing", describe(items))
	}
}

func TestLastWill_AProcStartErrorThatIsNotESRCHIsReturned(t *testing.T) {
	t.Parallel()
	for _, seed := range []bool{true, false} {
		t.Run(strconv.FormatBool(seed), func(t *testing.T) {
			t.Parallel()
			f, s := newFixture(t), newScript()
			f.append("loop", loopStarted(loopPid, 1, procA))
			s.starts[loopPid] = []string{procA}
			if seed {
				s.startErr = syscall.EPERM
			} else {
				s.onArm = func(n int) {
					if n == 2 {
						s.startErr = syscall.EPERM
					}
				}
			}
			r := f.reader(s, []string{"loop"}, Since{})

			_, err := r.Next(context.Background(), time.Time{})

			if !errors.Is(err, syscall.EPERM) {
				t.Errorf("Next = %v, want EPERM", err)
			}
		})
	}
}

func TestLastWill_AWipedLoopChannelIsMadeAgain(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("loop", sig(1, 1, signalcenter.KindLoopWave, signalcenter.SeverityInfo))
	s.starts[loopPid] = []string{procA, "gone"}
	r := f.reader(s, []string{"errors"}, Since{})
	s.then(func() (wake.Wake, error) {
		mustRemove(t, f.dir("loop"))
		return wake.Wake{Changed: true}, nil
	})
	s.then(func() (wake.Wake, error) {
		f.append("loop", loopStarted(loopPid, 1, procA))
		return wake.Wake{Changed: true}, nil
	})
	s.then(exits(loopPid))

	items := drain(t, r)

	wantLoss(t, items, loopPid, "crash", false)
}

func TestLastWill_AnUnreadableLoopChannelIsAnError(t *testing.T) {
	t.Parallel()
	for _, during := range []string{"seed", "catch-up"} {
		t.Run(during, func(t *testing.T) {
			t.Parallel()
			f, s := newFixture(t), newScript()
			bad := func() {
				if err := os.MkdirAll(f.dir("loop")+"/seg-00000000000000000000.ndjson", 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if during == "seed" {
				bad()
			} else {
				s.then(func() (wake.Wake, error) {
					bad()
					return wake.Wake{Changed: true}, nil
				})
			}
			r := f.reader(s, []string{"errors"}, Since{mode: sinceAll})

			_, err := r.Next(context.Background(), time.Time{})

			if !errors.Is(err, syscall.EISDIR) {
				t.Errorf("Next = %v, want the read error of the loop channel", err)
			}
		})
	}
}

func TestLastWill_SinceLastAtTheStaleRecordReportsItAsHistory(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("loop", loopStarted(loopPid, 1, procA))
	r := f.reader(s, []string{"loop"}, Since{mode: sinceLast})

	items := drain(t, r)

	wantLoss(t, items, loopPid, "crash", true)
}

func TestLastWill_ACursorListWithoutLoopGivesNoHistory(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("loop", loopStarted(loopPid, 1, procA))
	r := f.reader(s, []string{"cycle", "errors"}, since(t, "cycle:0"))

	items := drain(t, r)

	if len(items) != 0 {
		t.Errorf("items = %s, want no history: the list names no loop cursor", describe(items))
	}
}

func TestLastWill_ALoopExitFromAnotherSourceIsNotTheEnd(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("loop", loopStarted(loopPid, 1, procA))
	s.starts[loopPid] = []string{procA, procA}
	r := f.reader(s, []string{"errors"}, Since{})
	s.then(func() (wake.Wake, error) {
		other := loopExit(loopPid, 2)
		other.Source = "cycle"
		f.append("loop", other)
		return wake.Wake{Exited: []int{loopPid}}, nil
	})

	items := drain(t, r)

	wantLoss(t, items, loopPid, "crash", false)
}

func TestLastWill_ANewChannelWithoutARouteHoldsNoLoss(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	cat := []Channel{{Name: "loop", Route: route(t, "kind=cycle.sealed")}, {Name: "ci.required", Route: route(t, "kind=cycle.sealed")}}
	f.append("loop", loopStarted(loopPid, 1, procA))
	f.append("ci.release", sealed(1))
	s.starts[loopPid] = []string{procA, procA}
	r := f.readerOver(cat, s, []string{"ci.>"}, Since{})
	s.then(exits(loopPid))

	items := drain(t, r)

	if len(items) != 0 {
		t.Errorf("items = %s, want nothing: a channel with no route holds no loss", describe(items))
	}
}

func TestLastWill_TwoRecordsOfOnePidArmOneExitWatch(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("loop", loopStarted(loopPid, 1, procA))
	s.starts[loopPid] = []string{procA}
	r := f.reader(s, []string{"errors"}, Since{})
	s.then(func() (wake.Wake, error) {
		f.append("loop", loopStarted(loopPid, 2, procB))
		return wake.Wake{Changed: true}, nil
	})

	drain(t, r)

	for i, a := range s.arms {
		if len(a.Pids) > 1 {
			t.Errorf("arm %d pids = %v, want one exit watch for one pid", i, a.Pids)
		}
	}
}

func TestLastWill_OnlyTheRetentionGapFromZeroNamesTheOldestBase(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	r := f.reader(s, []string{"errors"}, Since{mode: sinceAll})
	read := r.read
	faked := false
	r.read = func(l *channel.Log, from, maxBytes int64) (channel.Batch, error) {
		if faked || filepath.Base(l.Dir()) != "loop" {
			return read(l, from, maxBytes)
		}
		faked = true
		return channel.Batch{Next: 400, Records: []channel.Record{
			{Source: channel.SourceWatch, Gap: &channel.Gap{Reason: channel.ReasonRetention, From: 0, To: 200}},
			{Source: channel.SourceWatch, Gap: &channel.Gap{Reason: channel.ReasonRetention, From: 200, To: 400}},
		}}, nil
	}

	items := next(t, r)

	if describe(items) != "loop:gap(retention 0->200)" {
		t.Errorf("items = %s, want one gap from the start to the oldest base", describe(items))
	}
}

func TestLastWill_ALossIsTheLastItemAndItsNextSkipsNoRecord(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("loop", loopStarted(loopPid, 1, procA))
	s.starts[loopPid] = []string{procA, procA}
	r := f.reader(s, []string{"loop", "errors"}, Since{})
	s.then(func() (wake.Wake, error) {
		f.append("loop", sig(1, 2, signalcenter.KindLoopWave, signalcenter.SeverityInfo))
		warn := sealed(3)
		warn.Signal.Severity = signalcenter.SeverityWarn
		f.append("errors", warn, warn)
		f.append("loop", sig(1, 4, signalcenter.KindLoopWave, signalcenter.SeverityInfo))
		return wake.Wake{Exited: []int{loopPid}}, nil
	})

	items := next(t, r)

	end, _ := f.log("loop").End()
	last := items[len(items)-1]
	if describe(items) != "loop:loop.wave#2 errors:cycle.sealed#3 loop:loop.wave#4 loop:loop.lost#0" {
		t.Fatalf("items = %s, want the records, then the loss last", describe(items))
	}
	if last.Channel != "loop" || last.Next != end {
		t.Errorf("loss channel %q next %d, want loop at its end %d: an ack of it skips no record", last.Channel, last.Next, end)
	}
}
