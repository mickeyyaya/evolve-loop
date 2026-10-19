package main

import (
	"encoding/json"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/lanetime"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

var latencyFixture = filepath.Join("..", "..", "internal", "lanetime", "testdata", "evolve")

func latencyPlane(t *testing.T) wavePlane {
	t.Helper()
	p := newWavePlane(t)
	err := filepath.WalkDir(latencyFixture, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(latencyFixture, path)
		body, err := os.ReadFile(path)
		if err == nil {
			p.write(filepath.Join(".evolve", rel), string(body))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	end := time.Date(2026, 10, 19, 11, 30, 0, 0, time.UTC)
	for _, r := range []wave.Record{
		{Number: 1, RunID: "r1", CycleFloor: 0, StartedAt: time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC), EndedAt: &end},
		{Number: 2, RunID: "r2", CycleFloor: 3, StartedAt: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)},
	} {
		if err := p.store().Save(r); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func decodeLatency(t *testing.T, h *waveHarness, rc int) waveLatencyReport {
	t.Helper()
	var got waveLatencyReport
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil || rc != 0 {
		t.Fatalf("rc = %d, decode %v:\nstdout %s\nstderr %s", rc, err, h.stdout.String(), h.stderr.String())
	}
	return got
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestWaveLatency_AWaveReportsTheAverageLiveLanesAndThePhaseShares(t *testing.T) {
	t.Parallel()
	p := latencyPlane(t)
	h := newWaveHarness()

	rc := h.run("latency", "--wave", "1", "--json", "--project-root", p.root)

	got := decodeLatency(t, h, rc)
	if got.Wave != 1 || got.Cycles != 3 || got.FirstCycle != 1 || got.LastCycle != 3 {
		t.Errorf("scope = wave %d cycles %d (%d-%d), want wave 1, 3 cycles (1-3)", got.Wave, got.Cycles, got.FirstCycle, got.LastCycle)
	}
	if !near(got.Slots.AvgLive, 2.0) || !near(got.Slots.UsedSlotHours, 3.0) || !near(got.Slots.CapacitySlotHours, 6.0) || !near(got.Slots.Utilization, 0.5) {
		t.Errorf("slots = %+v, want avg live 2.0, 3.0 of 6.0 slot-hours, 50%%", got.Slots)
	}
	build := got.Phases.Phases[0]
	if build.Phase != "build" || !near(build.Share, 110.0/165.0) || build.Model != 85*time.Minute || build.Host != 25*time.Minute {
		t.Errorf("largest phase = %+v, want build at 110/165 with 85m model and 25m host", build)
	}
	if got.Phases.Lane != 165*time.Minute || got.Phases.Gap != 5*time.Minute {
		t.Errorf("lane %v gap %v, want 165m and 5m", got.Phases.Lane, got.Phases.Gap)
	}
}

func TestWaveLatency_TheFamilyMeasureCoversTheLastTwentyBuildsOfTheWholeHistoryNotTheWave(t *testing.T) {
	t.Parallel()
	p := latencyPlane(t)
	h := newWaveHarness()

	rc := h.run("latency", "--wave", "1", "--json", "--project-root", p.root)

	got := decodeLatency(t, h, rc)
	want := []lanetime.FamilyLane{
		{Family: "agy", Builds: 1, Passes: 1, Lane: 70 * time.Minute, PerPass: 70 * time.Minute},
		{Family: "claude", Builds: 3, Passes: 2, Lane: 125 * time.Minute, PerPass: 62*time.Minute + 30*time.Second},
	}
	if !reflect.DeepEqual(got.Families, want) {
		t.Errorf("families = %+v, want %+v", got.Families, want)
	}
}

func TestWaveLatency_WithoutFlagsItReportsTheLatestWaveAndItsOpenEnd(t *testing.T) {
	t.Parallel()
	p := latencyPlane(t)
	h := newWaveHarness()

	rc := h.run("latency", "--json", "--project-root", p.root)

	got := decodeLatency(t, h, rc)
	if got.Wave != 2 || got.FirstCycle != 5 || got.LastCycle != 6 || got.Cycles != 2 {
		t.Errorf("scope = wave %d cycles %d-%d (%d), want wave 2 cycles 5-6 (2)", got.Wave, got.FirstCycle, got.LastCycle, got.Cycles)
	}
	if got.Slots.Window != 0 {
		t.Errorf("slots = %+v, want none: no orchestration sample lies in the window of wave 2", got.Slots)
	}
}

func TestWaveLatency_ACycleRangeDerivesItsWindowFromTheCycles(t *testing.T) {
	t.Parallel()
	p := latencyPlane(t)
	h := newWaveHarness()

	rc := h.run("latency", "--cycles", "2-3", "--json", "--project-root", p.root)

	got := decodeLatency(t, h, rc)
	if got.Wave != 0 || got.Cycles != 2 || !near(got.Slots.UsedSlotHours, 3.0) || !near(got.Slots.CapacitySlotHours, 7.0) || !near(got.Slots.AvgLive, 3.0/1.75) {
		t.Errorf("report = wave %d cycles %d slots %+v, want 2 cycles, 3.0 of 7.0 slot-hours, avg 3/1.75", got.Wave, got.Cycles, got.Slots)
	}
}

func TestWaveLatency_ARangeWithNoRunDirWarnsInsteadOfPrintingZeros(t *testing.T) {
	t.Parallel()
	p := latencyPlane(t)
	h := newWaveHarness()

	rc := h.run("latency", "--cycles", "40-50", "--json", "--project-root", p.root)

	got := decodeLatency(t, h, rc)
	if got.Cycles != 0 || len(got.Warnings) == 0 || !strings.Contains(strings.Join(got.Warnings, "|"), "no run dir") {
		t.Errorf("report = %+v, want 0 cycles and a no-run-dir warning", got)
	}
}

func TestWaveLatency_TextNamesTheSlotUseThePhasesAndTheFamilies(t *testing.T) {
	t.Parallel()
	p := latencyPlane(t)
	h := newWaveHarness()

	rc := h.run("latency", "--wave", "1", "--project-root", p.root)

	if rc != 0 {
		t.Fatalf("rc = %d; stderr:\n%s", rc, h.stderr.String())
	}
	out := h.stdout.String()
	for _, want := range []string{
		"wave 1: cycles 1-3, 3 cycle(s), lane time 2h45m0s\n",
		"slots: avg live 2.00 of 4.00, 3.00 of 6.00 slot-hours used (50.0%) over 1h30m0s\n",
		"build ",
		" 66.7% ",
		"(gap) ",
		"lane time to a PASS, last 20 builds of each family:\n",
		"claude  builds 3  passes 2  lane 2h5m0s  per pass 1h2m30s\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
}

func TestWaveLatency_TextSaysSoWhenNoPassExistsForAFamilyAndWhenThereIsNoSlotSample(t *testing.T) {
	t.Parallel()
	p := latencyPlane(t)
	p.write(".evolve/runs/cycle-8/signals.ndjson", `{"ts":"2026-10-19T14:00:00Z","phase":"build","kind":"phase.dispatched"}`+"\n"+`{"ts":"2026-10-19T14:10:00Z","phase":"build","kind":"phase.outcome","fields":{"verdict":"FAIL"}}`+"\n")
	p.write(".evolve/runs/cycle-8/llm-calls.ndjson", `{"phase":"build","cli":"agy-claude-tmux","started_at":"2026-10-19T14:00:00Z","ended_at":"2026-10-19T14:05:00Z"}`+"\n")
	h := newWaveHarness()

	rc := h.run("latency", "--cycles", "8-8", "--project-root", p.root)

	out := h.stdout.String()
	if rc != 0 || !strings.Contains(out, "slots: no orchestration sample lies in the window\n") || !strings.Contains(out, "agy-claude  builds 1  passes 0  lane 10m0s  per pass none\n") {
		t.Errorf("rc = %d; stdout:\n%s", rc, out)
	}
}

func TestWaveLatency_WritesNothing(t *testing.T) {
	t.Parallel()
	p := latencyPlane(t)
	before := treeBytes(t, p.root)
	h := newWaveHarness()

	for _, args := range [][]string{
		{"latency", "--wave", "1"}, {"latency", "--json"}, {"latency", "--cycles", "1-3", "--json"},
	} {
		if rc := h.run(append(args, "--project-root", p.root)...); rc != 0 {
			t.Fatalf("%v: rc = %d; stderr:\n%s", args, rc, h.stderr.String())
		}
	}

	if after := treeBytes(t, p.root); !reflect.DeepEqual(before, after) {
		t.Errorf("the verb changed the plane: it must read run dirs and signals and write no record, no signal and no lock\nbefore %v\nafter  %v", before, after)
	}
}

func TestWaveLatency_RefusesWhatItCannotRead(t *testing.T) {
	t.Parallel()
	p := latencyPlane(t)
	empty := newWavePlane(t)
	for name, args := range map[string][]string{
		"unknown flag":       {"latency", "--nope", "--project-root", p.root},
		"stray operand":      {"latency", "extra", "--project-root", p.root},
		"wave and cycles":    {"latency", "--wave", "1", "--cycles", "1-2", "--project-root", p.root},
		"wave not a number":  {"latency", "--wave", "x", "--project-root", p.root},
		"wave zero":          {"latency", "--wave", "0", "--project-root", p.root},
		"cycles not a range": {"latency", "--cycles", "5", "--project-root", p.root},
		"cycles reversed":    {"latency", "--cycles", "5-2", "--project-root", p.root},
		"cycles from zero":   {"latency", "--cycles", "0-2", "--project-root", p.root},
		"unknown wave":       {"latency", "--wave", "9", "--project-root", p.root},
		"no wave recorded":   {"latency", "--project-root", empty.root},
	} {
		h := newWaveHarness()
		if rc := h.run(args...); rc != exitUsage || !strings.Contains(h.stderr.String(), wavePrefix) {
			t.Errorf("%s: rc = %d stderr %q, want exitUsage with the wave prefix", name, rc, h.stderr.String())
		}
	}
}

func TestWaveLatency_ReportsAPlaneThatCannotBeResolvedOrRead(t *testing.T) {
	t.Parallel()
	p := latencyPlane(t)
	h := newWaveHarness()
	h.rootErr = io.ErrUnexpectedEOF
	if rc := h.run("latency", "--project-root", p.root); rc != exitIO {
		t.Errorf("unresolvable root: rc = %d, want exitIO", rc)
	}

	broken := latencyPlane(t)
	broken.write(".evolve/waves/wave-3.json", "{not json")
	h = newWaveHarness()
	if rc := h.run("latency", "--project-root", broken.root); rc != exitIO {
		t.Errorf("unreadable wave records: rc = %d, want exitIO", rc)
	}

	noRuns := newWavePlane(t)
	h = newWaveHarness()
	if rc := h.run("latency", "--cycles", "1-2", "--project-root", noRuns.root); rc != exitIO {
		t.Errorf("no runs dir: rc = %d, want exitIO", rc)
	}

	noSignals := latencyPlane(t)
	if err := os.Remove(filepath.Join(noSignals.evolveDir(), "signals.ndjson")); err != nil {
		t.Fatal(err)
	}
	h = newWaveHarness()
	if rc := h.run("latency", "--wave", "1", "--project-root", noSignals.root); rc != exitIO {
		t.Errorf("no orchestration signals: rc = %d, want exitIO", rc)
	}
}

func TestWaveLatency_WarnsAboutMalformedOrchestrationLines(t *testing.T) {
	t.Parallel()
	p := latencyPlane(t)
	body, err := os.ReadFile(filepath.Join(p.evolveDir(), "signals.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	p.write(".evolve/signals.ndjson", string(body)+"{broken\n")
	h := newWaveHarness()

	rc := h.run("latency", "--wave", "1", "--json", "--project-root", p.root)

	got := decodeLatency(t, h, rc)
	if want := []string{"1 run dir(s) had no signals.ndjson", "1 malformed orchestration line(s) were skipped"}; !reflect.DeepEqual(got.Warnings, want) {
		t.Errorf("warnings = %v, want %v", got.Warnings, want)
	}
}

func TestWave_ASubVerbIsRequiredAndLatencyIsListedInTheUsage(t *testing.T) {
	t.Parallel()
	h := newWaveHarness()
	if rc := h.run(); rc != exitUsage || !strings.Contains(h.stderr.String(), "a sub-verb is required") || !strings.Contains(h.stderr.String(), "evolve wave latency [--wave N | --cycles A-B]") {
		t.Errorf("rc = %d stderr %q, want exitUsage naming the missing sub-verb and the latency usage", rc, h.stderr.String())
	}
}

func TestWaveLatency_AWaveWithAPIDCountsOnlyItsOwnLoopFromItsFirstSample(t *testing.T) {
	t.Parallel()
	p := latencyPlane(t)
	signals := `{"pid":77,"ts":"2026-10-19T09:59:00Z","code":"LOOP_ORCH_POOL_IDLE","fields":{"live":"2","target":"4"}}` + "\n" +
		`{"pid":88,"ts":"2026-10-19T10:30:00Z","code":"LOOP_ORCH_POOL_IDLE","fields":{"live":"4","target":"4"}}` + "\n" +
		`{"pid":77,"ts":"2026-10-19T10:59:00Z","code":"LOOP_ORCH_POOL_IDLE","fields":{"live":"0","target":"4"}}` + "\n"
	p.write(".evolve/signals.ndjson", signals)
	end := time.Date(2026, 10, 19, 11, 30, 0, 0, time.UTC)
	rec := wave.Record{Number: 1, RunID: "r1", CycleFloor: 0, PID: 77, StartedAt: time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC), EndedAt: &end}
	if err := p.store().Save(rec); err != nil {
		t.Fatal(err)
	}
	h := newWaveHarness()

	rc := h.run("latency", "--wave", "1", "--json", "--project-root", p.root)

	got := decodeLatency(t, h, rc)
	if got.Slots.Window != 91*time.Minute || !near(got.Slots.UsedSlotHours, 2.0) || !near(got.Slots.CapacitySlotHours, 4.0*91.0/60.0) {
		t.Errorf("slots = %+v, want pid 77 only: live 2 from 09:59 to 10:59 then 0 to 11:30", got.Slots)
	}
}

func TestWaveLatency_ANextRecordWithoutACycleFloorLeavesTheCeilingOpen(t *testing.T) {
	t.Parallel()
	recs := []wave.Record{{Number: 1, CycleFloor: 4}, {Number: 2, CycleFloor: 0}}
	s, err := latencyRequest{wave: 1}.scope(recs)
	if err != nil || s.ceiling != 0 {
		t.Errorf("scope = %+v err = %v, want an open ceiling", s, err)
	}
}
