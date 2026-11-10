package mainstop

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
)

var (
	dupKey   = baseattr.Key{Package: "internal/dup", Test: "TestCount"}
	sizeKey  = baseattr.Key{Package: "internal/size", Test: "TestSize"}
	otherKey = baseattr.Key{Package: "internal/other", Test: "TestOther"}
	opened   = time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
)

const (
	mainSHA = "9a2bbe0e1c0ffee"
	nextSHA = "fe9550c22c0ffee"
)

func openStop(keys ...baseattr.Key) Stop {
	return Stop{}.Opened(baseattr.Verdict{MainRed: keys, Main: mainSHA}, FixItemID(mainSHA), opened)
}

func TestStop_AdmitsOnlyAFixOnTheMainTip(t *testing.T) {
	t.Parallel()
	stop := openStop(dupKey, sizeKey)
	cases := []struct {
		name  string
		stop  Stop
		proof Proof
		want  Admission
	}{
		{"no stop", Stop{}, Proof{Base: "stale"}, AdmitNoStop},
		{"a released stop", stop.Released([]baseattr.Key{dupKey, sizeKey}, "cycle 7", opened), Proof{Base: "stale"}, AdmitNoStop},
		{"a full fix on a stale base", stop, Proof{Base: "stale", Green: []baseattr.Key{dupKey, sizeKey}}, RefuseRebase},
		{"a partial fix on a stale base", stop, Proof{Base: "stale", Green: []baseattr.Key{dupKey}}, RefuseRebase},
		{"a partial fix on the tip", stop, Proof{Base: mainSHA, Green: []baseattr.Key{dupKey}}, AdmitFix},
		{"a fix on the tip with a lane red", stop, Proof{Base: mainSHA, Green: []baseattr.Key{dupKey}, LaneOnly: []baseattr.Key{otherKey}}, RefuseNotAFix},
		{"no stop key green on the tip", stop, Proof{Base: mainSHA, Green: []baseattr.Key{otherKey}}, RefuseNotAFix},
		{"nothing green on the tip", stop, Proof{Base: mainSHA}, RefuseNotAFix},
	}
	for _, c := range cases {
		if got := c.stop.Admit(mainSHA, c.proof); got != c.want {
			t.Errorf("%s: Admit = %d, want %d", c.name, got, c.want)
		}
	}
	if got := stop.Admit("", Proof{Green: []baseattr.Key{dupKey}}); got != RefuseRebase {
		t.Errorf("an unread main tip: Admit = %d, want %d", got, RefuseRebase)
	}
}

func TestStop_OpenedStartsAStopWithTheMainRedKeysAndTheFixItem(t *testing.T) {
	t.Parallel()
	stop := openStop(dupKey)
	if !stop.Holds() || stop.Schema != Schema || stop.MainSHA != mainSHA || !stop.OpenedAt.Equal(opened) || stop.FixItem != "fix-main-red-9a2bbe0e" {
		t.Errorf("Opened = %+v, want a held stop on %s with the fix item of that commit", stop, mainSHA)
	}
	if !slices.Equal(stop.Keys, []baseattr.Key{dupKey}) {
		t.Errorf("keys = %v, want the main-red key", stop.Keys)
	}
	if (Stop{}).Holds() || (Stop{}).Opened(baseattr.Verdict{Main: mainSHA}, "x", opened).Holds() {
		t.Error("a stop with no key holds")
	}
}

func TestStop_ARefreshMergesTheKeysAndNeverMovesTheOpenTime(t *testing.T) {
	t.Parallel()
	first := openStop(dupKey).WithParked(Parked{Cycle: 2249}).Escalate(opened)
	later := opened.Add(time.Hour)
	next := first.Opened(baseattr.Verdict{MainRed: []baseattr.Key{sizeKey, dupKey}, Main: nextSHA}, "other-item", later)
	if !slices.Equal(next.Keys, []baseattr.Key{dupKey, sizeKey}) || next.MainSHA != nextSHA {
		t.Errorf("refresh = keys %v on %s, want both keys once on the new tip", next.Keys, next.MainSHA)
	}
	if !next.OpenedAt.Equal(opened) || next.FixItem != first.FixItem || len(next.Parked) != 1 || next.Escalated == nil {
		t.Errorf("refresh = %+v, want the open time, the fix item, the parked row and the escalation of the open stop", next)
	}
	if !slices.Equal(first.Keys, []baseattr.Key{dupKey}) {
		t.Errorf("the refreshed stop changed the keys of its source: %v", first.Keys)
	}
	held := openStop(dupKey, sizeKey).Released([]baseattr.Key{sizeKey}, "cycle 7", later)
	one := held.Opened(baseattr.Verdict{MainRed: []baseattr.Key{otherKey}}, "x", later)
	held.Opened(baseattr.Verdict{MainRed: []baseattr.Key{sizeKey}}, "x", later)
	if !slices.Equal(one.Keys, []baseattr.Key{dupKey, otherKey}) {
		t.Errorf("a second refresh of one stop changed the keys of the first refresh: %v", one.Keys)
	}
	verdictKeys := []baseattr.Key{dupKey}
	started := Stop{}.Opened(baseattr.Verdict{MainRed: verdictKeys, Main: mainSHA}, "x", opened)
	verdictKeys[0] = otherKey
	if !slices.Equal(started.Keys, []baseattr.Key{dupKey}) {
		t.Errorf("a change to the verdict keys changed the stop: %v", started.Keys)
	}
	if kept := first.Opened(baseattr.Verdict{MainRed: []baseattr.Key{sizeKey}}, "x", later); kept.MainSHA != mainSHA {
		t.Errorf("a refresh with an unread main moved the tip to %q", kept.MainSHA)
	}
	released := first.Released([]baseattr.Key{dupKey}, "cycle 7", later)
	again := released.Opened(baseattr.Verdict{MainRed: []baseattr.Key{sizeKey}, Main: nextSHA}, "new-item", later)
	if !again.OpenedAt.Equal(later) || again.FixItem != "new-item" || again.ReleasedAt != nil || len(again.Parked) != 0 || again.Escalated != nil {
		t.Errorf("Opened after a release = %+v, want a new stop", again)
	}
}

func TestStop_WithParkedKeepsOneRowForEachCycleInCycleOrder(t *testing.T) {
	t.Parallel()
	first := openStop(dupKey).WithParked(Parked{Cycle: 2249, Workspace: "old"})
	next := first.WithParked(Parked{Cycle: 2242}).WithParked(Parked{Cycle: 2249, Workspace: "new"})
	if len(next.Parked) != 2 || next.Parked[0].Cycle != 2242 || next.Parked[1].Workspace != "new" {
		t.Errorf("parked = %+v, want cycle 2242 and then the new row of cycle 2249", next.Parked)
	}
	if len(first.Parked) != 1 || first.Parked[0].Workspace != "old" {
		t.Errorf("WithParked changed the rows of its source: %+v", first.Parked)
	}
	two := openStop(dupKey).WithParked(Parked{Cycle: 1, Workspace: "a"}).WithParked(Parked{Cycle: 2, Workspace: "b"})
	two.WithParked(Parked{Cycle: 1, Workspace: "z"})
	if two.Parked[0].Workspace != "a" || two.Parked[1].Cycle != 2 {
		t.Errorf("a second park of a cycle changed the rows of its source: %+v", two.Parked)
	}
}

func TestStop_ReleasedDropsTheGreenKeysAndEndsTheStopWithTheLastKey(t *testing.T) {
	t.Parallel()
	stop := openStop(dupKey, sizeKey)
	later := opened.Add(time.Hour)
	partial := stop.Released([]baseattr.Key{dupKey, otherKey}, "cycle 7", later)
	if !partial.Holds() || !slices.Equal(partial.Keys, []baseattr.Key{sizeKey}) || partial.ReleasedBy != "" {
		t.Errorf("a partial release = %+v, want a held stop with the size key", partial)
	}
	if !slices.Equal(stop.Keys, []baseattr.Key{dupKey, sizeKey}) {
		t.Errorf("Released changed the keys of its source: %v", stop.Keys)
	}
	done := partial.Released([]baseattr.Key{sizeKey}, "main fe9550c2", later)
	if done.Holds() || done.ReleasedAt == nil || !done.ReleasedAt.Equal(later) || done.ReleasedBy != "main fe9550c2" {
		t.Errorf("the last release = %+v, want a released stop with its time and its source", done)
	}
	if again := done.Released([]baseattr.Key{sizeKey}, "other", later.Add(time.Hour)); again.ReleasedBy != "main fe9550c2" || !again.ReleasedAt.Equal(later) {
		t.Errorf("a release of a released stop = %+v, want no change", again)
	}
}

func TestStop_IsOverdueOnlyPastAPositiveBoundWhileItHolds(t *testing.T) {
	t.Parallel()
	stop := openStop(dupKey)
	bound := 2 * time.Hour
	cases := []struct {
		name  string
		stop  Stop
		now   time.Time
		bound time.Duration
		want  bool
	}{
		{"at the bound", stop, opened.Add(bound), bound, false},
		{"past the bound", stop, opened.Add(bound + time.Second), bound, true},
		{"no bound", stop, opened.Add(100 * bound), 0, false},
		{"released", stop.Released([]baseattr.Key{dupKey}, "x", opened), opened.Add(3 * bound), bound, false},
	}
	for _, c := range cases {
		if got := c.stop.Overdue(c.now, c.bound); got != c.want {
			t.Errorf("%s: Overdue = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestStop_EscalateRecordsTheFirstTimeOnly(t *testing.T) {
	t.Parallel()
	stop := openStop(dupKey)
	first := stop.Escalate(opened.Add(time.Hour))
	second := first.Escalate(opened.Add(2 * time.Hour))
	if stop.Escalated != nil || first.Escalated == nil || !second.Escalated.Equal(opened.Add(time.Hour)) {
		t.Errorf("Escalate = %v then %v, want the first time kept and the source unchanged", first.Escalated, second.Escalated)
	}
}

func TestStop_CauseNamesTheKeysTheTipTheOpenTimeAndTheParkedCycles(t *testing.T) {
	t.Parallel()
	stop := openStop(dupKey, sizeKey)
	if cause := stop.Cause(); !strings.Contains(cause, "parked cycles: none") {
		t.Errorf("Cause = %q, want no parked cycle", cause)
	}
	cause := stop.WithParked(Parked{Cycle: 2249}).WithParked(Parked{Cycle: 2242}).Cause()
	for _, want := range []string{"internal/dup.TestCount, internal/size.TestSize", mainSHA, "2026-11-09T10:00:00Z", "parked cycles: 2242, 2249"} {
		if !strings.Contains(cause, want) {
			t.Errorf("Cause = %q, want it to hold %q", cause, want)
		}
	}
	if got := stop.ParkedCycles(); got != "" {
		t.Errorf("ParkedCycles of a stop with no parked cycle = %q, want the empty string", got)
	}
	if got := stop.WithParked(Parked{Cycle: 2249}).WithParked(Parked{Cycle: 2242}).ParkedCycles(); got != "2242, 2249" {
		t.Errorf("ParkedCycles = %q, want the cycles in order", got)
	}
	if resume := stop.ResumeCondition(); !strings.Contains(resume, "green on main") || !strings.Contains(resume, "next pool fill") {
		t.Errorf("ResumeCondition = %q, want the green keys and the pool fill", resume)
	}
}

func TestFixItemID_UsesTheFirstEightOfTheCommit(t *testing.T) {
	t.Parallel()
	if got := FixItemID(mainSHA); got != "fix-main-red-9a2bbe0e" {
		t.Errorf("FixItemID = %q", got)
	}
	if got := FixItemID("abc"); got != "fix-main-red-abc" {
		t.Errorf("FixItemID of a short commit = %q", got)
	}
}

func TestStop_ResumesParkedLanesWhenItEndsOrMainMoves(t *testing.T) {
	t.Parallel()
	stop := openStop(dupKey)
	cases := []struct {
		name string
		stop Stop
		main string
		want bool
	}{
		{"the stop holds on the main tip", stop, mainSHA, false},
		{"the main tip is unknown", stop, "", false},
		{"main moved", stop, nextSHA, true},
		{"released", stop.Released([]baseattr.Key{dupKey}, "x", opened), mainSHA, true},
		{"released and the main tip is unknown", stop.Released([]baseattr.Key{dupKey}, "x", opened), "", true},
		{"no stop", Stop{}, mainSHA, true},
	}
	for _, c := range cases {
		if got := c.stop.Resumes(c.main); got != c.want {
			t.Errorf("%s: Resumes = %v, want %v", c.name, got, c.want)
		}
	}
}
