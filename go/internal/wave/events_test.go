package wave_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	building := wave.Cycle{ID: 7, Phase: "build"}
	cases := []struct {
		name      string
		prev, cur wave.Snapshot
		want      []wave.Event
	}{
		{"no change is no event",
			wave.Snapshot{LoopLive: true, Cycles: []wave.Cycle{building}},
			wave.Snapshot{LoopLive: true, Cycles: []wave.Cycle{building}}, nil},
		{"a new cycle shows its phase",
			wave.Snapshot{LoopLive: true},
			wave.Snapshot{LoopLive: true, Cycles: []wave.Cycle{{ID: 7, Phase: "scout"}}},
			[]wave.Event{{Kind: wave.EventPhase, Cycle: 7, Detail: "scout"}}},
		{"a phase change",
			wave.Snapshot{LoopLive: true, Cycles: []wave.Cycle{building}},
			wave.Snapshot{LoopLive: true, Cycles: []wave.Cycle{{ID: 7, Phase: "audit"}}},
			[]wave.Event{{Kind: wave.EventPhase, Cycle: 7, Detail: "audit"}}},
		{"a quota pause, a seal and a ship in one poll come in that order",
			wave.Snapshot{LoopLive: true, Cycles: []wave.Cycle{{ID: 7, Phase: "ship", QuotaPauses: 1}}},
			wave.Snapshot{LoopLive: true, Cycles: []wave.Cycle{{ID: 7, Phase: "ship", QuotaPauses: 2, Verdict: "PASS", Shipped: true}}},
			[]wave.Event{
				{Kind: wave.EventQuotaPaused, Cycle: 7, Detail: "ship"},
				{Kind: wave.EventSealed, Cycle: 7, Detail: "PASS"},
				{Kind: wave.EventShipped, Cycle: 7},
			}},
		{"a login wall is its own event and never a quota pause",
			wave.Snapshot{LoopLive: true, Cycles: []wave.Cycle{{ID: 7, Phase: "audit"}}},
			wave.Snapshot{LoopLive: true, Cycles: []wave.Cycle{{ID: 7, Phase: "audit", LoginWalls: 1}}},
			[]wave.Event{{Kind: wave.EventLoginRequired, Cycle: 7, Detail: "audit"}}},
		{"a lane deadline stop with no verdict is a stop",
			wave.Snapshot{LoopLive: true, Cycles: []wave.Cycle{building}},
			wave.Snapshot{LoopLive: true, Cycles: []wave.Cycle{{ID: 7, Phase: "build", Termination: "stopped: lane deadline"}}},
			[]wave.Event{{Kind: wave.EventStopped, Cycle: 7, Detail: "stopped: lane deadline"}}},
		{"a termination beside a verdict is the seal only",
			wave.Snapshot{LoopLive: true, Cycles: []wave.Cycle{building}},
			wave.Snapshot{LoopLive: true, Cycles: []wave.Cycle{{ID: 7, Phase: "build", Verdict: "FAIL", Termination: "blocked-needs-console"}}},
			[]wave.Event{{Kind: wave.EventSealed, Cycle: 7, Detail: "FAIL"}}},
		{"the loop exit comes last",
			wave.Snapshot{LoopLive: true, Cycles: []wave.Cycle{{ID: 8, Phase: "audit"}}},
			wave.Snapshot{LoopLive: false, Cycles: []wave.Cycle{{ID: 8, Phase: "audit", Verdict: "FAIL"}}},
			[]wave.Event{{Kind: wave.EventSealed, Cycle: 8, Detail: "FAIL"}, {Kind: wave.EventLoopExit}}},
		{"cycles come in id order",
			wave.Snapshot{LoopLive: true},
			wave.Snapshot{LoopLive: true, Cycles: []wave.Cycle{{ID: 9, Phase: "tdd"}, {ID: 8, Phase: "build"}}},
			[]wave.Event{{Kind: wave.EventPhase, Cycle: 8, Detail: "build"}, {Kind: wave.EventPhase, Cycle: 9, Detail: "tdd"}}},
		{"a loop that was not live does not exit again",
			wave.Snapshot{LoopLive: false}, wave.Snapshot{LoopLive: false}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := wave.Diff(c.prev, c.cur)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Diff = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestEvent_String(t *testing.T) {
	t.Parallel()
	cases := []struct {
		event wave.Event
		want  string
	}{
		{wave.Event{Kind: wave.EventPhase, Cycle: 7, Detail: "build"}, "cycle 7: phase build"},
		{wave.Event{Kind: wave.EventSealed, Cycle: 7, Detail: "WARN"}, "cycle 7: sealed WARN"},
		{wave.Event{Kind: wave.EventShipped, Cycle: 7}, "cycle 7: shipped"},
		{wave.Event{Kind: wave.EventQuotaPaused, Cycle: 7, Detail: "scout"}, "cycle 7: quota-paused in scout"},
		{wave.Event{Kind: wave.EventLoginRequired, Cycle: 7, Detail: "audit"}, "cycle 7: login-required in audit"},
		{wave.Event{Kind: wave.EventLoopExit}, "loop: exit"},
		{wave.Event{Kind: wave.EventStopped, Cycle: 7, Detail: "stopped: lane deadline"}, "cycle 7: stopped: lane deadline"},
		{wave.Event{Kind: wave.EventWaveStarted, Detail: "83"}, "wave 83 started; watch follows it"},
		{wave.Event{Kind: wave.EventPool, Detail: "LOOP_ORCH_POOL_IDLE: pool idle"}, "LOOP_ORCH_POOL_IDLE: pool idle"},
		{wave.Event{Kind: wave.EventStall, Trigger: "lane-stalls", Detail: "3 line(s)", Lanes: []string{"lane-1", "lane-2"}, Evidence: []string{".evolve/signals.ndjson"}}, "stall lane-stalls: 3 line(s); lanes lane-1, lane-2; evidence .evolve/signals.ndjson"},
	}
	for _, c := range cases {
		if got := c.event.String(); got != c.want {
			t.Errorf("%+v.String() = %q, want %q", c.event, got, c.want)
		}
	}
}

func TestEventKind_IsTheWatchJSONVocabulary(t *testing.T) {
	t.Parallel()
	kinds := []wave.EventKind{wave.EventPhase, wave.EventSealed, wave.EventShipped, wave.EventQuotaPaused, wave.EventLoginRequired, wave.EventLoopExit, wave.EventWaveStarted, wave.EventPool, wave.EventStopped, wave.EventStall}
	want := []string{"phase", "sealed", "shipped", "quota-paused", "login-required", "loop-exit", "wave-started", "pool", "stopped", "stall"}
	for i, k := range kinds {
		if string(k) != want[i] {
			t.Errorf("kind %d = %q, want %q: a watch consumer reads this text", i, k, want[i])
		}
	}
}

func TestEvent_StallJSONRoundTrip(t *testing.T) {
	t.Parallel()
	in := wave.Event{
		Kind:     wave.EventStall,
		Detail:   "verify-single-flight-unavailable: 20 event(s) in 9 cycle(s); SHARED FAULT",
		Trigger:  "abnormal-kind",
		Lanes:    []string{"cycle-2581"},
		Evidence: []string{".evolve/runs/cycle-2581/abnormal-events.jsonl"},
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out wave.Event
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Errorf("round-trip = %+v, want %+v", out, in)
	}
}
