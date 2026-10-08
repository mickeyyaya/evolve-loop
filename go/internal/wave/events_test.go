package wave_test

import (
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
		{wave.Event{Kind: wave.EventLoopExit}, "loop: exit"},
		{wave.Event{Kind: wave.EventWaveStarted, Detail: "83"}, "wave 83 started; watch follows it"},
	}
	for _, c := range cases {
		if got := c.event.String(); got != c.want {
			t.Errorf("%+v.String() = %q, want %q", c.event, got, c.want)
		}
	}
}

func TestEventKind_IsTheWatchJSONVocabulary(t *testing.T) {
	t.Parallel()
	kinds := []wave.EventKind{wave.EventPhase, wave.EventSealed, wave.EventShipped, wave.EventQuotaPaused, wave.EventLoopExit, wave.EventWaveStarted}
	want := []string{"phase", "sealed", "shipped", "quota-paused", "loop-exit", "wave-started"}
	for i, k := range kinds {
		if string(k) != want[i] {
			t.Errorf("kind %d = %q, want %q: a watch consumer reads this text", i, k, want[i])
		}
	}
}
