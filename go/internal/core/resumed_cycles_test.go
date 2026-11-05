package core

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var resumeEpoch = time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)

func streamPathOf(root string) string {
	return filepath.Join(root, ".evolve", signalcenter.StreamFileName)
}

func emitResumeAt(t *testing.T, root string, at time.Time, cycle int) {
	t.Helper()
	path := streamPathOf(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	c := signalcenter.New(signalcenter.WithClock(func() time.Time { return at }))
	c.Subscribe(c.NDJSONSink(func(int) string { return path }))
	EmitLaneResumed(c, &ResumePoint{CycleID: cycle, Phase: "build", AutoAttempts: 1, AutoMaxAttempts: 3}, []string{"todo"})
}

func appendStreamLine(t *testing.T, root, line string) {
	t.Helper()
	f, err := os.OpenFile(streamPathOf(root), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(line + "\n"); err != nil {
		t.Fatal(err)
	}
}

func TestResumedCycleIDs_ReturnsEachResumedCycleOnceInTheWindowSorted(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	emitResumeAt(t, root, resumeEpoch.Add(time.Minute), 9)
	emitResumeAt(t, root, resumeEpoch.Add(2*time.Minute), 4)
	emitResumeAt(t, root, resumeEpoch.Add(3*time.Minute), 9)

	ids, fails := ResumedCycleIDs(root, resumeEpoch, resumeEpoch.Add(time.Hour))

	if !reflect.DeepEqual(ids, []int{4, 9}) || len(fails) != 0 {
		t.Fatalf("ids %v, failures %v; want [4 9] and none", ids, fails)
	}
}

func TestResumedCycleIDs_TheWindowHoldsFromAndNotTo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	emitResumeAt(t, root, resumeEpoch.Add(-time.Nanosecond), 1)
	emitResumeAt(t, root, resumeEpoch, 2)
	emitResumeAt(t, root, resumeEpoch.Add(time.Hour-time.Nanosecond), 3)
	emitResumeAt(t, root, resumeEpoch.Add(time.Hour), 4)

	ids, _ := ResumedCycleIDs(root, resumeEpoch, resumeEpoch.Add(time.Hour))
	if !reflect.DeepEqual(ids, []int{2, 3}) {
		t.Errorf("bounded window ids %v, want [2 3]", ids)
	}
	open, _ := ResumedCycleIDs(root, resumeEpoch, time.Time{})
	if !reflect.DeepEqual(open, []int{2, 3, 4}) {
		t.Errorf("a zero To has no end, ids %v, want [2 3 4]", open)
	}
}

func TestResumedCycleIDs_OnlyTheLaneResumedEventCounts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	emitResumeAt(t, root, resumeEpoch.Add(time.Minute), 5)
	appendStreamLine(t, root, `{"ts":"`+resumeEpoch.Add(time.Minute).Format(time.RFC3339Nano)+`","cycle":6,"kind":"loop.wave","code":"ORCHESTRATOR_LANE_DEADLINE"}`)

	ids, _ := ResumedCycleIDs(root, resumeEpoch, time.Time{})
	if !reflect.DeepEqual(ids, []int{5}) {
		t.Fatalf("ids %v, want [5]", ids)
	}
}

func TestResumedCycleIDs_AnAbsentStreamIsEmptyAndFailsNothing(t *testing.T) {
	t.Parallel()
	ids, fails := ResumedCycleIDs(t.TempDir(), resumeEpoch, time.Time{})
	if len(ids) != 0 || len(fails) != 0 {
		t.Fatalf("ids %v, failures %v", ids, fails)
	}
}

func TestResumedCycleIDs_AnUnreadableStreamAndABadLineAreFailures(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(streamPathOf(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	_, fails := ResumedCycleIDs(dir, resumeEpoch, time.Time{})
	if len(fails) != 1 || !strings.Contains(fails[0].Error(), signalcenter.StreamFileName) {
		t.Errorf("unreadable stream failures %v, want one naming the stream", fails)
	}

	root := t.TempDir()
	emitResumeAt(t, root, resumeEpoch.Add(time.Minute), 5)
	appendStreamLine(t, root, "not json")
	ids, fails := ResumedCycleIDs(root, resumeEpoch, time.Time{})
	if !reflect.DeepEqual(ids, []int{5}) || len(fails) != 1 || !strings.Contains(fails[0].Error(), "malformed") {
		t.Errorf("bad line: ids %v, failures %v; want [5] and one malformed failure", ids, fails)
	}
}

func TestResumedCycleIDs_ABadStampIsAFailureNotAMembership(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	emitResumeAt(t, root, resumeEpoch.Add(time.Minute), 5)
	appendStreamLine(t, root, `{"ts":"yesterday","cycle":8,"kind":"loop.wave","code":"ORCHESTRATOR_LANE_RESUMED"}`)

	ids, fails := ResumedCycleIDs(root, resumeEpoch, time.Time{})
	if !reflect.DeepEqual(ids, []int{5}) || len(fails) != 1 || !strings.Contains(fails[0].Error(), "malformed") {
		t.Fatalf("ids %v, failures %v", ids, fails)
	}
}
