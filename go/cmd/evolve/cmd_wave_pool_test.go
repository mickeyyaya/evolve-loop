package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func poolSignal(seq int, at time.Time, severity, code, reason, fields string) string {
	return fmt.Sprintf(`{"schema_version":"1.0","seq":%d,"pid":1,"ts":%q,"module":"loop","origin":"Pool.Run","kind":"loop.wave","code":%q,"severity":%q,"reason":%q,"fields":{%s}}`+"\n",
		seq, at.Format(time.RFC3339Nano), code, severity, reason, fields)
}

func seedPoolWave(p wavePlane) {
	p.t.Helper()
	if err := p.store().Save(wave.Record{Number: 130, RunID: "r130", PID: 9, StartedAt: waveTestClock}); err != nil {
		p.t.Fatal(err)
	}
}

func TestWaveWatch_PrintsThePoolTransitionsAndEveryLoopWaveWarn(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	seedPoolWave(p)
	at := func(m int) time.Time { return waveTestClock.Add(time.Duration(m) * time.Minute) }
	p.write(".evolve/signals.ndjson",
		poolSignal(1, at(-5), "WARN", "LOOP_ORCH_POOL_UNDERFILLED", "before the wave", `"live":"1"`)+
			poolSignal(2, at(1), "INFO", "LOOP_ORCH_LANE_LAUNCHED", "pool launched todo-a", `"todo":"todo-a","live":"1","target":"4"`)+
			poolSignal(3, at(2), "INFO", "LOOP_ORCH_LANE_EXITED", "pool exited todo-a", `"todo":"todo-a","live":"0","target":"4"`)+
			poolSignal(4, at(3), "INFO", "LOOP_ORCH_LANE_BACKFILLED", "pool backfilled todo-b", `"todo":"todo-b","live":"1","target":"4"`)+
			poolSignal(5, at(4), "INFO", "LOOP_ORCH_POOL_IDLE", "pool idle", `"live":"1","target":"4"`)+
			poolSignal(6, at(5), "WARN", "LOOP_ORCH_POOL_UNDERFILLED", "pool underfilled: 1 of 4 lanes live, min_lanes 4", `"live":"1","target":"4","min_lanes":"4","ready":"0"`)+
			poolSignal(7, at(6), "WARN", "LOOP_ORCH_LANE_DEADLINE", "lane todo-b passed its deadline", `"todo":"todo-b"`)+
			poolSignal(8, at(7), "INFO", "LOOP_ORCH_LANE_STALLED", "lane todo-b is stalled", `"todo":"todo-b"`)+
			poolSignal(9, at(8), "INFO", "LOOP_ORCH_WINDOW", "window grew", `"old":"2","new":"3"`)+
			poolSignal(10, at(9), "INFO", "LOOP_ORCH_SUPPLY", "supply census", `"admissible":"2"`)+
			poolSignal(11, at(10), "WARN", "LOOP_ORCH_LANE_RECOVERY_FAILED", "release of todo-b failed", `"todo":"todo-b"`))
	h := newWaveHarness()

	rc := h.run("watch", "--project-root", p.root)
	lines := strings.Split(strings.TrimSpace(h.stdout.String()), "\n")

	wantCodes := []string{
		"LOOP_ORCH_LANE_LAUNCHED", "LOOP_ORCH_LANE_EXITED", "LOOP_ORCH_LANE_BACKFILLED", "LOOP_ORCH_POOL_IDLE",
		"LOOP_ORCH_POOL_UNDERFILLED", "LOOP_ORCH_LANE_DEADLINE", "LOOP_ORCH_LANE_STALLED", "LOOP_ORCH_LANE_RECOVERY_FAILED",
	}
	if rc != 0 || len(lines) < len(wantCodes) {
		t.Fatalf("rc = %d, stdout:\n%s\nwant one line for each of %v", rc, h.stdout.String(), wantCodes)
	}
	for i, code := range wantCodes {
		if !strings.HasPrefix(lines[i], "wave 130: ") || !strings.Contains(lines[i], code) {
			t.Errorf("line %d = %q, want a wave 130 line naming %s", i, lines[i], code)
		}
	}
	if !strings.Contains(lines[4], "min_lanes 4") {
		t.Errorf("underfilled line = %q, want its reason with the min_lanes", lines[4])
	}
	out := h.stdout.String()
	for _, absent := range []string{"before the wave", "LOOP_ORCH_WINDOW", "LOOP_ORCH_SUPPLY"} {
		if strings.Contains(out, absent) {
			t.Errorf("stdout names %q, want an event before the wave start and the INFO window and supply records left out:\n%s", absent, out)
		}
	}
}

func TestWaveWatch_APoolEventPrintsOnceAcrossPolls(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	if err := p.store().Save(wave.Record{Number: 130, RunID: "r130", PID: 777, LogPath: p.loopLog("r130", 777), StartedAt: waveTestClock}); err != nil {
		t.Fatal(err)
	}
	at := func(m int) time.Time { return waveTestClock.Add(time.Duration(m) * time.Minute) }
	first := poolSignal(1, at(1), "INFO", "LOOP_ORCH_LANE_LAUNCHED", "pool launched todo-a", `"todo":"todo-a"`)
	second := poolSignal(2, at(2), "WARN", "LOOP_ORCH_POOL_UNDERFILLED", "pool underfilled", `"live":"1"`)
	p.write(".evolve/signals.ndjson", first)
	h := newWaveHarness()
	h.alive[777] = true
	h.onSleep = func(n int) {
		switch n {
		case 1:
			p.write(".evolve/signals.ndjson", first+second)
		case 3:
			h.alive[777] = false
		}
	}

	rc := h.run("watch", "--project-root", p.root)
	out := h.stdout.String()

	if rc != 0 || strings.Count(out, "LOOP_ORCH_LANE_LAUNCHED") != 1 || strings.Count(out, "LOOP_ORCH_POOL_UNDERFILLED") != 1 {
		t.Errorf("rc = %d, stdout:\n%s\nwant each pool event printed once across %d polls", rc, out, h.sleeps)
	}
	if strings.Index(out, "LOOP_ORCH_LANE_LAUNCHED") > strings.Index(out, "LOOP_ORCH_POOL_UNDERFILLED") {
		t.Errorf("stdout:\n%s\nwant the events in signal order", out)
	}
}

func TestWaveWatch_JSONLinesCarryThePoolEventAsAPoolKind(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	seedPoolWave(p)
	p.write(".evolve/signals.ndjson",
		poolSignal(1, waveTestClock.Add(time.Minute), "WARN", "LOOP_ORCH_POOL_UNDERFILLED", "pool underfilled", `"live":"1"`))
	h := newWaveHarness()

	rc := h.run("watch", "--json", "--project-root", p.root)
	lines := strings.Split(strings.TrimSpace(h.stdout.String()), "\n")

	var e wave.Event
	if rc != 0 || len(lines) < 1 || json.Unmarshal([]byte(lines[0]), &e) != nil || string(e.Kind) != "pool" || !strings.Contains(e.Detail, "LOOP_ORCH_POOL_UNDERFILLED") {
		t.Errorf("rc = %d, stdout %q, want a first JSON line of kind pool whose detail names LOOP_ORCH_POOL_UNDERFILLED", rc, h.stdout.String())
	}
}

func TestWaveWatch_ASignalStreamThatCannotBeReadIsAWarnOnceAndTheWatchGoesOn(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	seedPoolWave(p)
	p.write(".evolve/signals.ndjson", "not json\n")
	h := newWaveHarness()

	rc := h.run("watch", "--project-root", p.root)

	if rc != 0 || strings.Count(h.stderr.String(), "skipped 1 malformed signal line") != 1 {
		t.Errorf("rc = %d, stderr %q, want exit 0 and one WARN for the malformed signal line", rc, h.stderr.String())
	}
}

func poolWatcher(p wavePlane, stdout io.Writer, stderr io.Writer) *waveWatcher {
	return &waveWatcher{
		env:    newWaveHarness().env(p.root),
		plane:  wavePlaneDirs{root: p.root, evolveDir: p.evolveDir(), store: p.store()},
		cur:    wave.Record{Number: 130, StartedAt: waveTestClock},
		prev:   wave.Snapshot{LoopLive: true},
		seen:   map[string]bool{},
		stdout: stdout,
		stderr: stderr,
	}
}

func TestPollSignals_AWriteFailureIsReturnedAndPollPassesItOn(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	seedPoolWave(p)
	p.write(".evolve/signals.ndjson",
		poolSignal(1, waveTestClock.Add(time.Minute), "WARN", "LOOP_ORCH_POOL_UNDERFILLED", "pool underfilled", `"live":"1"`))
	w := poolWatcher(p, &failingWriter{}, io.Discard)

	_, err := w.poll()

	if err == nil || !strings.Contains(err.Error(), "write") {
		t.Errorf("poll error = %v, want the write failure", err)
	}
}

func TestPollSignals_AStreamThatIsNotAFileIsAWarnOnceAndNoEvent(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	seedPoolWave(p)
	if err := os.MkdirAll(filepath.Join(p.evolveDir(), "signals.ndjson"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	w := poolWatcher(p, &stdout, &stderr)

	for i := 0; i < 2; i++ {
		if err := w.pollSignals(); err != nil {
			t.Fatalf("pollSignals error = %v, want the watch to go on", err)
		}
	}

	if stdout.Len() != 0 || strings.Count(stderr.String(), "WARN: ") != 1 {
		t.Errorf("stdout %q, stderr %q, want no event and one WARN", stdout.String(), stderr.String())
	}
}

func TestPollSignals_OnlyLoopWaveSignalsPrint(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	seedPoolWave(p)
	other := strings.Replace(
		poolSignal(1, waveTestClock.Add(time.Minute), "WARN", "LOOP_ORCH_POOL_UNDERFILLED", "other kind", `"live":"1"`),
		`"kind":"loop.wave"`, `"kind":"loop.other"`, 1)
	p.write(".evolve/signals.ndjson", other)
	var stdout bytes.Buffer
	w := poolWatcher(p, &stdout, io.Discard)

	if err := w.pollSignals(); err != nil || stdout.Len() != 0 {
		t.Errorf("err = %v, stdout %q, want no output for a signal of another kind", err, stdout.String())
	}
}

func TestPoolEventDetail(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		sig  signalcenter.Event
		want string
	}{
		{"with reason", signalcenter.Event{Code: "LOOP_ORCH_POOL_IDLE", Reason: "pool idle"}, "LOOP_ORCH_POOL_IDLE: pool idle"},
		{"without reason", signalcenter.Event{Code: "LOOP_ORCH_POOL_IDLE"}, "LOOP_ORCH_POOL_IDLE"},
	}
	for _, c := range cases {
		if got := poolEventDetail(c.sig); got != c.want {
			t.Errorf("%s: detail = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestPollSignals_NoSignalStreamYetIsNotAnError(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	seedPoolWave(p)
	var stdout, stderr bytes.Buffer
	w := poolWatcher(p, &stdout, &stderr)

	if err := w.pollSignals(); err != nil || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Errorf("err = %v, stdout %q, stderr %q, want silence while the stream does not exist", err, stdout.String(), stderr.String())
	}
}
