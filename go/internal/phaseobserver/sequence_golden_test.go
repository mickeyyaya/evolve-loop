package phaseobserver

// sequence_golden_test.go — ADR-0103 unit 12 step 0, G3: the ordered event
// sequence of one idle scenario driven through Run with a count-stepping
// clock, captured on 8e8f080f. The clock is indexed by CALL: construction (1),
// observer_started (2), one tick that ingests four lines (3-6: one read per
// line; 7: the idle rule; 8: the no-progress rule; 9: the heartbeat), a tick
// at +400 s (10: idle → the nudge; 11: inbox.Append's own read — the SEVENTH
// clock site, host-side; 12: soft_stall_nudge; 13: no-progress; 14: heartbeat),
// a tick at +700 s (15: idle → stuck_no_output under an extend policy; 16: the
// INCIDENT emit; 17: no-progress; 18: heartbeat — the clock closes ShutdownSig
// here), the shutdown (19) and the report (20). One extra or missing clock
// read shifts the shutdown point and every `ts` after it — the clock-parity
// tripwire the leaf's replay (its test 31) must match call for call.
//
// Q12 note: under a frozen clock two emits in one tick share an `id` —
// eventCount is a line count, not an emit sequence; the golden documents it.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// goldenAt is the fixed instant every step-0 pin starts from.
var goldenAt = time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)

func readGolden(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("golden %s: %v (capture it on the base first)", name, err)
	}
	return string(b)
}

// sequenceEntry is the shape the golden records per envelope.
type sequenceEntry struct {
	Type     string   `json:"type"`
	Severity string   `json:"severity"`
	TS       string   `json:"ts"`
	Keys     []string `json:"keys"`
	Action   string   `json:"action,omitempty"`
}

type sequenceGolden struct {
	Events []sequenceEntry `json:"events"`
	Report map[string]any  `json:"report"`
}

// scenarioClock is the G3 clock: t0 for calls 1-9, +400 s for 10-14, +700 s
// from 15; onClose is invoked once at call closeAt (the host test closes the
// shutdown channel there; the leaf's replay passes nil).
func scenarioClock(t0 time.Time, closeAt int, onClose func()) func() time.Time {
	var mu sync.Mutex
	calls := 0
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls == closeAt && onClose != nil {
			onClose()
		}
		switch {
		case calls <= 9:
			return t0
		case calls <= 14:
			return t0.Add(400 * time.Second)
		default:
			return t0.Add(700 * time.Second)
		}
	}
}

var scenarioLines = []string{
	`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"path":"x"}}]}}`,
	`{"type":"user","message":{"content":[{"type":"tool_result","is_error":true}]}}`,
	`{"type":"result","total_cost_usd":0.5,"usage":{"cache_read_input_tokens":1024,"cache_creation_input_tokens":256}}`,
	`{"type":"rate_limit_event","reason":"quota"}`,
}

// sequenceOf projects the events file and the report into the golden shape.
func sequenceOf(t *testing.T, ws string) sequenceGolden {
	t.Helper()
	var g sequenceGolden
	for _, env := range eventsOf(t, ws) {
		data, _ := env["data"].(map[string]any)
		keys := make([]string, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		action, _ := data["action"].(string)
		g.Events = append(g.Events, sequenceEntry{
			Type: env["type"].(string), Severity: env["severity"].(string), TS: env["ts"].(string), Keys: keys, Action: action,
		})
	}
	raw, err := os.ReadFile(filepath.Join(ws, "builder-observer-report.json"))
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	var report map[string]any
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	delete(report, "incidents") // pinned by G2; here only the counters
	delete(report, "trace_id")
	g.Report = report
	return g
}

func marshalSequence(t *testing.T, g sequenceGolden) string {
	t.Helper()
	b, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(b) + "\n"
}

// TestGolden_IdleScenarioSequence — G3 through Run. Kills any dropped or
// reordered emit in the move and any clock-read drift that crosses a threshold
// or shifts a `ts`.
func TestGolden_IdleScenarioSequence(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	seedLog(t, filepath.Join(ws, "builder-stdout.log"), scenarioLines...)
	shutdown := make(chan struct{})
	var once sync.Once
	rc := Run(Config{
		Workspace: ws, SubagentPGID: 99999, Cycle: 7, Phase: "build", Agent: "builder",
		PollS: 1, StallS: 600, NudgeS: 300, EOFGraceS: 9999, HeartbeatEvery: 1, MaxNoProgressS: 900,
		StallPolicy: &scriptedStallPolicy{action: recovery.StallExtend, reason: "deep-thinking phase; extend"},
		Now:         scenarioClock(goldenAt, 18, func() { once.Do(func() { close(shutdown) }) }),
		ShutdownSig: shutdown,
		StopAfterMS: 2000, // ticks every 500 ms; the clock closes shutdown inside tick 3, well before the timer
	}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	if got, want := marshalSequence(t, sequenceOf(t, ws)), readGolden(t, "sequence.golden.json"); got != want {
		t.Fatalf("the idle-scenario sequence drifted from the golden:\n got: %s\nwant: %s", got, want)
	}
}
