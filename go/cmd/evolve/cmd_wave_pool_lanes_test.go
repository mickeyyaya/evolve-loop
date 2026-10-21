package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

type wave130Cycle struct {
	id       int
	todo     string
	start    string
	sealedAt string
	verdict  string
}

var wave130Cycles = []wave130Cycle{
	{1952, "handoff-h1-types-ledger", "2026-10-18T23:54:49Z", "2026-10-19T00:28:00Z", "PASS"},
	{1953, "inbox-item-writer-keeps-the-filed-format-lane", "2026-10-18T23:54:49Z", "2026-10-19T01:29:21Z", "FAIL"},
	{1954, "handoff-h6-policy-block", "2026-10-18T23:54:49Z", "2026-10-19T00:53:01Z", "PASS"},
	{1955, "landing-s1-readonly-redesign-lane", "2026-10-18T23:54:49Z", "", ""},
	{1956, "history-index-merge-conflict-lane", "2026-10-19T00:28:03Z", "2026-10-19T01:30:22Z", "PASS"},
	{1957, "cli-loop-zero-ship-breaker-lane", "2026-10-19T00:53:05Z", "", ""},
	{1958, "history-refusal-names-the-evolve-comments-command-lane", "2026-10-19T01:29:23Z", "2026-10-19T02:17:45Z", "FAIL"},
	{1959, "practice-claude-code-changelog-c6b0a656-202610182352", "2026-10-19T01:30:25Z", "2026-10-19T01:56:09Z", "PASS"},
	{1960, "landing-s1-readonly-redesign-lane", "2026-10-19T02:02:24Z", "2026-10-19T02:28:55Z", "PASS"},
	{1961, "cli-loop-zero-ship-breaker-lane", "2026-10-19T03:00:39Z", "2026-10-19T04:16:36Z", "PASS"},
	{1962, "cli-floor-run-lane", "2026-10-19T04:16:38Z", "2026-10-19T04:55:32Z", "PASS"},
	{1963, "cli-rebuild-lane", "2026-10-19T04:55:34Z", "", ""},
}

func (p wavePlane) seedWave130() {
	p.t.Helper()
	pool, err := os.ReadFile("testdata/wave-130-signals.ndjson")
	if err != nil {
		p.t.Fatal(err)
	}
	p.write(".evolve/signals.ndjson", string(pool))
	p.write(".evolve/policy.json", `{"fleet":{"count":4}}`)
	started, _ := time.Parse(time.RFC3339, "2026-10-18T23:54:49Z")
	if err := p.store().Save(wave.Record{Number: 130, RunID: "20261018T235040Z", CycleFloor: 1951, PID: 20898, StartedAt: started}); err != nil {
		p.t.Fatal(err)
	}
	for _, c := range wave130Cycles {
		dir := fmt.Sprintf(".evolve/runs/cycle-%d/", c.id)
		p.write(dir+"lane-scope.json", fmt.Sprintf(`{"todo_ids":[%q]}`, c.todo))
		phase := "build"
		lines := fmt.Sprintf(`{"schema_version":"1.0","seq":1,"pid":1,"ts":%q,"module":"loop","origin":"x","kind":"ledger.appended","severity":"INFO","reason":"r","fields":{}}`+"\n", c.start)
		if c.sealedAt != "" {
			phase = "ship"
			lines += fmt.Sprintf(`{"schema_version":"1.0","seq":2,"pid":1,"ts":%q,"module":"loop","origin":"x","kind":"cycle.sealed","severity":"INFO","reason":"r","fields":{"final_verdict":%q}}`+"\n", c.sealedAt, c.verdict)
		}
		p.write(dir+"run.json", fmt.Sprintf(`{"cycle_id":%d,"phase":%q}`, c.id, phase))
		p.write(dir+"signals.ndjson", lines)
	}
}

func TestWaveStatus_Wave130CountsTheLiveLanesFromThePool(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedWave130()
	h := newWaveHarness()
	env := h.env(p.root)
	env.now = func() time.Time { return time.Date(2026, 10, 19, 5, 28, 49, 0, time.UTC) }

	rc := runWaveWith(env, []string{"status", "--project-root", p.root}, &h.stdout, &h.stderr)

	out := h.stdout.String()
	for _, want := range []string{
		"measures: width 1/4 · utilization 0.56 ",
		"cycle 1955: stopped: lane deadline (phase build)",
		"cycle 1957: stopped: lane deadline (phase build)",
		"cycle 1963: phase build",
	} {
		if rc != 0 || !strings.Contains(out, want) {
			t.Errorf("rc = %d; stdout lacks %q:\n%s", rc, want, out)
		}
	}
	if strings.Contains(out, "pool:") {
		t.Errorf("a wave whose loop is gone printed a live pool count:\n%s", out)
	}
}

func TestWaveStatus_Wave137CountsTheLanesThatLaunchedBeforeTheRecord(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	pool, err := os.ReadFile("testdata/wave-137-signals.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	earlier := `{"schema_version":"signal/1.0","seq":9,"pid":91234,"ts":"2026-10-20T06:41:10Z","module":"loop","origin":"Pool.Run","kind":"loop.wave","code":"LOOP_ORCH_LANE_LAUNCHED","severity":"INFO","reason":"r","fields":{"live":"4","target":"4","todo":"a-lane-of-the-stopped-loop"}}` + "\n"
	p.write(".evolve/signals.ndjson", earlier+string(pool))
	p.write(".evolve/policy.json", `{"fleet":{"count":4}}`)
	started := time.Date(2026, 10, 20, 6, 41, 11, 721016000, time.UTC)
	if err := p.store().Save(wave.Record{Number: 137, RunID: "20261020T063647Z", CycleFloor: 2008, PID: 38124, LogPath: p.loopLog("20261020T063647Z", 38124), StartedAt: started}); err != nil {
		t.Fatal(err)
	}
	h := newWaveHarness()
	h.alive[38124] = true
	env := h.env(p.root)
	env.now = func() time.Time { return time.Date(2026, 10, 20, 8, 15, 0, 0, time.UTC) }

	rc := runWaveWith(env, []string{"status", "--project-root", p.root}, &h.stdout, &h.stderr)

	out := h.stdout.String()
	for _, want := range []string{"measures: width 2/4 · utilization 0.48 ", "pool:    live 1\n"} {
		if rc != 0 || !strings.Contains(out, want) {
			t.Errorf("rc = %d; stdout lacks %q:\n%s", rc, want, out)
		}
	}
}

func TestWaveWatchAndNext_Wave130ApplyTheDeadlineStopsLikeWaveStatus(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedWave130()
	h := newWaveHarness()

	watchRC := h.run("watch", "--project-root", p.root)
	watch := h.stdout.String()
	nextRC := h.run("next", "--dry-run", "--json", "--project-root", p.root)
	goal := decodeWaveEnvelope(t, h.stdout.Bytes()).Goal

	for _, id := range []int{1955, 1957} {
		if want := fmt.Sprintf("wave 130: cycle %d: stopped: lane deadline\n", id); watchRC != 0 || !strings.Contains(watch, want) {
			t.Errorf("watch rc = %d; stdout lacks %q:\n%s", watchRC, want, watch)
		}
		if want := fmt.Sprintf("- Cycle %d: no final verdict. Termination: stopped: lane deadline.", id); nextRC != 0 || !strings.Contains(goal, want) {
			t.Errorf("next rc = %d; goal lacks %q:\n%s", nextRC, want, goal)
		}
	}
	if strings.Contains(watch, "cycle 1963: stopped") || !strings.Contains(goal, "- Cycle 1963: no final verdict. Last phase: build.") {
		t.Errorf("cycle 1963 had no deadline stop; watch:\n%s\ngoal:\n%s", watch, goal)
	}
}

func TestWaveStatus_Wave130TheLoopSealOfADeadlineStopHoldsOverTheSignal(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedWave130()
	seal := `{"schema_version":"1.0","seq":3,"pid":20898,"ts":"2026-10-19T02:02:31Z","module":"loop","origin":"Pool.seal","kind":"cycle.sealed","severity":"INFO","reason":"r","fields":{"termination_reason":"stopped: lane deadline","todo":"landing-s1-readonly-redesign-lane"}}` + "\n"
	body, err := os.ReadFile(p.root + "/.evolve/runs/cycle-1955/signals.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	p.write(".evolve/runs/cycle-1955/signals.ndjson", string(body)+seal)
	h := newWaveHarness()

	rc := h.run("status", "--json", "--project-root", p.root)

	var report waveStatusReport
	if err := json.Unmarshal(h.stdout.Bytes(), &report); err != nil || rc != 0 {
		t.Fatalf("rc = %d, decode %v:\n%s", rc, err, h.stdout.String())
	}
	stopped := map[int]string{}
	for _, c := range report.Cycles {
		stopped[c.ID] = c.Termination
	}
	if stopped[1955] != "stopped: lane deadline" || stopped[1957] != "stopped: lane deadline" || stopped[1963] != "" {
		t.Errorf("terminations = %v, want 1955 by its loop seal and 1957 by the deadline signal", stopped)
	}
}
