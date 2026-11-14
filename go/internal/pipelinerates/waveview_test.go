package pipelinerates

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func ladderLine(class string) string {
	return `{"schema_version":"signal/1.0","seq":1,"ts":"2026-11-13T10:30:00Z","module":"ladder","kind":"ladder.decision","code":"LADDER_DECISION","severity":"INFO","origin":"t","reason":"r","fields":{"class":"` + class + `"}}`
}

func shipLandedAt(ts string) string {
	return `{"schema_version":"signal/1.0","seq":1,"ts":"` + ts + `","module":"orchestrator","kind":"ship.landed","code":"","severity":"INFO","origin":"t","reason":"landed","fields":{}}`
}

var buildPhaseLines = []string{
	`{"ts":"2026-11-13T12:00:00Z","phase":"build","kind":"phase.dispatched"}`,
	`{"ts":"2026-11-13T13:00:00Z","phase":"build","kind":"phase.outcome","fields":{"verdict":"PASS"}}`,
}

func viewFixture(t *testing.T) (string, time.Time) {
	t.Helper()
	root := flowFixture(t)
	evolve := filepath.Join(root, ".evolve")
	run := filepath.Join(evolve, "runs")
	writeJSON(t, filepath.Join(evolve, "waves", "wave-2.json"), map[string]any{"number": 2, "cycle_floor": 2, "pid": 12, "started_at": "2026-11-13T12:00:00Z"})
	writeJSON(t, filepath.Join(evolve, "waves", "wave-1.json"), map[string]any{"number": 1, "cycle_floor": 0, "pid": 11, "started_at": "2026-11-13T10:00:00Z", "ended_at": "2026-11-13T12:00:00Z"})
	writeLines(t, filepath.Join(evolve, "signals.ndjson"),
		orchEvent{ts: "2026-11-13T12:00:00Z", code: "LOOP_ORCH_POOL_FILLED", pid: 12, live: 2, target: 2}.line(),
		orchEvent{ts: "2026-11-13T13:00:00Z", code: "LOOP_ORCH_POOL_UNDERFILLED", pid: 12, live: 1, target: 2, cause: "run_cap"}.line(),
		orchEvent{ts: "2026-11-13T14:00:00Z", code: "LOOP_ORCH_POOL_UNDERFILLED", pid: 12, live: 1, target: 2, cause: "run_cap"}.line())
	writeJSON(t, filepath.Join(run, "cycle-1", "run.json"), map[string]any{"shipped": true})
	writeJSON(t, filepath.Join(run, "cycle-2", "run.json"), map[string]any{"shipped": false})
	writeJSON(t, filepath.Join(run, "cycle-3", "run.json"), map[string]any{"shipped": false})
	writeLines(t, filepath.Join(run, "cycle-3", "signals.ndjson"), buildPhaseLines...)
	writeLines(t, filepath.Join(run, "cycle-3", "llm-calls.ndjson"), `{"phase":"build","cli":"claude","started_at":"2026-11-13T12:00:00Z","ended_at":"2026-11-13T12:30:00Z"}`)
	return root, time.Date(2026, 11, 13, 20, 0, 0, 0, time.UTC)
}

func ptr(v float64) *float64 { return &v }

func TestWaveView_GivesPQ6ToPQ9AsPinnedValuesOfTheLatestWave(t *testing.T) {
	t.Parallel()
	root, now := viewFixture(t)
	writeLines(t, filepath.Join(root, ".evolve", "runs", "cycle-1", "signals.ndjson"), shipLandedAt("2026-11-10T10:00:00Z"))

	view, err := ReadWaveView(root, now)

	if err != nil {
		t.Fatal(err)
	}
	if view.Wave != 2 || view.Throughput != 1.0/7 {
		t.Errorf("wave %d, PQ6 %v; want wave 2 and one delivery in 7 days", view.Wave, view.Throughput)
	}
	if view.Utilization == nil || *view.Utilization != 0.75 || view.GateShare == nil || *view.GateShare != 0.5 || view.TopIdleShare == nil || *view.TopIdleShare != 1 || view.TopIdleCause != "run_cap" {
		t.Errorf("view = %+v, want PQ7 0.75, PQ9 0.5, PQ8 1 on run_cap", view)
	}
}

func TestWaveView_NamesTheLargestIdleCauseAndTheMostFrequentFailClassOfTheWaveAlone(t *testing.T) {
	t.Parallel()
	root, now := viewFixture(t)
	run := filepath.Join(root, ".evolve", "runs")
	writeJSON(t, filepath.Join(run, "cycle-1", "run.json"), map[string]any{"shipped": false})
	writeLines(t, filepath.Join(run, "cycle-1", "signals.ndjson"), ladderLine("y"), ladderLine("y"), ladderLine("y"))
	writeJSON(t, filepath.Join(run, "cycle-2", "run.json"), map[string]any{"shipped": true})
	writeLines(t, filepath.Join(run, "cycle-2", "signals.ndjson"), ladderLine("x"), ladderLine("x"), ladderLine("x"))
	writeLines(t, filepath.Join(run, "cycle-3", "signals.ndjson"), append(buildPhaseLines, ladderLine("spec"), ladderLine("capability"), ladderLine("spec"), ladderLine(""), ladderLine(""), ladderLine(""), "not json")...)

	view, err := ReadWaveView(root, now)

	if err != nil || view.Wave != 2 || view.TopIdleCause != "run_cap" || view.TopFailClass != "spec" {
		t.Errorf("view = %+v, %v; want run_cap and the class spec (2 of the open decisions of this wave)", view, err)
	}
	line := view.Line()
	if want := "wave 2 · PQ6 0.000/day · top idle cause run_cap · top fail class spec · PQ7 0.750 · PQ8 1.000 · PQ9 0.500"; !strings.Contains(line, want) {
		t.Errorf("line = %q, want %q", line, want)
	}
}

func TestWaveView_AReadingWithNoCountPrintsNotAvailableAndAMissingSampleStoreIsAnError(t *testing.T) {
	t.Parallel()
	line := WaveView{Wave: 3}.Line()
	if !strings.Contains(line, "PQ7 n/a") || !strings.Contains(line, "PQ8 n/a") || !strings.Contains(line, "PQ9 n/a") {
		t.Errorf("line = %q, want n/a for the readings that were never taken", line)
	}
	root, now := viewFixture(t)
	if err := os.Remove(filepath.Join(root, ".evolve", "signals.ndjson")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadWaveView(root, now); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want the absent sample store named as the panel names it", err)
	}
}

func TestWaveView_AWaveWithNoIdleAndNoFailureNamesNone(t *testing.T) {
	t.Parallel()
	view := WaveView{Wave: 3}

	if line := view.Line(); !strings.Contains(line, "top idle cause none") || !strings.Contains(line, "top fail class none") {
		t.Errorf("line = %q, want none for both", line)
	}
	if _, err := json.Marshal(view); err != nil {
		t.Fatal(err)
	}
}

func TestWaveView_ErrorsWhenNoWaveIsRecordedOrTheCyclesAreUnreadable(t *testing.T) {
	t.Parallel()
	if _, err := ReadWaveView(t.TempDir(), time.Now()); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want not exist", err)
	}
	root, now := viewFixture(t)
	writeLines(t, filepath.Join(root, ".evolve", "runs", "cycle-2", "run.json"), "{")
	if _, err := ReadWaveView(root, now); err == nil {
		t.Error("an unreadable cycle gave no error")
	}
}

func TestWaveView_TieOfFailClassesGoesToTheFirstNameAndTheCyclesOfOtherWavesAndShippedCyclesAreSkipped(t *testing.T) {
	t.Parallel()
	cycles := []Cycle{
		{Wave: 4, FailClasses: map[string]int{"b": 2, "a": 1}},
		{Wave: 4, FailClasses: map[string]int{"a": 1, "c": 1}},
		{Wave: 4, Shipped: true, FailClasses: map[string]int{"z": 9}},
		{Wave: 5, FailClasses: map[string]int{"y": 9}},
	}
	if got := topFailClass(cycles, 4); got != "a" {
		t.Errorf("topFailClass = %q, want a on the tie of a and b", got)
	}
	if got := topFailClass(nil, 4); got != "" {
		t.Errorf("topFailClass = %q, want empty with no cycles", got)
	}
	if causeOf("wave:4/cause:bench") != "bench" || causeOf("odd") != "" {
		t.Error("causeOf did not cut the cause name of the window")
	}
}

func TestWaveView_AWaveWithNoPoolSamplesHasNoPQ7AndNoPQ8ReadingYetKeepsPQ9(t *testing.T) {
	t.Parallel()
	root, now := viewFixture(t)
	writeLines(t, filepath.Join(root, ".evolve", "signals.ndjson"),
		orchEvent{ts: "2026-11-13T10:00:00Z", code: "LOOP_ORCH_POOL_FILLED", pid: 11, live: 2, target: 2}.line())

	view, err := ReadWaveView(root, now)

	if err != nil || view.Wave != 2 || view.Utilization != nil || view.TopIdleShare != nil || view.GateShare == nil || *view.GateShare != 0.5 {
		t.Errorf("view = %+v, %v; want PQ7 and PQ8 absent and PQ9 0.5", view, err)
	}
	if line := view.Line(); !strings.Contains(line, "PQ7 n/a · PQ8 n/a · PQ9 0.500") {
		t.Errorf("line = %q", line)
	}
}
