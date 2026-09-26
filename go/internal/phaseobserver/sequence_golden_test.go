package phaseobserver

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

// scenarioClock returns t0 for calls 1-9, t0+400 s for 10-14 and t0+700 s after, calling onClose at call closeAt.
// The scenario makes exactly 20 reads; one more or fewer shifts the shutdown and every ts after it.
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
	delete(report, "incidents") // observerengine's report golden pins these; this one keeps the counters
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
