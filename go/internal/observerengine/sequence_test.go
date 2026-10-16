package observerengine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

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

func scenarioClock(t0 time.Time, calls *int) func() time.Time {
	return func() time.Time {
		*calls++
		switch {
		case *calls <= 9:
			return t0
		case *calls <= 14:
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

func sequenceOf(t *testing.T, e *Engine) string {
	t.Helper()
	var g sequenceGolden
	for _, env := range readEvents(t, e.s.Paths.Events) {
		data, _ := env["data"].(map[string]any)
		keys := make([]string, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		action, _ := data["action"].(string)
		g.Events = append(g.Events, sequenceEntry{Type: env["type"].(string), Severity: env["severity"].(string), TS: env["ts"].(string), Keys: keys, Action: action})
	}
	raw, err := os.ReadFile(e.s.Paths.Report)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &g.Report); err != nil {
		t.Fatal(err)
	}
	delete(g.Report, "incidents")
	delete(g.Report, "trace_id")
	b, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(b) + "\n"
}

func scenarioEngine(t *testing.T, calls *int) (*Engine, *recordingCenter) {
	t.Helper()
	var d *Deps
	e, rc, ws := newEngine(t, func(s *Settings, dd *Deps) {
		s.PGID, s.NudgeS, s.StallS, s.HeartbeatEvery, s.MaxNoProgressS = 99999, 300, 600, 1, 900
		dd.Policy = &scriptedPolicy{action: recovery.StallExtend, reason: "deep-thinking phase; extend"}
		dd.Now = scenarioClock(fixtureAt, calls)
		d = dd
	})
	d.Nudge = func(string) error { d.Now(); return nil }
	e.d.Nudge = d.Nudge
	seedLog(t, filepath.Join(ws, "builder-stdout.log"), scenarioLines...)
	return e, rc
}

func TestEngine_ReplaysTheIdleScenarioSequence(t *testing.T) {
	t.Parallel()
	calls := 0
	e, rc := scenarioEngine(t, &calls)
	e.Start()
	for i := 0; i < 3; i++ {
		if got := e.Tick(); got != StopNone {
			t.Fatalf("tick %d: %v", i, got)
		}
	}
	e.Shutdown("sigusr1")
	if err := e.WriteReport(); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "sequence.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := sequenceOf(t, e); got != string(want) {
		t.Fatalf("sequence:\n got: %s\nwant: %s", got, want)
	}
	if calls != 20 {
		t.Errorf("the host's capture made 20 clock reads (18 at the heartbeat of tick 3, then the shutdown and the report); the leaf made %d", calls)
	}
	if len(rc.all()) != 0 {
		t.Errorf("the happy path emits ZERO observer signals: %+v", rc.all())
	}
}

func TestEngine_ClockReadCountIsExact(t *testing.T) {
	t.Parallel()
	reads := 0
	e, _, ws := newEngine(t, func(s *Settings, d *Deps) {
		s.HeartbeatEvery, s.MaxNoProgressS = 1, 900
		d.Now = func() time.Time { reads++; return fixtureAt }
	})
	if reads != 1 {
		t.Fatalf("construction reads once: %d", reads)
	}
	e.Start()
	seedLog(t, filepath.Join(ws, "builder-stdout.log"), threeLines...)
	e.Tick()
	if reads != 8 {
		t.Errorf("after Start and one 3-line tick with heartbeat + no-progress: 8 reads, got %d", reads)
	}
	e.s.HeartbeatEvery, e.s.MaxNoProgressS = 12, 0
	e.Tick()
	if reads != 9 {
		t.Errorf("a quiet tick without heartbeat/no-progress reads once: %d", reads)
	}
	e.Shutdown("stop-timer")
	_ = e.WriteReport()
	if reads != 11 {
		t.Errorf("shutdown + report read once each: %d", reads)
	}
}

func TestEngine_StreamIsByteIdenticalApartFromTheDeclaredCodes(t *testing.T) {
	t.Parallel()
	e, rc, _ := newEngine(t, func(s *Settings, _ *Deps) { s.Paths.Stdout = filepath.Join(t.TempDir(), "file-not-dir") })
	if err := os.WriteFile(e.s.Paths.Stdout, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	e.s.Paths.Stdout = filepath.Join(e.s.Paths.Stdout, "stdout.log")
	if err := os.Mkdir(e.s.Paths.Report, 0o755); err != nil {
		t.Fatal(err)
	}
	e.Start()
	e.Tick()
	e.Tick()
	e.Shutdown("sigusr1")
	_ = e.WriteReport()
	got := rc.all()
	if len(got) != 2 || got[0].Code != CodeStdoutTailFailed || got[1].Code != CodeReportWriteFailed {
		t.Fatalf("exactly the two provoked codes, once each: %+v", got)
	}
	for _, ev := range got {
		if ev.Module != "observer" || ev.Kind != "observer.warning" || ev.Cycle != 7 || ev.Phase != "build" {
			t.Errorf("module/kind/cycle/phase: %+v", ev)
		}
	}
}
