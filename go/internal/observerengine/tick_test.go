package observerengine

// tick_test.go — §6 tests 20-23, 27 and the critic's folds B5/G6: the tick
// order under a stepped clock, the threshold table, the nudge failure path,
// the heartbeat guard and the structural "no ticker in the leaf" proof.

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// scriptedPolicy returns a fixed verdict and records the events it saw.
type scriptedPolicy struct {
	action recovery.StallAction
	reason string
	seen   []recovery.StallEvent
}

func (p *scriptedPolicy) Decide(ev recovery.StallEvent) (recovery.StallAction, string) {
	p.seen = append(p.seen, ev)
	return p.action, p.reason
}

// TestTick_ProbeRunsBeforeIngest — the ProcessAlive closure observes
// eventCount == 0 while three lines are pending; after the tick they are
// ingested; the incident fires exactly once over three ticks. Kills M1 (in
// the leaf) and M5 (processDeadFired dropped).
func TestTick_ProbeRunsBeforeIngest(t *testing.T) {
	t.Parallel()
	var seenAtProbe []int
	var e *Engine
	e, _, ws := newEngine(t, func(s *Settings, d *Deps) {
		s.PGID = 4242
		s.Enforce = true
		d.ProcessAlive = func(int) bool { seenAtProbe = append(seenAtProbe, e.eventCount); return false }
	})
	seedLog(t, filepath.Join(ws, "builder-stdout.log"), threeLines...)
	for i := 0; i < 3; i++ {
		e.Tick()
	}
	if len(seenAtProbe) != 1 || seenAtProbe[0] != 0 {
		t.Errorf("the probe runs once, before ingest: %v", seenAtProbe)
	}
	if e.eventCount != 3 {
		t.Errorf("eventCount = %d after the tick", e.eventCount)
	}
	events := readEvents(t, e.s.Paths.Events)
	if n := strings.Count(strings.Join(typesOf(events), ","), "process_dead"); n != 1 {
		t.Errorf("process_dead fires exactly once: %v", typesOf(events))
	}
}

// TestTick_OrderIsProbeIngestRulesHeartbeatEof — on ONE tick that trips every
// rule the events file reads process_dead → soft_stall_nudge → stuck_no_output
// → stuck_no_progress → heartbeat, and StopEOFGrace comes only on a later
// quiet tick with eventCount > 0. Kills every pairwise swap (M1-M3, M10) and
// M11.
func TestTick_OrderIsProbeIngestRulesHeartbeatEof(t *testing.T) {
	t.Parallel()
	calls := 0
	e, _, ws := newEngine(t, func(s *Settings, d *Deps) {
		s.PGID, s.NudgeS, s.StallS, s.MaxNoProgressS, s.HeartbeatEvery, s.EOFGraceS = 99, 300, 600, 600, 1, 1
		d.ProcessAlive = func(int) bool { return false }
		d.Now = func() time.Time { // construction, start, 3 lines at t0; every read from the rules on at +700 s
			calls++
			if calls <= 6 {
				return fixtureAt
			}
			return fixtureAt.Add(700 * time.Second)
		}
	})
	e.Start()
	seedLog(t, filepath.Join(ws, "builder-stdout.log"), threeLines...)
	if got := e.Tick(); got != StopNone {
		t.Fatalf("a tick that ingested lines is not EOF: %v", got)
	}
	want := []string{"observer_started", "process_dead", "soft_stall_nudge", "stuck_no_output", "stuck_no_progress", "heartbeat"}
	if got := typesOf(readEvents(t, e.s.Paths.Events)); !reflect.DeepEqual(got, want) {
		t.Fatalf("tick order:\n got %v\nwant %v", got, want)
	}
	if got := e.Tick(); got != StopEOFGrace {
		t.Errorf("a quiet tick after events reaches the EOF grace: %v", got)
	}
}

// TestTick_EOFGraceNeedsAnEvent — an empty log never reaches StopEOFGrace
// (eventCount stays 0) however many quiet ticks pass. Kills M11.
func TestTick_EOFGraceNeedsAnEvent(t *testing.T) {
	t.Parallel()
	e, _, _ := newEngine(t, func(s *Settings, _ *Deps) { s.EOFGraceS = 1 })
	for i := 0; i < 5; i++ {
		if got := e.Tick(); got != StopNone {
			t.Fatalf("tick %d: %v without any event", i, got)
		}
	}
	if e.eofQuietCount != 5 {
		t.Errorf("quiet ticks counted: %d", e.eofQuietCount)
	}
}

// TestTick_FrozenClockNeverBlocks — 1,000 ticks under a frozen clock complete
// well within the bound with no ticker; the counters match the seeded lines.
// Kills M31 (a ticker or a Sleep moved into the engine) together with
// TestEngine_HasNoTickerSleepOrTimer below, which is the structural proof.
func TestTick_FrozenClockNeverBlocks(t *testing.T) {
	t.Parallel()
	e, rc, ws := newEngine(t, func(s *Settings, _ *Deps) { s.HeartbeatEvery = 100 })
	seedLog(t, filepath.Join(ws, "builder-stdout.log"), threeLines...)
	start := time.Now()
	for i := 0; i < 1000; i++ {
		e.Tick()
	}
	if el := time.Since(start); el > 5*time.Second {
		t.Fatalf("1,000 ticks took %v", el)
	}
	if e.eventCount != 3 || e.toolCallCount != 1 || e.toolResultCnt != 1 || e.pollCounter != 1000 {
		t.Errorf("counters: events=%d tc=%d tr=%d polls=%d", e.eventCount, e.toolCallCount, e.toolResultCnt, e.pollCounter)
	}
	if n := strings.Count(strings.Join(typesOf(readEvents(t, e.s.Paths.Events)), ","), "heartbeat"); n != 10 {
		t.Errorf("heartbeat every 100 ticks → 10, got %d", n)
	}
	if len(rc.all()) != 0 {
		t.Errorf("the happy path emits nothing into the Center: %+v", rc.all())
	}
}

// TestEngine_HasNoTickerSleepOrTimer — go/ast scan of the leaf's production
// sources: the engine never references time.NewTicker, time.NewTimer,
// time.Sleep, time.After or time.Tick — the host's select loop owns time.
func TestEngine_HasNoTickerSleepOrTimer(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parser.ParseFile(fset, name, src, 0); err != nil {
			t.Fatal(err)
		}
		for _, banned := range []string{"time.NewTicker", "time.NewTimer", "time.Sleep", "time.After", "time.Tick"} {
			if strings.Contains(string(src), banned) {
				t.Errorf("%s references %s: the leaf is clock-stepped, the host owns the ticker", name, banned)
			}
		}
	}
}

// TestIdleRules_Table — the thresholds (>=), the int truncation of idle
// seconds, the nudge-once guard, NudgeS 0 = off, the MaxNoProgressS gate, and
// the typed StallEvent the policy receives. Kills M32 (>= → >), M33
// (truncation → rounding), M4, M34 (the MaxNoProgressS gate), M35 (a wrong
// Kind string).
func TestIdleRules_Table(t *testing.T) {
	t.Parallel()
	rows := []struct {
		name                     string
		idle                     float64
		nudgeS, stallS, maxNoPro int
		nudged                   bool
		wantNudge, wantStall     bool
		wantNoProgress           bool
	}{
		{"below nudge", 299.9, 300, 600, 0, false, false, false, false},
		{"at nudge", 300, 300, 600, 0, false, true, false, false},
		{"above nudge", 301, 300, 600, 0, false, true, false, false},
		{"truncation keeps 300.9 under a 301 threshold", 300.9, 301, 600, 0, false, false, false, false},
		{"below stall", 599.9, 300, 600, 0, false, true, false, false},
		{"at stall", 600, 300, 600, 0, false, true, true, false},
		{"already nudged", 700, 300, 600, 0, true, false, true, false},
		{"nudge off", 700, 0, 600, 0, false, false, true, false},
		{"no-progress off", 700, 0, 9999, 0, false, false, false, false},
		{"no-progress on", 700, 0, 9999, 600, false, false, false, true},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			pol := &scriptedPolicy{action: recovery.StallExtend, reason: "r"}
			e, _, _ := newEngine(t, func(s *Settings, d *Deps) {
				s.NudgeS, s.StallS, s.MaxNoProgressS = row.nudgeS, row.stallS, row.maxNoPro
				d.Policy = pol
				d.Now = fixedClock(fixtureAt.Add(time.Duration(row.idle * float64(time.Second))))
			})
			e.lastEventTS, e.lastProgressTS = fixtureAt, fixtureAt // construction read the shifted clock; pin the baselines
			e.nudged = row.nudged
			e.toolCallCount, e.toolResultCnt = 2, 1
			e.runStallRules()
			types := strings.Join(typesOf(readEvents(t, e.s.Paths.Events)), ",")
			if got := strings.Contains(types, "soft_stall_nudge"); got != row.wantNudge {
				t.Errorf("nudge fired=%v want %v (%s)", got, row.wantNudge, types)
			}
			if got := strings.Contains(types, "stuck_no_output"); got != row.wantStall {
				t.Errorf("stall fired=%v want %v (%s)", got, row.wantStall, types)
			}
			if got := strings.Contains(types, "stuck_no_progress"); got != row.wantNoProgress {
				t.Errorf("no-progress fired=%v want %v (%s)", got, row.wantNoProgress, types)
			}
			for _, ev := range pol.seen {
				want := recovery.StallEvent{Kind: "stuck_no_output", Phase: "build", IdleS: int(row.idle), ThresholdS: row.stallS}
				if ev.Kind == "stuck_no_progress" {
					want = recovery.StallEvent{Kind: "stuck_no_progress", Phase: "build", IdleS: int(row.idle), ThresholdS: row.maxNoPro, ToolCalls: 2, ToolResults: 1}
				}
				if ev != want {
					t.Errorf("policy saw %+v, want %+v", ev, want)
				}
			}
		})
	}
}

// TestIdleRules_NudgeAppendFailureReportsAndStillEmits — the intent of
// coverage_test.go:310-359 (which stays in the host): a failing Nudge port
// reports OBSERVER_NUDGE_APPEND_FAILED with idle_s/threshold_s/agent, the
// soft_stall_nudge envelope still emits, nudged is set, and a second tick
// nudges no more. Kills M40 (nudged unset on failure), M41 (envelope skipped),
// M42 (a second Nudge call).
func TestIdleRules_NudgeAppendFailureReportsAndStillEmits(t *testing.T) {
	t.Parallel()
	nudges := 0
	e, rc, _ := newEngine(t, func(s *Settings, d *Deps) {
		s.NudgeS = 300
		d.Nudge = func(body string) error {
			nudges++
			if body != s.NudgeBody {
				t.Errorf("the configured body is handed to the port: %q", body)
			}
			return errors.New("inbox: open: is a directory")
		}
		d.Now = fixedClock(fixtureAt.Add(400 * time.Second))
	})
	e.lastEventTS = fixtureAt
	e.Tick()
	e.Tick()
	got := rc.byCode(CodeNudgeAppendFailed)
	if len(got) != 1 || got[0].Reason != "inbox: open: is a directory" || got[0].Origin != "Engine.Tick" {
		t.Fatalf("one nudge fault: %+v", got)
	}
	if f := got[0].Fields; f["step"] != "nudge" || f["idle_s"] != "400" || f["threshold_s"] != "300" || f["agent"] != "builder" {
		t.Errorf("fields: %v", f)
	}
	if !e.nudged || nudges != 1 {
		t.Errorf("nudged=%v nudges=%d — once, even on failure", e.nudged, nudges)
	}
	if n := strings.Count(strings.Join(typesOf(readEvents(t, e.s.Paths.Events)), ","), "soft_stall_nudge"); n != 1 {
		t.Errorf("the WARN envelope still emits, once: %d", n)
	}
}

// TestHeartbeat_GuardsAZeroInterval — HeartbeatEvery 0 (an undefaulted
// Settings) emits no heartbeat and never divides by zero (critic B5); 1 emits
// every tick.
func TestHeartbeat_GuardsAZeroInterval(t *testing.T) {
	t.Parallel()
	e, _, _ := newEngine(t, func(s *Settings, _ *Deps) { s.HeartbeatEvery = 0 })
	e.Tick()
	if n := len(readEvents(t, e.s.Paths.Events)); n != 0 {
		t.Errorf("no heartbeat with a zero interval, got %d events", n)
	}
	e2, _, _ := newEngine(t, func(s *Settings, _ *Deps) { s.HeartbeatEvery = 1 })
	e2.Tick()
	e2.Tick()
	if got := typesOf(readEvents(t, e2.s.Paths.Events)); !reflect.DeepEqual(got, []string{"heartbeat", "heartbeat"}) {
		t.Errorf("heartbeat every tick: %v", got)
	}
}

// TestShutdown_EmitsTheReason — the three reasons the host passes.
func TestShutdown_EmitsTheReason(t *testing.T) {
	t.Parallel()
	e, _, _ := newEngine(t, nil)
	for _, r := range []string{"sigusr1", "stop-timer", "eof_grace"} {
		e.Shutdown(r)
	}
	events := readEvents(t, e.s.Paths.Events)
	if len(events) != 3 {
		t.Fatalf("%d events", len(events))
	}
	for i, r := range []string{"sigusr1", "stop-timer", "eof_grace"} {
		data, _ := events[i]["data"].(map[string]any)
		if events[i]["type"] != "observer_shutdown" || events[i]["severity"] != "INFO" || data["reason"] != r {
			t.Errorf("event %d: %v", i, events[i])
		}
	}
}

// TestRunStallRules_UnknownScopeRunsNothing — the leaf's twin of the host's
// characterization: a scope that is neither phase nor cycle runs no rule and
// reads no clock; both known scopes run them. Kills M12 in the leaf.
func TestRunStallRules_UnknownScopeRunsNothing(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{"batch", "", ScopePhase, ScopeCycle} {
		reads := 0
		e, _, _ := newEngine(t, func(s *Settings, d *Deps) {
			s.Scope, s.Enforce, s.PGID = scope, true, 4242
			d.Now = func() time.Time { reads++; return fixtureAt.Add(1000 * time.Second) }
		})
		e.lastEventTS = fixtureAt
		reads = 0
		e.runStallRules()
		fired := strings.Contains(strings.Join(typesOf(readEvents(t, e.s.Paths.Events)), ","), "stuck_no_output")
		if known := scope == ScopePhase || scope == ScopeCycle; fired != known || (reads == 2) != known { // the rule read + the INCIDENT emit read
			t.Errorf("scope %q: known=%v fired=%v reads=%d", scope, known, fired, reads)
		}
	}
}
