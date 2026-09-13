package observerengine

// sequence_test.go — §6 tests 31-32 and the critic's B4 fold: the G3 idle
// scenario replayed through Start/Tick×3/Shutdown/WriteReport with the SAME
// call-indexed clock the host captured it with (the Nudge fake consumes one
// clock read, mirroring inbox.Append's TS mint — the seventh clock site that
// stays host-side); the Center stream on the happy path is EMPTY; and the
// exact clock-read count is the real M47 killer.

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

// scenarioClock is the host's G3 clock: t0 for calls 1-9, +400 s for 10-14,
// +700 s from 15 on.
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

// scenarioEngine builds the G3 engine: PGID set, NudgeS 300, StallS 600,
// HeartbeatEvery 1, MaxNoProgressS 900, an extend policy, the scenario clock.
func scenarioEngine(t *testing.T, calls *int) (*Engine, *recordingCenter) {
	t.Helper()
	var d *Deps
	e, rc, ws := newEngine(t, func(s *Settings, dd *Deps) {
		s.PGID, s.NudgeS, s.StallS, s.HeartbeatEvery, s.MaxNoProgressS = 99999, 300, 600, 1, 900
		dd.Policy = &scriptedPolicy{action: recovery.StallExtend, reason: "deep-thinking phase; extend"}
		dd.Now = scenarioClock(fixtureAt, calls)
		d = dd
	})
	d.Nudge = func(string) error { d.Now(); return nil } // inbox.Append mints its TS from the injected clock — ONE read
	e.d.Nudge = d.Nudge
	seedLog(t, filepath.Join(ws, "builder-stdout.log"), scenarioLines...)
	return e, rc
}

// TestEngine_ReplaysTheIdleScenarioSequence — the sequence golden the host
// captured through Run, replayed call for call. Kills M47 when a drifted read
// crosses a threshold or shifts a ts; TestEngine_ClockReadCountIsExact kills
// the rest.
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

// TestEngine_ClockReadCountIsExact — a counting clock: 1 at construction, 1
// per emit, 1 per valid line, 1 per rules tick, 1 per no-progress tick (only
// when MaxNoProgressS > 0), 1 per report. The load-bearing contract for the
// host's count-stepping clocks (design §8). Kills M47 (an extra or missing
// Now read anywhere).
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
	e.Start() // 1 emit
	seedLog(t, filepath.Join(ws, "builder-stdout.log"), threeLines...)
	e.Tick() // 3 lines + rules + no-progress + heartbeat emit = 6
	if reads != 8 {
		t.Errorf("after Start and one 3-line tick with heartbeat + no-progress: 8 reads, got %d", reads)
	}
	e.s.HeartbeatEvery, e.s.MaxNoProgressS = 12, 0
	e.Tick() // rules only = 1
	if reads != 9 {
		t.Errorf("a quiet tick without heartbeat/no-progress reads once: %d", reads)
	}
	e.Shutdown("stop-timer") // 1
	_ = e.WriteReport()      // 1
	if reads != 11 {
		t.Errorf("shutdown + report read once each: %d", reads)
	}
}

// TestEngine_StreamIsByteIdenticalApartFromTheDeclaredCodes — beside the empty
// happy path (test 31), two fault fixtures emit exactly their code once, all
// under module observer / kind observer.warning. Kills M48 (a stray Emit).
func TestEngine_StreamIsByteIdenticalApartFromTheDeclaredCodes(t *testing.T) {
	t.Parallel()
	e, rc, _ := newEngine(t, func(s *Settings, _ *Deps) { s.Paths.Stdout = filepath.Join(t.TempDir(), "file-not-dir") })
	if err := os.WriteFile(e.s.Paths.Stdout, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	e.s.Paths.Stdout = filepath.Join(e.s.Paths.Stdout, "stdout.log") // ENOTDIR on stat
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
