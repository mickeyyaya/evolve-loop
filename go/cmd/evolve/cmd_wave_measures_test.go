package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
	"github.com/mickeyyaya/evolve-loop/go/internal/wavemeasure"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func waveSignalLine(at time.Time, kind, code, fields string) string {
	return fmt.Sprintf(`{"schema_version":"1.0","seq":1,"pid":1,"ts":%q,"module":"loop","origin":"x","kind":%q,"code":%q,"severity":"INFO","reason":"r","fields":{%s}}`+"\n",
		at.Format(time.RFC3339Nano), kind, code, fields)
}

func (p wavePlane) seedMeasuredRuns() {
	p.t.Helper()
	ago := func(m int) time.Time { return waveTestClock.Add(-time.Duration(m) * time.Minute) }
	for i, n := range []int{79, 80, 81} {
		ended := ago(600 - 60*i)
		if err := p.store().Save(wave.Record{Number: n, RunID: fmt.Sprintf("r%d", n), CycleFloor: 1820 + i, StartedAt: ago(660 - 60*i), EndedAt: &ended}); err != nil {
			p.t.Fatal(err)
		}
	}
	if err := p.store().Save(wave.Record{Number: 82, RunID: "r82", CycleFloor: 1834, StartedAt: ago(120)}); err != nil {
		p.t.Fatal(err)
	}
	p.write(".evolve/runs/cycle-1835/signals.ndjson",
		waveSignalLine(ago(120), "ledger.appended", "", "")+
			waveSignalLine(ago(70), "ship.error", "SHIP_GIT_FLEET_REBASE_NEEDED", "")+
			waveSignalLine(ago(60), "cycle.sealed", "", `"final_verdict":"PASS"`))
	p.write(".evolve/landing/cycle-1835.json", `{"cycle":1835,"status":"complete"}`)
	p.write(".evolve/runs/cycle-1836/signals.ndjson",
		waveSignalLine(ago(120), "ledger.appended", "", "")+
			waveSignalLine(ago(30), "cycle.sealed", "", `"final_verdict":"PASS"`))
	p.write(".evolve/landing/cycle-1836.json", `{"cycle":1836,"status":"complete"}`)
	p.write(".evolve/signals.ndjson",
		waveSignalLine(ago(119), "loop.wave", "", `"width_sized":"3","desired":"4"`)+
			waveSignalLine(ago(90), "loop.wave", wavemeasure.CodeAdmitWait, ""))
}

func TestWaveStatus_ReportsTheConflictRate(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedMeasuredRuns()
	h := newWaveHarness()

	rc := h.run("status", "--json", "--project-root", p.root)

	var got waveStatusReport
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil || rc != 0 {
		t.Fatalf("rc = %d, decode %v:\n%s", rc, err, h.stdout.String())
	}
	want := wavemeasure.Measures{WidthSized: 3, WidthRealized: 1, Utilization: 150.0 / 360.0, ConflictRate: 0.5, Conflicts: 1, Landed: 2, AdmissionWaits: 1}
	if withoutCost(got.Measures) != want {
		t.Errorf("measures = %+v\nwant %+v", got.Measures, want)
	}
	if len(got.Runs) != waveMeasuredRuns || got.Runs[0].Wave != 80 || got.Runs[2].Wave != 82 || withoutCost(got.Runs[2].Measures) != want ||
		got.Runs[2].WidthSource != wave.WidthFromSignal || got.Runs[0].WidthSource != wave.WidthFromPolicy || got.Runs[0].RunID != "r80" {
		t.Errorf("runs = %+v, want the last %d runs, oldest first, the newest equal to the current measures", got.Runs, waveMeasuredRuns)
	}
}

func TestWaveStatus_TextPrintsOneMeasuresLine(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedMeasuredRuns()
	h := newWaveHarness()

	rc := h.run("status", "--project-root", p.root)

	if want := "measures: width 1/3 · utilization 0.42 · conflicts 1/2\n"; rc != 0 || !strings.Contains(h.stdout.String(), want) {
		t.Errorf("rc = %d; stdout lacks %q:\n%s", rc, want, h.stdout.String())
	}
}

func TestWaveStatus_NoRecordGivesZeroMeasuresAndNoRuns(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	h := newWaveHarness()

	rc := h.run("status", "--json", "--project-root", p.root)

	var got waveStatusReport
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil || rc != 0 {
		t.Fatalf("rc = %d, decode %v:\n%s", rc, err, h.stdout.String())
	}
	if got.Measures != (wavemeasure.Measures{}) || got.Runs == nil || len(got.Runs) != 0 {
		t.Errorf("measures %+v runs %v, want zero measures and an empty run list", got.Measures, got.Runs)
	}
}

func TestWaveStatus_AMalformedProjectSignalIsAWarning(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	if err := p.store().Save(wave.Record{Number: 1, RunID: "r1", StartedAt: waveTestClock.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	p.write(".evolve/signals.ndjson", "not json\n")
	h := newWaveHarness()

	h.run("status", "--json", "--project-root", p.root)

	var got waveStatusReport
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "signals.ndjson") {
		t.Errorf("warnings = %q, want one that names the project signal stream", got.Warnings)
	}
}

func TestDispatchFleetIteration_RecordsTheSizedWidth(t *testing.T) {
	t.Parallel()
	root := gittest.Fixture(t).Dir
	writeWave20Shape(t, root)
	evolveDir := filepath.Join(root, ".evolve")
	var console bytes.Buffer
	b := &loopBatchCoordinator{ctx: context.Background(), cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, result: &loopResult{}, stdout: io.Discard, stderr: &console, loadGraph: noModuleGraph}
	b.deps.Signals = newRootSignalCenter(root, evolveDir, &console)
	b.deps.Storage = &fixtures.FakeStorage{State: core.State{LastCycleNumber: 7}}
	fleetCfg := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Scheduling: "wave"}
	var starvation fleet.StarvationTracker

	b.dispatchFleetIteration(3, fleetCfg, "/nonexistent/evolve", &starvation)
	b.deps.Signals.Flush()

	chunk, err := signalcenter.ReadStream(filepath.Join(evolveDir, signalcenter.StreamFileName), 0)
	if err != nil {
		t.Fatal(err)
	}
	var sized []signalcenter.Event
	for _, e := range chunk.Events {
		if e.Kind == signalcenter.KindLoopWave && e.Fields[wavemeasure.FieldWidthSized] != "" {
			sized = append(sized, e)
		}
	}
	if len(sized) != 1 || sized[0].Fields[wavemeasure.FieldWidthSized] != "2" || sized[0].Fields["desired"] != "2" || sized[0].Fields["wave"] != "3" ||
		sized[0].Severity != signalcenter.SeverityInfo || sized[0].Reason != "wave 3 sized to 2 of 2 lane(s)" {
		t.Errorf("sizing records = %+v, want one INFO loop.wave record of width 2 for wave 3", sized)
	}
}

func TestWaveStatus_AFinishedRunEndsAtItsLoopLogAndALiveRunAtTheClock(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		alive bool
		want  wavemeasure.Measures
	}{
		{name: "finished", want: wavemeasure.Measures{WidthSized: 3, WidthRealized: 2, Utilization: 150.0 / 270.0, ConflictRate: 0.5, Conflicts: 1, Landed: 2, AdmissionWaits: 1}},
		{name: "live", alive: true, want: wavemeasure.Measures{WidthSized: 3, WidthRealized: 1, Utilization: 150.0 / 360.0, ConflictRate: 0.5, Conflicts: 1, Landed: 2, AdmissionWaits: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := newWavePlane(t)
			p.seedMeasuredRuns()
			logPath := p.loopLog("r82", 4343)
			ended := waveTestClock.Add(-30 * time.Minute)
			if err := os.Chtimes(logPath, ended, ended); err != nil {
				t.Fatal(err)
			}
			if err := p.store().Save(wave.Record{Number: 82, RunID: "r82", CycleFloor: 1834, PID: 4343, LogPath: logPath, StartedAt: waveTestClock.Add(-2 * time.Hour)}); err != nil {
				t.Fatal(err)
			}
			h := newWaveHarness()
			h.alive[4343] = tc.alive

			h.run("status", "--json", "--project-root", p.root)

			var got waveStatusReport
			if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			tc.want.Cost = wavemeasure.Cost{Started: 2, Shipped: 2, LLMMinutesPerShippedCycle: f64(0), FreshTokensPerShippedCycle: f64(0), CacheReadTokensPerShippedCycle: f64(0), AuditRoundsPerCycle: f64(0), ConvergenceRate: f64(1)}
			if !reflect.DeepEqual(got.Measures, tc.want) {
				t.Errorf("measures = %+v\nwant %+v", got.Measures, tc.want)
			}
		})
	}
}

func TestWaveLogEnded_IsTheLogMtimeOrNothing(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	logPath := p.loopLog("r1", 1)
	ended := waveTestClock.Add(-time.Hour)
	if err := os.Chtimes(logPath, ended, ended); err != nil {
		t.Fatal(err)
	}

	got, ok := waveLogEnded(wave.Record{LogPath: logPath})
	_, noPath := waveLogEnded(wave.Record{})
	_, missing := waveLogEnded(wave.Record{LogPath: filepath.Join(p.root, "absent.log")})

	if !ok || !got.Equal(ended) || noPath || missing {
		t.Errorf("waveLogEnded = %v %t, no path %t, missing log %t; want the mtime only for a present log", got, ok, noPath, missing)
	}
}

func TestCloseLastWave_WithNoLoopLogEndsAtTheClock(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	plan := &wavePlan{plane: wavePlaneDirs{root: p.root, evolveDir: p.evolveDir(), store: p.store()}, last: &wave.Record{Number: 3, LogPath: filepath.Join(p.root, "absent.log")}}

	if err := closeLastWave(newWaveHarness().env(p.root), plan); err != nil {
		t.Fatal(err)
	}

	records, err := p.store().Records()
	if err != nil || len(records) != 1 || records[0].EndedAt == nil || !records[0].EndedAt.Equal(waveTestClock) {
		t.Errorf("records = %+v, %v; want wave 3 closed at the clock", records, err)
	}
}

func (p wavePlane) seedSupplyRecords() {
	p.t.Helper()
	ago := func(m int) time.Time { return waveTestClock.Add(-time.Duration(m) * time.Minute) }
	path := waveSupplyPath(p.evolveDir())
	for _, s := range []orchestrator.Supply{
		{At: ago(570), Counts: map[string]int{orchestrator.ClassAdmissible: 1}},
		{At: ago(500), Counts: map[string]int{orchestrator.ClassAdmissible: 2}},
		{At: ago(110), Counts: map[string]int{orchestrator.ClassAdmissible: 5}},
		{At: ago(100), Counts: map[string]int{orchestrator.ClassAdmissible: 3, orchestrator.ClassWaitingDeps: 4, orchestrator.ClassConsoleProtected: 40}},
	} {
		if err := orchestrator.AppendSupply(path, s); err != nil {
			p.t.Fatal(err)
		}
	}
}

func TestWaveStatus_ReportsTheSupplyOfTheLastCensusOfEachRun(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedMeasuredRuns()
	p.seedSupplyRecords()
	h := newWaveHarness()

	rc := h.run("status", "--json", "--project-root", p.root)

	var got waveStatusReport
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil || rc != 0 {
		t.Fatalf("rc = %d, decode %v:\n%s", rc, err, h.stdout.String())
	}
	current := &wavemeasure.Supply{Admissible: 3, WaitingDeps: 4, ConsoleProtected: 40}
	if len(got.Runs) != waveMeasuredRuns || !reflect.DeepEqual(got.Runs[0].Measures.Supply, &wavemeasure.Supply{Admissible: 1}) ||
		!reflect.DeepEqual(got.Runs[1].Measures.Supply, &wavemeasure.Supply{Admissible: 2}) || !reflect.DeepEqual(got.Runs[2].Measures.Supply, current) {
		t.Errorf("run supply = %+v %+v %+v, want the last census inside the window of each run, oldest first", got.Runs[0].Measures.Supply, got.Runs[1].Measures.Supply, got.Runs[2].Measures.Supply)
	}
	if !reflect.DeepEqual(got.Measures.Supply, current) {
		t.Errorf("current supply = %+v, want the last census of the current run %+v", got.Measures.Supply, current)
	}
}

func TestWaveStatus_TheTextLineNamesTheSupplyOfTheCurrentRunAndARunWithNoCensusHasNone(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedMeasuredRuns()
	p.seedSupplyRecords()
	h := newWaveHarness()

	rc := h.run("status", "--project-root", p.root)

	if want := "measures: width 1/3 · utilization 0.42 · conflicts 1/2 · supply 3 admissible, 4 waiting, 40 console\n"; rc != 0 || !strings.Contains(h.stdout.String(), want) {
		t.Errorf("rc = %d; stdout lacks %q:\n%s", rc, want, h.stdout.String())
	}
	bare := newWavePlane(t)
	bare.seedMeasuredRuns()
	var report waveStatusReport
	if h.run("status", "--json", "--project-root", bare.root); json.Unmarshal(h.stdout.Bytes(), &report) != nil || report.Measures.Supply != nil || report.Runs[0].Measures.Supply != nil {
		t.Errorf("a plane with no census gave supply %+v, want none", report.Measures.Supply)
	}
}

func TestWaveStatus_ABrokenSupplyRecordIsAWarningAndTheMeasuresStayWhole(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedMeasuredRuns()
	p.write(".evolve/waves/supply.json", "{not json")
	h := newWaveHarness()

	rc := h.run("status", "--json", "--project-root", p.root)

	var got waveStatusReport
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil || rc != 0 {
		t.Fatalf("rc = %d, decode %v:\n%s", rc, err, h.stdout.String())
	}
	if got.Measures.Supply != nil || got.Measures.Landed != 2 {
		t.Errorf("measures = %+v, want the other measures whole and no supply", got.Measures)
	}
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "supply record") {
		t.Errorf("warnings = %q, want one that names the supply record", got.Warnings)
	}
}

func TestWaveStatus_TextPrintsTheOptimisticFailureCountWhenTheWaveHadOptimisticAdmissions(t *testing.T) {
	t.Parallel()
	var with, without strings.Builder
	writeWaveMeasuresText(&with, wavemeasure.Measures{WidthSized: 3, OptimisticAdmissions: 4, OptimisticFailures: 1, OptimisticFailureRate: 0.25})
	writeWaveMeasuresText(&without, wavemeasure.Measures{WidthSized: 3})

	if want := " · optimistic 1/4 failed (0.25)\n"; !strings.Contains(with.String(), want) {
		t.Errorf("text = %q, want suffix %q", with.String(), want)
	}
	if strings.Contains(without.String(), "optimistic") {
		t.Errorf("text = %q, want no optimistic part without an optimistic admission", without.String())
	}
}
