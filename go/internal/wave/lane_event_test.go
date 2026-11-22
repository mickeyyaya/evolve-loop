package wave_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func streamAt(minutes ...int) string {
	body := ""
	for _, m := range minutes {
		body += fmt.Sprintf(`{"schema_version":"signal/1.0","seq":1,"pid":1,"ts":%q,"module":"x","origin":"x","kind":"k","severity":"INFO","reason":"r"}`+"\n", clock(m).Format(time.RFC3339Nano))
	}
	return body
}

func laneRoot(t *testing.T) plane {
	t.Helper()
	p := newPlane(t)
	for id, minutes := range map[int][]int{1: {10, 40}, 2: {30}, 3: {90}} {
		p.write(filepath.Join(".evolve", "runs", fmt.Sprintf("cycle-%d", id), signalcenter.StreamFileName), streamAt(minutes...))
	}
	return p
}

func TestLaneLastEvent_TheLatestSignalOfTheLaneUpToNow(t *testing.T) {
	t.Parallel()
	p := laneRoot(t)
	read := wave.WaveRead{Cycles: []wave.Cycle{{ID: 1, Todo: "a"}, {ID: 2, Todo: "a"}, {ID: 3, Todo: "b"}, {ID: 4, Todo: "a"}}}
	cases := []struct {
		name string
		loop []signalcenter.Event
		lane string
		now  int
		want time.Time
	}{
		{"no event", nil, "none", 100, time.Time{}},
		{"the later signal of two cycles of the lane wins", nil, "a", 100, clock(40)},
		{"a cycle of another lane does not count", nil, "c", 100, time.Time{}},
		{"a later loop signal of the lane beats the cycle signals", []signalcenter.Event{poolEvent(60, "LOOP_ORCH_LANE_EXITED", "a")}, "a", 100, clock(60)},
		{"a later cycle signal beats an earlier loop signal", []signalcenter.Event{poolEvent(20, "LOOP_ORCH_LANE_EXITED", "a")}, "a", 100, clock(40)},
		{"a deadline line is a watchdog line, not lane activity", []signalcenter.Event{poolEvent(70, "LOOP_ORCH_LANE_DEADLINE", "a")}, "a", 100, clock(40)},
		{"a stall line is not an event of its lane", []signalcenter.Event{poolEvent(70, "LOOP_ORCH_LANE_STALLED", "a")}, "a", 100, clock(40)},
		{"a loop signal of another lane does not count", []signalcenter.Event{poolEvent(70, "LOOP_ORCH_LANE_EXITED", "b")}, "c", 100, time.Time{}},
		{"a loop signal after now does not count", []signalcenter.Event{poolEvent(101, "LOOP_ORCH_LANE_EXITED", "a")}, "a", 100, clock(40)},
		{"a cycle signal after now does not count", nil, "b", 80, time.Time{}},
	}
	for _, c := range cases {
		read.Loop = c.loop
		if got := read.LaneLastEvent(p.root, c.lane, clock(c.now)); !got.Equal(c.want) {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestLaneLastEvent_APhaseFileOfTheLaneCycleIsAnEventAsTheLeaseCountsIt(t *testing.T) {
	t.Parallel()
	p := laneRoot(t)
	stdout := filepath.Join(paths.RunWorkspace(p.root, 1), "build-stdout.log")
	p.write(filepath.Join(".evolve", "runs", "cycle-1", "build-stdout.log"), "x")
	if err := os.Chtimes(stdout, clock(70), clock(70)); err != nil {
		t.Fatal(err)
	}
	read := wave.WaveRead{Cycles: []wave.Cycle{{ID: 1, Todo: "a"}}}
	if got := read.LaneLastEvent(p.root, "a", clock(100)); !got.Equal(clock(70)) {
		t.Errorf("got %s, want the stdout time %s", got, clock(70))
	}
	if got := read.LaneLastEvent(p.root, "a", clock(50)); !got.Equal(clock(40)) {
		t.Errorf("a file time after now must not count: got %s, want %s", got, clock(40))
	}
}
