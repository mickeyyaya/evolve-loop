package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/pipelinerates"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func seedHealthPlane(t *testing.T) wavePlane {
	t.Helper()
	p := newWavePlane(t)
	at := func(m int) time.Time { return waveTestClock.Add(-time.Duration(m) * time.Minute) }
	ended := at(10)
	if err := p.store().Save(wave.Record{Number: 7, RunID: "r7", PID: 77, CycleFloor: 1, StartedAt: at(120), EndedAt: &ended}); err != nil {
		t.Fatal(err)
	}
	p.write(".evolve/runs/cycle-1/run.json", `{"shipped":true}`)
	p.write(".evolve/runs/cycle-1/signals.ndjson", waveSignalLine(at(30), "ship.landed", "", ""))
	p.write(".evolve/runs/cycle-2/run.json", `{"shipped":false}`)
	p.write(".evolve/runs/cycle-2/llm-calls.ndjson", `{"phase":"build","cli":"claude","started_at":"2026-10-08T10:00:00Z","ended_at":"2026-10-08T10:30:00Z"}`+"\n")
	p.write(".evolve/runs/cycle-2/signals.ndjson", `{"ts":"2026-10-08T10:00:00Z","phase":"build","kind":"phase.dispatched"}`+"\n"+`{"ts":"2026-10-08T11:00:00Z","phase":"build","kind":"phase.outcome","fields":{"verdict":"PASS"}}`+"\n"+`{"schema_version":"signal/1.0","seq":1,"ts":"2026-10-08T10:30:00Z","module":"ladder","kind":"ladder.decision","code":"LADDER_DECISION","severity":"INFO","origin":"t","reason":"r","fields":{"class":"spec"}}`+"\n")
	p.write(".evolve/signals.ndjson", fmt.Sprintf(`{"ts":%q,"code":"LOOP_ORCH_POOL_FILLED","pid":77,"fields":{"live":"2","target":"2"}}`+"\n"+`{"ts":%q,"code":"LOOP_ORCH_POOL_UNDERFILLED","pid":77,"fields":{"live":"1","target":"2","cause":"run_cap"}}`+"\n",
		at(120).Format(time.RFC3339), at(60).Format(time.RFC3339)))
	return p
}

var healthPQ = regexp.MustCompile(`PQ6 \S+/day.*PQ9 \S+`)

func TestWaveStatus_AndQualityReportShowTheSameThroughput(t *testing.T) {
	t.Parallel()
	p := seedHealthPlane(t)
	view := func(root string) (pipelinerates.WaveView, error) {
		return pipelinerates.ReadWaveView(root, waveTestClock)
	}

	h := newWaveHarness()
	h.run("status", "--project-root", p.root)
	status := healthPQ.FindString(h.stdout.String())
	var report bytes.Buffer
	env := qualityEnv{scopes: map[string][]qualityReader{"wave": nil}, view: view, newCenter: publishingQualityCenter}
	if rc := runQualityWith(env, []string{"report", "--scope", "wave", "--project-root", p.root}, &report, io.Discard); rc != cliExitOK {
		t.Fatalf("rc = %d", rc)
	}
	quality := healthPQ.FindString(report.String())

	const want = "PQ6 0.143/day · top idle cause run_cap · top fail class spec · PQ7 0.773 · PQ8 1.000 · PQ9 0.500"
	if status != want || quality != want {
		t.Errorf("status %q, report %q; want %q", status, quality, want)
	}
}

func TestWaveStatus_JSONCarriesTheHealthAndAMissingPlaneAddsNothing(t *testing.T) {
	t.Parallel()
	p := seedHealthPlane(t)
	h := newWaveHarness()
	h.run("status", "--json", "--project-root", p.root)
	var got waveStatusReport
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil || got.Health == nil || got.Health.Wave != 7 {
		t.Fatalf("health = %+v, %v", got.Health, err)
	}

	bare := newWavePlane(t)
	if err := bare.store().Save(wave.Record{Number: 1, RunID: "r1", StartedAt: waveTestClock}); err != nil {
		t.Fatal(err)
	}
	h = newWaveHarness()
	h.run("status", "--project-root", bare.root)
	if out := h.stdout.String(); strings.Contains(out, "health:") || strings.Contains(out, "WARN:    health") {
		t.Errorf("status:\n%s\nwant no health line and no warning when the records are absent", out)
	}
}

func TestWaveStatus_AnUnreadableCycleIsAHealthWarning(t *testing.T) {
	t.Parallel()
	p := seedHealthPlane(t)
	p.write(".evolve/runs/cycle-2/run.json", "{")
	h := newWaveHarness()
	h.run("status", "--project-root", p.root)
	if out := h.stdout.String(); !strings.Contains(out, "WARN:    wave health:") {
		t.Errorf("status:\n%s\nwant a wave health warning", out)
	}
}

func TestQualityReport_JSONCarriesTheWaveViewAndASilentViewAddsNothing(t *testing.T) {
	t.Parallel()
	p := seedHealthPlane(t)
	view := func(root string) (pipelinerates.WaveView, error) {
		return pipelinerates.ReadWaveView(root, waveTestClock)
	}
	var out bytes.Buffer
	env := qualityEnv{scopes: map[string][]qualityReader{"wave": nil}, view: view, newCenter: publishingQualityCenter}
	if rc := runQualityWith(env, []string{"report", "--scope", "wave", "--json", "--project-root", p.root}, &out, io.Discard); rc != cliExitOK {
		t.Fatalf("rc = %d", rc)
	}
	var got qualityReportResult
	if err := json.Unmarshal(out.Bytes(), &got); err != nil || got.WaveView == nil || got.WaveView.TopIdleCause != "run_cap" {
		t.Errorf("result = %+v, %v; want the wave view", got, err)
	}

	out.Reset()
	failing := func(string) (pipelinerates.WaveView, error) { return pipelinerates.WaveView{}, io.EOF }
	env = qualityEnv{scopes: map[string][]qualityReader{"wave": nil}, view: failing, newCenter: func(string, string, io.Writer) (*signalcenter.Center, func()) {
		return publishingQualityCenter(p.root, p.root+"/.evolve", io.Discard)
	}}
	if rc := runQualityWith(env, []string{"report", "--scope", "wave", "--project-root", p.root}, &out, io.Discard); rc != cliExitOK || strings.Contains(out.String(), "health:") {
		t.Errorf("rc = %d out = %q, want no health line when the view fails", rc, out.String())
	}
}

func TestQualityReport_TreeScopePrintsNoWaveHealthLine(t *testing.T) {
	t.Parallel()
	p := seedHealthPlane(t)
	view := func(root string) (pipelinerates.WaveView, error) {
		return pipelinerates.ReadWaveView(root, waveTestClock)
	}
	var out bytes.Buffer
	env := qualityEnv{scopes: map[string][]qualityReader{"tree": nil}, view: view, newCenter: publishingQualityCenter}

	rc := runQualityWith(env, []string{"report", "--scope", "tree", "--project-root", p.root}, &out, io.Discard)

	if rc != cliExitOK || strings.Contains(out.String(), "health:") {
		t.Errorf("rc = %d out = %q, want no health line on the tree scope", rc, out.String())
	}
}

func TestLiveWaveView_ReadsTheSharedReaderAndNamesAnAbsentWaveStore(t *testing.T) {
	t.Parallel()
	if _, err := liveWaveView(t.TempDir()); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("liveWaveView on a root with no waves: err = %v, want fs.ErrNotExist from the shared reader", err)
	}
}
