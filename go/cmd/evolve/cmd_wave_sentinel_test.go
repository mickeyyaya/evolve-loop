package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func sentinelRecord(free uint64, held bool) string {
	line, _ := json.Marshal(sentinelLine{TS: "2026-11-21T08:00:00Z", FreeBytes: free, Path: "/p", CacheBytes: 41e9, CacheRead: true, Servers: 3, Orphans: 4, CountsRead: true, Held: held, HoldStage: "enforce", ReleaseStage: "shadow"})
	return string(line) + "\n"
}

func TestWaveStatus_ShowsTheLastHostSentinelSampleAndTheLoad(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	seedPoolWave(p)
	p.write(".evolve/logs/host-sentinel.ndjson", sentinelRecord(9e9, false)+sentinelRecord(8e9, true))
	p.write(".evolve/logs/host-pressure.ndjson", `{"ts":"2026-11-21T08:00:30Z","load_per_core":1.25,"mem_available_pct":40,"cap":3}`+"\n")
	h := newWaveHarness()
	if rc := h.run("status", "--project-root", p.root); rc != 0 {
		t.Fatalf("rc = %d, stderr %s", rc, h.stderr.String())
	}
	out := h.stdout.String()
	for _, want := range []string{"host:    free 8.00 GB at /p, hold enforce (held=true), release shadow", "go cache 41.00 GB", "owner-dead tmux servers 3, orphan processes 4", "load:    1.25 per core"} {
		if !strings.Contains(out, want) {
			t.Errorf("status lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "free 9.00 GB") {
		t.Errorf("status shows an older sample:\n%s", out)
	}
	h = newWaveHarness()
	h.run("status", "--json", "--project-root", p.root)
	var got waveStatusReport
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil || got.Host == nil || got.Host.Sentinel == nil || got.Host.Sentinel.FreeBytes != 8e9 || got.Host.Load == nil || got.Host.Load.Cap != 3 {
		t.Fatalf("json host = %+v, %v", got.Host, err)
	}
}

func TestWaveStatus_ReadsTheSentinelFileFromTheLogDirOfTheWaveRecord(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	logDir := t.TempDir()
	if err := p.store().Save(wave.Record{Number: 130, RunID: "r130", PID: 9, StartedAt: waveTestClock, LogPath: filepath.Join(logDir, "run.log")}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "host-sentinel.ndjson"), []byte(sentinelRecord(7e9, false)), 0o644); err != nil {
		t.Fatal(err)
	}
	h := newWaveHarness()
	h.run("status", "--project-root", p.root)
	if !strings.Contains(h.stdout.String(), "free 7.00 GB") {
		t.Errorf("status did not read the sentinel file of the wave log dir:\n%s", h.stdout.String())
	}
}

func TestWaveStatus_AMissingOrBrokenSentinelFileAddsNoLineOrOneWarning(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	seedPoolWave(p)
	h := newWaveHarness()
	h.run("status", "--project-root", p.root)
	if strings.Contains(h.stdout.String(), "host:") || strings.Contains(h.stdout.String(), "WARN:    host") {
		t.Errorf("a missing file added output:\n%s", h.stdout.String())
	}
	p.write(".evolve/logs/host-sentinel.ndjson", "not json\n")
	p.write(".evolve/logs/host-pressure.ndjson", "\n")
	h = newWaveHarness()
	h.run("status", "--project-root", p.root)
	if !strings.Contains(h.stdout.String(), "WARN:    host sentinel sample:") || strings.Contains(h.stdout.String(), "host pressure sample") {
		t.Errorf("a broken sentinel file and an empty load file:\n%s", h.stdout.String())
	}
}

func TestLastLine_ReadsTheTailOfALargeFileAndRefusesNoneOfAnEmptyOne(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	big := filepath.Join(dir, "big")
	var sb strings.Builder
	for i := 0; i < 2000; i++ {
		fmt.Fprintf(&sb, "{\"n\":%d}\n", i)
	}
	if err := os.WriteFile(big, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	line, err := lastLine(big)
	if err != nil || string(line) != `{"n":1999}` {
		t.Fatalf("last line = %q, %v", line, err)
	}
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if line, err := lastLine(empty); err != nil || line != nil {
		t.Fatalf("empty file gave %q, %v", line, err)
	}
	if _, err := lastLine(dir); err == nil {
		t.Fatal("a directory gave no error")
	}
}

func TestWaveWatch_PrintsTheHoldLineAndTheStuckIncident(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	seedPoolWave(p)
	at := func(m int) time.Time { return waveTestClock.Add(time.Duration(m) * time.Minute) }
	hostSignal := func(seq int, when time.Time, severity, kind, code, reason string) string {
		return fmt.Sprintf(`{"schema_version":"1.0","seq":%d,"pid":1,"ts":%q,"module":"host","origin":"hostsentinel","kind":%q,"code":%q,"severity":%q,"reason":%q,"fields":{"free_bytes":"5"}}`+"\n",
			seq, when.Format(time.RFC3339Nano), kind, code, severity, reason)
	}
	p.write(".evolve/signals.ndjson",
		hostSignal(1, at(1), "WARN", "loop.wave", "HOST_LANE_HOLD", "free disk is under the floor, so no new lane starts")+
			poolSignal(2, at(2), "INFO", "LOOP_ORCH_POOL_FLOOR_HOLD", "pool floor_hold", `"held":"true","live":"2","target":"4"`)+
			hostSignal(3, at(3), "INCIDENT", "loop.wave", "HOST_LANE_HOLD_STUCK", "the hold on new lanes lasted past gc.sentinel.hold_max_s")+
			hostSignal(4, at(4), "WARN", "host.sentinel", "HOST_GOCACHE_OVER", "the go build cache is over its cap"))
	h := newWaveHarness()
	if rc := h.run("watch", "--project-root", p.root); rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	out := h.stdout.String()
	for _, want := range []string{"HOST_LANE_HOLD:", "LOOP_ORCH_POOL_FLOOR_HOLD", "HOST_LANE_HOLD_STUCK"} {
		if !strings.Contains(out, want) {
			t.Errorf("watch lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "HOST_GOCACHE_OVER") {
		t.Errorf("watch printed a host.sentinel record, which the loop.wave filter leaves out:\n%s", out)
	}
}

func TestWaveStatus_ABrokenLoadFileOrADirInPlaceOfTheSentinelFileWarns(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	seedPoolWave(p)
	p.write(".evolve/logs/host-pressure.ndjson", "not json\n")
	if err := os.MkdirAll(filepath.Join(p.root, ".evolve", "logs", "host-sentinel.ndjson"), 0o755); err != nil {
		t.Fatal(err)
	}
	h := newWaveHarness()
	h.run("status", "--project-root", p.root)
	out := h.stdout.String()
	for _, want := range []string{"WARN:    host pressure sample:", "WARN:    host sentinel sample:"} {
		if !strings.Contains(out, want) {
			t.Errorf("status lacks %q:\n%s", want, out)
		}
	}
}
