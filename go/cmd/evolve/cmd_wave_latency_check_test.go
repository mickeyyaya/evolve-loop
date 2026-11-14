package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

var checkClock = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

type checkWave struct {
	live int
	open bool
}

func checkPlane(t *testing.T, waves ...checkWave) wavePlane {
	t.Helper()
	p := newWavePlane(t)
	var stream strings.Builder
	for i, w := range waves {
		n := i + 1
		start := checkClock.Add(time.Duration(i) * 3 * time.Hour)
		rec := wave.Record{Number: n, RunID: fmt.Sprintf("r%d", n), CycleFloor: i, PID: 100 + n, StartedAt: start}
		if !w.open {
			end := start.Add(2 * time.Hour)
			rec.EndedAt = &end
		}
		if err := p.store().Save(rec); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&stream, `{"ts":%q,"pid":%d,"kind":"loop.wave","code":"LOOP_ORCH_POOL_IDLE","fields":{"live":"%d","target":"4"}}`+"\n", start.Format(time.RFC3339Nano), 100+n, w.live)
	}
	p.write(".evolve/signals.ndjson", stream.String()+"not json\n")
	p.write(".evolve/runs/cycle-2/signals.ndjson",
		fmt.Sprintf(`{"ts":%q,"kind":"phase.dispatched","phase":"build"}`+"\n", checkClock.Add(3*time.Hour).Format(time.RFC3339Nano))+
			fmt.Sprintf(`{"ts":%q,"kind":"phase.outcome","phase":"build","fields":{"verdict":"PASS"}}`+"\n", checkClock.Add(3*time.Hour+10*time.Minute).Format(time.RFC3339Nano)))
	return p
}

func decodeCheck(t *testing.T, h *waveHarness, rc int) waveLatencyCheck {
	t.Helper()
	var got waveLatencyCheck
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil || rc != 0 {
		t.Fatalf("rc = %d, decode %v:\nstdout %s\nstderr %s", rc, err, h.stdout.String(), h.stderr.String())
	}
	return got
}

func countExceeded(t *testing.T, p wavePlane) int {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(p.evolveDir(), signalcenter.StreamFileName))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(raw), string(signalcenter.CodeLatencyBudgetExceeded))
}

func inboxFiles(t *testing.T, p wavePlane) []string {
	t.Helper()
	entries, _ := os.ReadDir(filepath.Join(p.evolveDir(), "inbox"))
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	return names
}

func TestWaveLatencyCheck_TwoBreachesInARowFileOneItemAndEmitOneSignalAcrossReruns(t *testing.T) {
	t.Parallel()
	p := checkPlane(t, checkWave{live: 4}, checkWave{live: 4}, checkWave{live: 2})
	h := newWaveHarness()

	rc := h.run("latency", "--check", "--project-root", p.root)

	if rc != 0 {
		t.Fatalf("rc = %d, want 0 for a soft gate; stderr %s", rc, h.stderr.String())
	}
	text := h.stdout.String()
	for _, want := range []string{"wave 1: ok", "wave 2: breach", "gate share", "wave 3: breach", "live lanes", "LATENCY_BUDGET_EXCEEDED", "latency-wave-breach-from-wave-2", "warning: 1 malformed orchestration line(s) were skipped"} {
		if !strings.Contains(text, want) {
			t.Errorf("text report lacks %q:\n%s", want, text)
		}
	}
	if got := inboxFiles(t, p); len(got) != 1 || !strings.HasSuffix(got[0], "latency-wave-breach-from-wave-2.json") {
		t.Errorf("inbox = %v, want the one item of the run from wave 2", got)
	}
	if got := countExceeded(t, p); got != 1 {
		t.Errorf("signals = %d, want 1", got)
	}

	rc = h.run("latency", "--check", "--json", "--project-root", p.root)

	report := decodeCheck(t, h, rc)
	if !report.Exceeded || report.Item != "latency-wave-breach-from-wave-2" || len(report.Waves) != 3 {
		t.Errorf("report = %+v, want exceeded with the item of wave 2", report)
	}
	if got := inboxFiles(t, p); len(got) != 1 {
		t.Errorf("a rerun filed %v, want still one item", got)
	}
	if got := countExceeded(t, p); got != 1 {
		t.Errorf("a rerun emitted %d signals in all, want 1", got)
	}
}

func TestWaveLatencyCheck_AnOpenWaveIsLeftOutAndRaisesNoAlarm(t *testing.T) {
	t.Parallel()
	p := checkPlane(t, checkWave{live: 2}, checkWave{live: 2, open: true})
	h := newWaveHarness()

	rc := h.run("latency", "--check", "--json", "--project-root", p.root)

	report := decodeCheck(t, h, rc)
	if report.Exceeded || report.Waves[1].Status != "left_out" || report.Item != "" {
		t.Errorf("report = %+v, want wave 2 left out and no alarm", report)
	}
	if got := inboxFiles(t, p); len(got) != 0 {
		t.Errorf("inbox = %v, want none", got)
	}
}

func TestWaveLatencyCheck_ReadsAPlaneWithNoRunDirsAndNoStream(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	end := checkClock.Add(time.Hour)
	if err := p.store().Save(wave.Record{Number: 1, RunID: "r1", StartedAt: checkClock, EndedAt: &end}); err != nil {
		t.Fatal(err)
	}
	h := newWaveHarness()

	rc := h.run("latency", "--check", "--json", "--project-root", p.root)

	report := decodeCheck(t, h, rc)
	if report.Exceeded || len(report.Waves) != 1 || report.Waves[0].Status != "left_out" {
		t.Errorf("report = %+v, want one wave left out for want of data", report)
	}
}

func TestWaveLatencyCheck_ReportsWhatItCannotRead(t *testing.T) {
	t.Parallel()
	cases := map[string]func(p wavePlane){
		"wave records": func(p wavePlane) { p.write(".evolve/waves/wave-9.json", "{not json") },
		"run dirs":     func(p wavePlane) { p.write(".evolve/runs", "a file where the run dirs belong") },
		"signal stream": func(p wavePlane) {
			_ = os.Remove(filepath.Join(p.evolveDir(), "signals.ndjson"))
			_ = os.Mkdir(filepath.Join(p.evolveDir(), "signals.ndjson"), 0o755)
		},
		"inbox": func(p wavePlane) { p.write(".evolve/inbox", "a file where the inbox belongs") },
	}
	for name, spoil := range cases {
		p := checkPlane(t, checkWave{live: 2}, checkWave{live: 2})
		if name == "run dirs" {
			_ = os.RemoveAll(filepath.Join(p.evolveDir(), "runs"))
		}
		spoil(p)
		h := newWaveHarness()

		rc := h.run("latency", "--check", "--project-root", p.root)

		if rc != exitIO || !strings.Contains(h.stderr.String(), wavePrefix) {
			t.Errorf("%s: rc = %d stderr %q, want exitIO with the wave prefix", name, rc, h.stderr.String())
		}
	}
}
