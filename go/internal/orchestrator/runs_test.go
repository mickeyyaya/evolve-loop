package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestDeadlineOf_IsTheNinetyFifthPercentileWithoutTheLongestLaneWithAFloor(t *testing.T) {
	var times []time.Duration
	for i := 1; i <= 20; i++ {
		times = append(times, time.Duration(i)*time.Minute)
	}
	cases := []struct {
		name  string
		times []time.Duration
		floor time.Duration
		want  time.Duration
	}{
		{"no history takes the floor", nil, time.Hour, time.Hour},
		{"a p95 under the floor takes the floor", times, time.Hour, time.Hour},
		{"a p95 over the floor wins", times, 10 * time.Minute, 19 * time.Minute},
		{"one sample is its own p95", []time.Duration{90 * time.Minute}, time.Hour, 90 * time.Minute},
		{"the longest lane of a small sample is dropped", times[:10], 0, 9 * time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DeadlineOf(tc.times, tc.floor); got != tc.want {
				t.Errorf("DeadlineOf = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDeadlineOf_DoesNotReorderItsInput(t *testing.T) {
	in := []time.Duration{3, 1, 2}
	DeadlineOf(in, 0)
	if in[0] != 3 || in[1] != 1 || in[2] != 2 {
		t.Errorf("input = %v, want it unchanged", in)
	}
}

func writeCycle(t *testing.T, runs string, cycle int, todos []string, stamps ...time.Time) string {
	t.Helper()
	dir := filepath.Join(runs, "cycle-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if todos != nil {
		scope, _ := json.Marshal(map[string]any{"todo_ids": todos})
		if err := os.WriteFile(filepath.Join(dir, committedset.LanePinFile), scope, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var lines []string
	for i, ts := range stamps {
		b, _ := json.Marshal(signalcenter.Event{Seq: uint64(i + 1), TS: ts.Format(time.RFC3339Nano), Cycle: cycle, Module: signalcenter.ModuleLoop, Kind: signalcenter.KindLoopWave, Severity: signalcenter.SeverityInfo})
		lines = append(lines, string(b))
	}
	if len(lines) > 0 {
		if err := os.WriteFile(filepath.Join(dir, signalcenter.StreamFileName), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLaneTimes_ReadsTheLastCyclesFromTheirSignals(t *testing.T) {
	runs := t.TempDir()
	writeCycle(t, runs, 7, nil, epoch, epoch.Add(10*time.Minute))
	writeCycle(t, runs, 8, nil, epoch, epoch.Add(5*time.Minute), epoch.Add(30*time.Minute))
	writeCycle(t, runs, 9, nil, epoch, epoch.Add(40*time.Minute))
	writeCycle(t, runs, 10, nil, epoch)
	writeCycle(t, runs, 11, nil)
	if err := os.MkdirAll(filepath.Join(runs, "not-a-cycle"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := LaneTimes(runs, 4)
	if err != nil {
		t.Fatal(err)
	}
	want := []time.Duration{30 * time.Minute, 40 * time.Minute}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("LaneTimes(last 4) = %v, want %v (cycles 8 to 11; 10 has one event and 11 none)", got, want)
	}
}

func TestLaneTimes_ANoRunsDirectoryIsAnError(t *testing.T) {
	if _, err := LaneTimes(filepath.Join(t.TempDir(), "missing"), 50); err == nil {
		t.Error("LaneTimes of a missing directory returned no error")
	}
}

func TestRunsHeartbeat_IsTheLastSignalOfTheCycleThatHoldsTheTodo(t *testing.T) {
	runs := t.TempDir()
	writeCycle(t, runs, 20, []string{"other"}, epoch, epoch.Add(50*time.Minute))
	writeCycle(t, runs, 21, []string{"A", "B"}, epoch, epoch.Add(12*time.Minute))
	beat := RunsHeartbeat(runs)

	got, ok := beat(Lane{Key: "A+B", Scope: []string{"A", "B"}, Started: epoch.Add(-time.Hour)})
	if !ok || !got.Equal(epoch.Add(12*time.Minute)) {
		t.Errorf("heartbeat = %v, %v, want %v", got, ok, epoch.Add(12*time.Minute))
	}
	writeCycle(t, runs, 21, []string{"A", "B"}, epoch, epoch.Add(12*time.Minute), epoch.Add(14*time.Minute))
	if got, _ := beat(Lane{Key: "A+B", Scope: []string{"A", "B"}, Started: epoch.Add(-time.Hour)}); !got.Equal(epoch.Add(14 * time.Minute)) {
		t.Errorf("heartbeat after a new event = %v, want the new event", got)
	}
	if _, ok := beat(Lane{Key: "none", Scope: []string{"none"}, Started: epoch.Add(-time.Hour)}); ok {
		t.Error("a todo that no cycle holds has a heartbeat")
	}
}

func TestRunsHeartbeat_SkipsACycleOlderThanTheLane(t *testing.T) {
	runs := t.TempDir()
	dir := writeCycle(t, runs, 30, []string{"A"}, epoch)
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(dir, old, old); err != nil {
		t.Fatal(err)
	}
	if _, ok := RunsHeartbeat(runs)(Lane{Key: "A", Scope: []string{"A"}, Started: time.Now().Add(-time.Hour)}); ok {
		t.Error("an earlier cycle of the same todo gave the new lane a heartbeat")
	}
}

func TestRunsHeartbeat_AMissingRunsDirectoryHasNoHeartbeat(t *testing.T) {
	if _, ok := RunsHeartbeat(filepath.Join(t.TempDir(), "missing"))(Lane{Key: "A", Scope: []string{"A"}}); ok {
		t.Error("a missing runs directory gave a heartbeat")
	}
}

func TestRunsHeartbeat_SkipsACycleWithNoScopeAndALineWithNoTime(t *testing.T) {
	runs := t.TempDir()
	writeCycle(t, runs, 41, nil, epoch)
	dir := writeCycle(t, runs, 40, []string{"A"}, epoch.Add(3*time.Minute))
	f, err := os.OpenFile(filepath.Join(dir, signalcenter.StreamFileName), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"seq":9,"ts":"not a time","module":"loop","kind":"loop.wave","severity":"INFO"}` + "\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	got, ok := RunsHeartbeat(runs)(Lane{Key: "A", Scope: []string{"A"}, Started: epoch.Add(-time.Hour)})
	if !ok || !got.Equal(epoch.Add(3*time.Minute)) {
		t.Errorf("heartbeat = %v, %v, want the last timed event of cycle 40", got, ok)
	}
}

func TestDeadlineOf_TheLiveSamplesOfPlanQ1GiveNinetyFiveMinutes(t *testing.T) {
	minutes := []float64{8.89, 15.20, 16.20, 21.51, 22.16, 22.70, 23.05, 23.51, 24.97, 26.14, 29.07, 29.42, 29.52, 29.86, 30.08,
		32.01, 33.11, 33.12, 37.04, 37.67, 38.12, 38.14, 39.00, 39.76, 40.69, 41.63, 41.81, 43.06, 50.05, 56.00, 57.28, 62.40,
		64.35, 67.93, 68.38, 72.03, 77.50, 79.59, 83.55, 92.98, 95.14, 98.86, 110.57, 118.06}
	times := make([]time.Duration, len(minutes))
	for i, m := range minutes {
		times[i] = time.Duration(m * float64(time.Minute))
	}
	if got := DeadlineOf(times, time.Hour); got != times[40] {
		t.Errorf("DeadlineOf(44 live samples) = %v, want %v: the p95 of the 43 samples without the longest lane", got, times[40])
	}
}

func touch(t *testing.T, path string, at time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	setTime(t, path, at)
}

func TestRunsHeartbeat_IsTheFreshestOfTheSignalsAndThePhaseFiles(t *testing.T) {
	runs := t.TempDir()
	dir := writeCycle(t, runs, 50, []string{"A"}, epoch)
	touch(t, filepath.Join(dir, "build-pane-watch.json"), epoch.Add(25*time.Minute))
	touch(t, filepath.Join(dir, "build-events.ndjson"), epoch.Add(20*time.Minute))
	touch(t, filepath.Join(dir, "build-stdout.log"), epoch.Add(22*time.Minute))
	touch(t, filepath.Join(dir, "build-report.md"), epoch.Add(40*time.Minute))
	setTime(t, dir, epoch)

	got, ok := RunsHeartbeat(runs)(Lane{Key: "A", Scope: []string{"A"}, Started: epoch.Add(-time.Minute)})
	if !ok || !got.Equal(epoch.Add(25*time.Minute)) {
		t.Errorf("heartbeat = %v, %v, want the pane-watch time %v (a report file is no heartbeat)", got, ok, epoch.Add(25*time.Minute))
	}
}

func TestRunsHeartbeat_AFoundCycleWithNoSignalIsKnownButHasNoTime(t *testing.T) {
	runs := t.TempDir()
	writeCycle(t, runs, 51, []string{"A"})
	got, ok := RunsHeartbeat(runs)(Lane{Key: "A", Scope: []string{"A"}, Started: time.Now().Add(-time.Minute)})
	if !ok || !got.IsZero() {
		t.Errorf("heartbeat = %v, %v, want known with no time: the lease then counts from the lane start", got, ok)
	}
}

func TestLease_AFreshPaneWatchWithNoSignalFor25MinutesIsNotStalled(t *testing.T) {
	runs := t.TempDir()
	dir := writeCycle(t, runs, 52, []string{"A"}, epoch)
	touch(t, filepath.Join(dir, "build-pane-watch.json"), epoch.Add(25*time.Minute))
	setTime(t, dir, epoch)
	clock, events, g := newFakeClock(), &eventLog{}, newGatedLaunch()
	leases := NewLeases(LeaseConfig{Stage: StageEnforce, Deadline: 5 * time.Hour, Stall: 30 * time.Minute}, clock, RunsHeartbeat(runs), events.publish)
	done := runLane(leases.Watch(g.launch, nil), "A")
	waitStart(t, g)

	clock.Advance(31 * time.Minute)
	if got := events.withCode(CodeLaneStalled); len(got) != 0 {
		t.Errorf("stall events = %+v, want none: the pane changed at 25m although the last signal is at 0m", got)
	}
	clock.Advance(25 * time.Minute)
	if got := events.withCode(CodeLaneStalled); len(got) != 1 {
		t.Errorf("stall events = %+v at 56m, want one: 30m past the pane-watch time", got)
	}
	g.finish("A", 0)
	<-done
}

func TestLease_ALaneWhoseCycleIsNotFoundEmitsNoStall(t *testing.T) {
	clock, events, g := newFakeClock(), &eventLog{}, newGatedLaunch()
	leases := NewLeases(LeaseConfig{Stage: StageEnforce, Deadline: 5 * time.Hour, Stall: 30 * time.Minute}, clock, RunsHeartbeat(t.TempDir()), events.publish)
	done := runLane(leases.Watch(g.launch, nil), "A")
	waitStart(t, g)

	clock.Advance(2 * time.Hour)
	if got := events.withCode(CodeLaneStalled); len(got) != 0 {
		t.Errorf("stall events = %+v, want none for a lane whose cycle directory is not found", got)
	}
	g.finish("A", 0)
	<-done
}

func setTime(t *testing.T, path string, at time.Time) {
	t.Helper()
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

func TestLaneCycle_IsTheNumberOfTheCycleThatHoldsTheTodo(t *testing.T) {
	runs := t.TempDir()
	writeCycle(t, runs, 60, []string{"other"})
	writeCycle(t, runs, 61, []string{"A"})
	lane := Lane{Key: "A", Scope: []string{"A"}, Started: time.Now().Add(-time.Minute)}
	if got, ok := LaneCycle(runs, lane); !ok || got != 61 {
		t.Errorf("LaneCycle = %d, %v, want 61", got, ok)
	}
	if _, ok := LaneCycle(runs, Lane{Key: "B", Scope: []string{"B"}}); ok {
		t.Error("a todo that no cycle holds has a cycle")
	}
}
