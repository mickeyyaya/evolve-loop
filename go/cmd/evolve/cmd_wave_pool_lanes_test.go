package main

import (
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
	if err := p.store().Save(wave.Record{Number: 130, RunID: "20261018T235040Z", CycleFloor: 1951, StartedAt: started}); err != nil {
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
		"measures: width 1/4 ",
		"cycle 1955: stopped: lane deadline (phase build)",
		"cycle 1957: stopped: lane deadline (phase build)",
		"cycle 1963: phase build",
	} {
		if rc != 0 || !strings.Contains(out, want) {
			t.Errorf("rc = %d; stdout lacks %q:\n%s", rc, want, out)
		}
	}
}
