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
	"github.com/mickeyyaya/evolve-loop/go/internal/events/filter"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/wake"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestReader_CallOrderIsArmThenOneReadThenWait(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	r := f.reader(s, []string{"cycle"}, Since{})
	readsBeforeWake := -1
	s.then(func() (wake.Wake, error) {
		readsBeforeWake = s.count("read:cycle")
		f.append("cycle", sealed(1))
		return wake.Wake{Changed: true}, nil
	})

	items := next(t, r)

	if readsBeforeWake != 1 {
		t.Errorf("reads of cycle before the wake = %d, want 1", readsBeforeWake)
	}
	want := []string{"arm", "read:cycle", "wait", "arm", "read:cycle"}
	if got := s.only("arm", "read:cycle", "wait"); !slices.Equal(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
	if describe(items) != "cycle:cycle.sealed#1" {
		t.Errorf("items = %s, want the record that the wake announced", describe(items))
	}
}

func TestReader_AnAppendBetweenTheArmAndTheReadIsNotLost(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	r := f.reader(s, []string{"cycle"}, Since{})
	s.onArm = func(n int) {
		if n == 1 {
			f.append("cycle", sealed(1))
		}
	}

	items := next(t, r)

	if describe(items) != "cycle:cycle.sealed#1" || s.count("wait") != 0 {
		t.Errorf("items = %s after %d waits, want the record without a wait", describe(items), s.count("wait"))
	}
}

func TestReader_WithNoDeadlineCreatesNoTimer(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	r := f.reader(s, []string{"cycle"}, Since{})
	s.then(func() (wake.Wake, error) {
		f.append("cycle", sealed(1))
		return wake.Wake{Changed: true}, nil
	})

	next(t, r)

	if len(s.deadlines) != 1 || !s.deadlines[0].IsZero() {
		t.Errorf("Wait deadlines = %v, want one zero deadline", s.deadlines)
	}
	if s.clockN != 0 {
		t.Errorf("clock reads = %d, want 0: a watch with no deadline reads no time", s.clockN)
	}
}

func TestReader_ADeadlinePassesToWaitAndEndsTheCall(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	r := f.reader(s, []string{"cycle"}, Since{})
	deadline := time.Date(2026, 10, 9, 18, 30, 0, 0, time.UTC)

	items, err := r.Next(context.Background(), deadline)

	if !errors.Is(err, ErrDeadline) || len(items) != 0 {
		t.Errorf("Next = %s, %v, want no item and ErrDeadline", describe(items), err)
	}
	if len(s.deadlines) != 1 || !s.deadlines[0].Equal(deadline) {
		t.Errorf("Wait deadlines = %v, want [%v]", s.deadlines, deadline)
	}
}

func TestReader_FollowsRotationWithoutALostLine(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.seg = 200
	r := f.reader(s, []string{"cycle"}, Since{})
	for seq := uint64(1); seq <= 4; seq++ {
		s.then(func() (wake.Wake, error) {
			f.append("cycle", sealed(seq))
			return wake.Wake{Changed: true}, nil
		})
	}

	items := drain(t, r)

	want := "cycle:cycle.sealed#1 cycle:cycle.sealed#2 cycle:cycle.sealed#3 cycle:cycle.sealed#4"
	if describe(items) != want {
		t.Errorf("items = %s, want %s", describe(items), want)
	}
	segs := f.segments("cycle")
	if len(segs) < 4 {
		t.Fatalf("segments = %v, want a rotation on each append", segs)
	}
	if tail := segs[len(segs)-1].Path; !slices.Contains(s.lastArm().Files, tail) {
		t.Errorf("last armed files = %v, want the new tail %s", s.lastArm().Files, tail)
	}
}

func TestReader_ACursorBelowRetentionYieldsOneGapRecord(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.seg = 200
	for seq := uint64(1); seq <= 3; seq++ {
		f.append("cycle", sealed(seq))
	}
	segs := f.segments("cycle")
	mustRemove(t, segs[0].Path)
	r := f.reader(s, []string{"cycle"}, since(t, "cycle:0"))

	items := drain(t, r)

	want := "cycle:gap(retention 0->" + strconv.FormatInt(segs[1].Base, 10) + ") cycle:cycle.sealed#2 cycle:cycle.sealed#3"
	if describe(items) != want {
		t.Errorf("items = %s, want %s", describe(items), want)
	}
}

func TestReader_ACursorPastTheTailYieldsAResetGapAtTheTrueEnd(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("cycle", sealed(1), sealed(2))
	end, _ := f.log("cycle").End()
	r := f.reader(s, []string{"cycle"}, since(t, "cycle:"+strconv.FormatInt(end+1000, 10)))
	s.then(func() (wake.Wake, error) {
		f.append("cycle", sealed(3))
		return wake.Wake{Changed: true}, nil
	})

	items := drain(t, r)

	want := "cycle:gap(reset " + strconv.FormatInt(end+1000, 10) + "->" + strconv.FormatInt(end, 10) + ") cycle:cycle.sealed#3"
	if describe(items) != want {
		t.Fatalf("items = %s, want %s", describe(items), want)
	}
	if items[0].Next != end || items[1].Record.Cursor != end {
		t.Errorf("gap next = %d, record cursor = %d, want both %d", items[0].Next, items[1].Record.Cursor, end)
	}
}

func TestReader_ACursorPastASegmentEndMovesToTheNextBase(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.seg = 200
	f.append("cycle", sealed(1))
	f.append("cycle", sealed(2))
	segs := f.segments("cycle")
	if err := os.Truncate(segs[0].Path, segs[1].Base-10); err != nil {
		t.Fatal(err)
	}
	from := segs[1].Base - 5
	r := f.reader(s, []string{"cycle"}, since(t, "cycle:"+strconv.FormatInt(from, 10)))

	items := drain(t, r)

	want := "cycle:gap(reset " + strconv.FormatInt(from, 10) + "->" + strconv.FormatInt(segs[1].Base, 10) + ") cycle:cycle.sealed#2"
	if describe(items) != want {
		t.Errorf("items = %s, want %s", describe(items), want)
	}
}

func TestReader_AWipedDirectoryYieldsAResetGapAndArmsAgain(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("cycle", sealed(1))
	r := f.reader(s, []string{"cycle"}, Since{mode: sinceAll})
	end, _ := f.log("cycle").End()
	armsAtWipe := 0
	s.then(func() (wake.Wake, error) {
		mustRemove(t, f.dir("cycle"))
		armsAtWipe = len(s.arms)
		return wake.Wake{Changed: true}, nil
	})
	s.then(func() (wake.Wake, error) {
		f.append("cycle", sealed(2))
		return wake.Wake{Changed: true}, nil
	})

	items := drain(t, r)

	want := "cycle:cycle.sealed#1 cycle:gap(reset " + strconv.FormatInt(end, 10) + "->0) cycle:cycle.sealed#2"
	if describe(items) != want {
		t.Errorf("items = %s, want %s", describe(items), want)
	}
	if len(s.arms) <= armsAtWipe || !slices.Contains(s.arms[armsAtWipe].Dirs, f.dir("cycle")) {
		t.Errorf("arms = %v after the wipe at arm %d, want a new arm of %s", s.arms, armsAtWipe, f.dir("cycle"))
	}
	if items[2].Record.Cursor != 0 {
		t.Errorf("first record after the wipe has cursor %d, want 0", items[2].Record.Cursor)
	}
}

func TestReader_AMalformedLineYieldsAGapAndTheCursorMovesPastIt(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	if err := os.MkdirAll(f.dir("cycle"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.dir("cycle"), "seg-00000000000000000000.ndjson"), []byte("not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.append("cycle", sealed(1))
	r := f.reader(s, []string{"cycle"}, Since{mode: sinceAll})

	items := drain(t, r)

	if describe(items) != "cycle:gap(malformed 0->9) cycle:cycle.sealed#1" {
		t.Fatalf("items = %s, want a malformed gap over the 9 bytes, then the record", describe(items))
	}
	if items[0].Next != 9 || items[1].Record.Cursor != 9 || items[0].Record.Source != channel.SourceWatch {
		t.Errorf("gap next = %d, record cursor = %d, gap source = %q, want 9, 9, watch", items[0].Next, items[1].Record.Cursor, items[0].Record.Source)
	}
}

func TestReader_TheSameEventInTwoChannelsIsDeliveredOnce(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	ev := sealed(1)
	ev.Signal.Severity = signalcenter.SeverityWarn
	f.append("cycle", ev)
	f.append("errors", ev)
	other := sealed(2)
	other.Signal.Severity = signalcenter.SeverityWarn
	f.append("errors", other)
	r := f.reader(s, []string{"cycle", "errors"}, Since{mode: sinceAll})

	items := drain(t, r)

	if describe(items) != "cycle:cycle.sealed#1 errors:cycle.sealed#2" {
		t.Errorf("items = %s, want event 1 once and event 2", describe(items))
	}
}

func TestReader_TheSameIDFromAnotherSourceIsAnotherEvent(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	ev := sealed(1)
	ev.Signal.Severity = signalcenter.SeverityWarn
	twin := sealed(1)
	twin.Source = "cycle"
	twin.Signal.Severity = signalcenter.SeverityWarn
	f.append("cycle", ev)
	f.append("errors", twin)
	r := f.reader(s, []string{"cycle", "errors"}, Since{mode: sinceAll})

	items := drain(t, r)

	if describe(items) != "cycle:cycle.sealed#1 errors:cycle.sealed#1" {
		t.Errorf("items = %s, want both: one process can hold two Centers with one seq each", describe(items))
	}
}

func TestReader_SinceLastStartsAtTheLastRecord(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("cycle", sealed(1), sealed(2), sealed(3))
	r := f.reader(s, []string{"cycle"}, Since{mode: sinceLast})

	items := drain(t, r)

	if describe(items) != "cycle:cycle.sealed#3" {
		t.Errorf("items = %s, want the last record only", describe(items))
	}
}

func TestReader_SinceNewStartsAtTheEnd(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("cycle", sealed(1))
	r := f.reader(s, []string{"cycle"}, Since{})
	s.then(func() (wake.Wake, error) {
		f.append("cycle", sealed(2))
		return wake.Wake{Changed: true}, nil
	})

	items := drain(t, r)

	if describe(items) != "cycle:cycle.sealed#2" {
		t.Errorf("items = %s, want only the record after the arm", describe(items))
	}
}

func TestReader_SinceATimeStartsAtTheFirstRecordAtOrAfterIt(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("cycle", sealed(10), sealed(20), sealed(30))
	at, _ := time.Parse(time.RFC3339, "2026-10-09T17:46:20Z")
	r := f.reader(s, []string{"cycle"}, Since{mode: sinceTime, at: at})

	items := drain(t, r)

	if describe(items) != "cycle:cycle.sealed#20 cycle:cycle.sealed#30" {
		t.Errorf("items = %s, want the records at or after 17:46:20", describe(items))
	}
}

func TestReader_SinceATimeAfterEveryRecordStartsAtTheEnd(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("cycle", sealed(10))
	bad := sealed(11)
	bad.Signal.TS = "yesterday"
	f.append("cycle", bad, incidentGap(4242, channel.ReasonQueueFull, signalcenter.SeverityWarn))
	at, _ := time.Parse(time.RFC3339, "2026-10-09T17:47:00Z")
	r := f.reader(s, []string{"cycle"}, Since{mode: sinceTime, at: at})
	s.then(func() (wake.Wake, error) {
		f.append("cycle", sealed(12))
		return wake.Wake{Changed: true}, nil
	})

	items := drain(t, r)

	if describe(items) != "cycle:cycle.sealed#12" {
		t.Errorf("items = %s, want only the record after the scan end", describe(items))
	}
}

func TestReader_ACursorListStartsANamedChannelThereAndAnUnnamedOneAtZero(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("cycle", sealed(1), sealed(2))
	warn := sealed(3)
	warn.Signal.Severity = signalcenter.SeverityWarn
	f.append("errors", warn)
	second, _ := f.log("cycle").Last()
	r := f.reader(s, []string{"cycle", "errors"}, since(t, "cycle:"+strconv.FormatInt(second, 10)))

	items := drain(t, r)

	if describe(items) != "cycle:cycle.sealed#2 errors:cycle.sealed#3" {
		t.Errorf("items = %s, want cycle from its cursor and errors from 0", describe(items))
	}
}

func TestReader_AWildcardSelectionAddsANewChannel(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	cat := []Channel{{Name: "loop", Route: route(t, "module=loop")}, {Name: "ci.required"}}
	r := f.readerOver(cat, s, []string{"ci.>"}, Since{})
	s.then(func() (wake.Wake, error) {
		f.append("ci.release", sealed(1))
		return wake.Wake{Changed: true}, nil
	})

	items := drain(t, r)

	if !slices.Contains(s.arms[0].Dirs, f.root) {
		t.Errorf("first arm dirs = %v, want the channel root %s for the wildcard", s.arms[0].Dirs, f.root)
	}
	if describe(items) != "ci.release:cycle.sealed#1" || items[0].Record.Cursor != 0 {
		t.Errorf("items = %s, want the record of the new channel from cursor 0", describe(items))
	}
	if !slices.Contains(s.lastArm().Dirs, f.dir("ci.release")) {
		t.Errorf("last arm dirs = %v, want the new channel", s.lastArm().Dirs)
	}
}

func TestReader_ANameSelectionDoesNotWatchTheRoot(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	r := f.reader(s, []string{"cycle"}, Since{})
	s.then(func() (wake.Wake, error) {
		f.append("cycle.extra", sealed(1))
		return wake.Wake{Changed: true}, nil
	})

	items := drain(t, r)

	if slices.Contains(s.arms[0].Dirs, f.root) || len(items) != 0 {
		t.Errorf("arm dirs = %v, items = %s, want no root watch and no item from an unselected channel", s.arms[0].Dirs, describe(items))
	}
}

func TestReader_MakesEachChannelDirectoryBeforeItArms(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	r := f.reader(s, []string{"cycle"}, Since{})
	var missing []string
	s.onArm = func(int) {
		for _, name := range []string{"cycle", "loop"} {
			if _, err := os.Stat(f.dir(name)); err != nil {
				missing = append(missing, name)
			}
		}
	}

	drain(t, r)

	want := []string{f.dir("cycle"), f.dir("loop")}
	if len(missing) != 0 || !slices.Equal(s.arms[0].Dirs, want) {
		t.Errorf("missing at arm = %v, dirs = %v, want none missing and %v", missing, s.arms[0].Dirs, want)
	}
}

func TestReader_MergesChannelsByTimeAndKeepsTheOrderOfEachChannel(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("cycle", sealed(5), sealed(2), sealed(9))
	warn := func(seq uint64) channel.Record {
		r := sealed(seq)
		r.Signal.PID = 5151
		r.Signal.Severity = signalcenter.SeverityWarn
		return r
	}
	f.append("errors", warn(1), warn(7))
	r := f.reader(s, []string{"cycle", "errors"}, Since{mode: sinceAll})

	items := drain(t, r)

	want := "errors:cycle.sealed#1 cycle:cycle.sealed#5 cycle:cycle.sealed#2 errors:cycle.sealed#7 cycle:cycle.sealed#9"
	if describe(items) != want {
		t.Errorf("items = %s, want %s", describe(items), want)
	}
}

func TestReader_AHangupIsTerminal(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	r := f.reader(s, []string{"cycle"}, Since{})
	s.then(func() (wake.Wake, error) { return wake.Wake{Hangup: true, Changed: true}, nil })

	items, err := r.Next(context.Background(), time.Time{})

	if !errors.Is(err, ErrHangup) || len(items) != 0 {
		t.Errorf("Next = %s, %v, want ErrHangup", describe(items), err)
	}
}

func TestReader_AWaitErrorIsReturned(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	r := f.reader(s, []string{"cycle"}, Since{})
	cause := errors.New("kevent failed")
	s.then(func() (wake.Wake, error) { return wake.Wake{}, cause })

	_, err := r.Next(context.Background(), time.Time{})

	if !errors.Is(err, cause) {
		t.Errorf("Next = %v, want %v", err, cause)
	}
}

func TestReader_AnArmErrorIsReturned(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	s.armErr = wake.ErrRefused
	r := f.reader(s, []string{"cycle"}, Since{})

	_, err := r.Next(context.Background(), time.Time{})

	if !errors.Is(err, wake.ErrRefused) || s.count("read:cycle") != 0 {
		t.Errorf("Next = %v after %d reads, want ErrRefused before any read", err, s.count("read:cycle"))
	}
}

func TestReader_ALaterArmErrorIsReturned(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	r := f.reader(s, []string{"cycle"}, Since{})
	s.then(func() (wake.Wake, error) {
		s.armErr = wake.ErrRefused
		return wake.Wake{Changed: true}, nil
	})

	_, err := r.Next(context.Background(), time.Time{})

	if !errors.Is(err, wake.ErrRefused) {
		t.Errorf("Next = %v, want ErrRefused", err)
	}
}

func TestReader_AnUnreadableSegmentIsAnError(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	if err := os.MkdirAll(filepath.Join(f.dir("cycle"), "seg-00000000000000000000.ndjson"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := f.reader(s, []string{"cycle"}, Since{mode: sinceAll})

	_, err := r.Next(context.Background(), time.Time{})

	if !errors.Is(err, syscall.EISDIR) {
		t.Errorf("Next = %v, want the read error", err)
	}
}

func TestReader_AnUnmakeableRootIsAnError(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	if err := os.WriteFile(filepath.Dir(f.root)+"/ch", nil, 0o644); err != nil {
		t.Fatal(err)
	}
	r := f.reader(s, []string{"cycle"}, Since{})

	_, err := r.Next(context.Background(), time.Time{})

	if !errors.Is(err, syscall.ENOTDIR) || len(s.arms) != 0 {
		t.Errorf("Next = %v after %d arms, want the make error before any arm", err, len(s.arms))
	}
}

func TestReader_AGoneRootUnderAWildcardIsAnError(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	r := f.reader(s, []string{"*"}, Since{})
	s.then(func() (wake.Wake, error) {
		mustRemove(t, f.root)
		if err := os.WriteFile(f.root, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		return wake.Wake{Changed: true}, nil
	})

	_, err := r.Next(context.Background(), time.Time{})

	if !errors.Is(err, syscall.ENOTDIR) {
		t.Errorf("Next = %v, want the error of the gone root", err)
	}
}

func TestReader_AFailedOpenIsTriedAgainOnTheNextCall(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	if err := os.MkdirAll(filepath.Dir(f.root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.root, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	r := f.reader(s, []string{"cycle"}, Since{})

	_, first := r.Next(context.Background(), time.Time{})
	_, second := r.Next(context.Background(), time.Time{})

	if !errors.Is(first, syscall.ENOTDIR) || !errors.Is(second, syscall.ENOTDIR) {
		t.Errorf("Next = %v then %v, want the same open error twice", first, second)
	}
}

func TestReader_RefusesASelectorThatMatchesNoChannel(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	r := f.reader(s, []string{"nope"}, Since{})

	items, err := r.Next(context.Background(), time.Time{})

	if !errors.Is(err, filter.ErrRefused) || len(items) != 0 || len(s.arms) != 0 {
		t.Errorf("Next = %s, %v after %d arms, want ErrRefused before any arm", describe(items), err, len(s.arms))
	}
}

func TestItem_IDNamesTheEventOrThePositionOfAGap(t *testing.T) {
	t.Parallel()
	ev := sealed(41)
	tests := []struct {
		name string
		item Item
		want string
	}{
		{"signal", Item{Channel: "cycle", Record: ev}, "4242.41.2026-10-09T17:46:41.000Z"},
		{"stored gap", Item{Channel: "loop", Record: channel.Record{Cursor: 77, Source: "loop", Gap: &channel.Gap{From: 0, To: 0}}}, "gap.loop.77"},
		{"synthetic gap", Item{Channel: "loop", Record: channel.Record{Source: channel.SourceWatch, Gap: &channel.Gap{From: 12, To: 90}}}, "gap.loop.12"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.item.ID(); got != tc.want {
				t.Errorf("ID = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReader_AGapComesFirstAndABadTimeComesLastInTheMerge(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	bad := sealed(3)
	bad.Signal.TS = "yesterday"
	bad.Signal.Severity = signalcenter.SeverityWarn
	warn := sealed(1)
	warn.Signal.PID = 5151
	warn.Signal.Severity = signalcenter.SeverityWarn
	f.append("errors", bad, warn)
	if err := os.MkdirAll(f.dir("cycle"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.dir("cycle"), "seg-00000000000000000000.ndjson"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.append("cycle", sealed(5))
	r := f.reader(s, []string{"cycle", "errors"}, Since{mode: sinceAll})

	items := drain(t, r)

	want := "cycle:gap(malformed 0->2) cycle:cycle.sealed#5 errors:cycle.sealed#3 errors:cycle.sealed#1"
	if describe(items) != want {
		t.Errorf("items = %s, want %s", describe(items), want)
	}
}

func TestReader_EachItemCarriesTheCursorAfterIt(t *testing.T) {
	t.Parallel()
	f, s := newFixture(t), newScript()
	f.append("cycle", sealed(1))
	path := f.segments("cycle")[0].Path
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, "x\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	f.append("cycle", sealed(2))
	r := f.reader(s, []string{"cycle"}, Since{mode: sinceAll})

	items := drain(t, r)

	end, _ := f.log("cycle").End()
	first := int64(len(raw))
	want := []int64{first, first + 2, end}
	got := []int64{items[0].Next, items[1].Next, items[2].Next}
	if describe(items) != "cycle:cycle.sealed#1 cycle:gap(malformed "+strconv.FormatInt(first, 10)+"->"+strconv.FormatInt(first+2, 10)+") cycle:cycle.sealed#2" || !slices.Equal(got, want) {
		t.Errorf("items = %s with next %v, want next %v", describe(items), got, want)
	}
}
