package publisher

import (
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/filter"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestPublisher_RoutesOneEventToEveryMatchingChannel(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t,
		lossless("loop", "module=loop"),
		lossless("errors", "severity>=WARN"),
		lossless("ship", "kind=ship.*"),
		bestEffort("signals", ""),
	)
	p := startPublisher(t, cfg, openLog)
	warn := loopEvent(1)
	warn.Severity = signalcenter.SeverityWarn

	p.Listen(warn)
	p.Close(time.Minute)

	for _, name := range []string{"loop", "errors", "signals"} {
		assertRecords(t, readChannel(t, cfg, name), []string{"seq:1"})
	}
	ship, _ := channel.New(cfg.Root, "ship", cfg.Log)
	if _, err := ship.Read(0); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ship Read = %v, want fs.ErrNotExist: the route kind=ship.* refuses a loop event", err)
	}
}

func TestPublisher_TheRouteSeesTheSourceOfTheRoot(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, lossless("mine", "source=loop"), lossless("theirs", "source=ship"))
	p := startPublisher(t, cfg, openLog)

	p.Listen(loopEvent(1))

	assertRecords(t, readChannel(t, cfg, "mine"), []string{"seq:1"})
	theirs, _ := channel.New(cfg.Root, "theirs", cfg.Log)
	if _, err := theirs.Read(0); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("theirs Read = %v, want fs.ErrNotExist: the role is loop, not ship", err)
	}
}

func TestPublisher_LosslessEventIsOnDiskWhenFlushReturns(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, lossless("loop", "module=loop"))
	p := startPublisher(t, cfg, openLog)
	center := signalcenter.New(signalcenter.WithPID(testPID))
	center.Subscribe(p.Listen)

	center.Emit(loopEvent(0))
	center.Flush()

	got := readChannel(t, cfg, "loop")
	assertRecords(t, got, []string{"seq:1"})
	if got[0].Source != "loop" || got[0].Signal.PID != testPID {
		t.Fatalf("record = %+v, want source loop and pid %d", got[0], testPID)
	}
}

func TestPublisher_StampsTheDispatchIDOfItsProcess(t *testing.T) {
	t.Parallel()
	const id = "events/0/sub-alerts/p4242n1t0"
	cfg := testConfig(t, lossless("loop", ""), bestEffort("signals", ""))
	cfg.Dispatch = id
	p := startPublisher(t, cfg, openLog)

	p.Listen(loopEvent(1))
	p.Listen(loopEvent(2))
	p.Close(time.Minute)

	for _, name := range []string{"loop", "signals"} {
		got := readChannel(t, cfg, name)
		if len(got) != 2 {
			t.Fatalf("%s records = %d, want 2", name, len(got))
		}
		for _, r := range got {
			if r.Dispatch != id || r.Source != "loop" {
				t.Fatalf("%s record = %+v, want dispatch %q and source loop", name, r, id)
			}
		}
	}
}

func TestPublisher_AProcessWithNoDispatchIDHasNoDispatchKey(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, lossless("loop", ""))
	p := startPublisher(t, cfg, openLog)

	p.Listen(loopEvent(1))

	got := readChannel(t, cfg, "loop")
	if len(got) != 1 || got[0].Dispatch != "" {
		t.Fatalf("records = %+v, want one record with no dispatch", got)
	}
}

func TestPublisher_CutsALongDispatchStampTo256Bytes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, id, want string
	}{
		{"exactly the limit", strings.Repeat("a", 256), strings.Repeat("a", 256)},
		{"one byte over", strings.Repeat("a", 257), strings.Repeat("a", 256)},
		{"a rune across the limit", strings.Repeat("a", 255) + "é", strings.Repeat("a", 255)},
	}
	if MaxDispatchBytes != 256 {
		t.Fatalf("MaxDispatchBytes = %d, want 256 (event-channels.md §3)", MaxDispatchBytes)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := testConfig(t, lossless("loop", ""))
			cfg.Dispatch = tc.id
			p := startPublisher(t, cfg, openLog)

			p.Listen(loopEvent(1))

			got := readChannel(t, cfg, "loop")
			if len(got) != 1 || got[0].Dispatch != tc.want {
				t.Fatalf("dispatch = %q (%d bytes), want %d bytes", got[0].Dispatch, len(got[0].Dispatch), len(tc.want))
			}
		})
	}
}

func TestPublisher_ListenNeverPanicsAndNeverEmits(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, lossless("loop", ""), bestEffort("signals", ""))
	_, open := scripted("loop", errors.New("disk full"))
	p := startPublisher(t, cfg, open)
	center := signalcenter.New(signalcenter.WithPID(testPID))
	center.Subscribe(p.Listen)

	center.Emit(signalcenter.Event{})
	center.Emit(loopEvent(0))
	center.Flush()
	p.Close(time.Minute)
	center.Flush()

	if got := center.Recent(); len(got) != 2 {
		t.Fatalf("Center saw %d events, want the 2 emitted ones: the publisher never emits and never panics: %+v", len(got), got)
	}
}

func TestNew_RefusesABadConfig(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		mutate func(*Config)
		want   error
		text   string
	}{
		{"a route that fails to parse", func(c *Config) { c.Channels = []Channel{lossless("loop", "colour=red")} }, filter.ErrUsage, "channel loop"},
		{"a route that names no kind", func(c *Config) { c.Channels = []Channel{lossless("loop", "kind=nope.*")} }, filter.ErrRefused, "channel loop"},
		{"an unknown QoS", func(c *Config) { c.Channels = []Channel{{Name: "loop", QoS: "maybe"}} }, nil, `qos "maybe"`},
		{"a bad channel name", func(c *Config) { c.Channels = []Channel{lossless("Loop", "")} }, nil, "dot-separated"},
		{"a duplicate channel", func(c *Config) { c.Channels = []Channel{lossless("loop", ""), bestEffort("loop", "")} }, nil, "channel loop is configured twice"},
		{"an empty role", func(c *Config) { c.Role = "" }, nil, "role"},
		{"a role outside the closed set", func(c *Config) { c.Role = "console" }, nil, `role "console"`},
		{"the reader role", func(c *Config) { c.Role = channel.SourceWatch }, nil, "role"},
		{"no queue events", func(c *Config) { c.QueueEvents = 0 }, nil, "queue"},
		{"no queue bytes", func(c *Config) { c.QueueBytes = 0 }, nil, "queue"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := testConfig(t, lossless("loop", ""))
			tc.mutate(&cfg)

			p, err := newPublisher(cfg, openLog)

			if p != nil || err == nil || !strings.Contains(err.Error(), tc.text) || (tc.want != nil && !errors.Is(err, tc.want)) {
				t.Fatalf("newPublisher = %v, %v, want an error with %q that wraps %v", p, err, tc.text, tc.want)
			}
		})
	}
}

func TestNew_BuildsAPublisherOverTheChannelLogs(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, lossless("loop", ""))

	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	p.Listen(loopEvent(1))

	if n := p.Close(time.Minute); n != 0 {
		t.Fatalf("Close = %d, want 0", n)
	}
	assertRecords(t, readChannel(t, cfg, "loop"), []string{"seq:1"})
}

func TestRoles_IsTheClosedSetOfTheSpecRoots(t *testing.T) {
	t.Parallel()
	want := []string{"cycle", "loop", "loop-chain", "ship", "subagent", "simulate", "phase-observer", "wave", "inbox", "ci", "subscriber"}

	got := Roles()
	got[0] = "changed"

	if again := Roles(); !reflect.DeepEqual(again, want) {
		t.Fatalf("Roles() = %q, want %q (event-channels.md §1), and a copy each call", again, want)
	}
	for _, role := range want {
		cfg := testConfig(t, lossless("loop", ""))
		cfg.Role = role
		p, err := newPublisher(cfg, openLog)
		if err != nil {
			t.Fatalf("newPublisher(role %q) = %v, want it accepted", role, err)
		}
		p.Close(time.Minute)
	}
}

func TestPublisher_AChannelSegmentSizeOverridesTheDefault(t *testing.T) {
	t.Parallel()
	small := lossless("small", "")
	small.SegmentBytes = 1
	cfg := testConfig(t, small, lossless("big", ""))
	p := startPublisher(t, cfg, openLog)

	p.Listen(loopEvent(1))
	p.Listen(loopEvent(2))

	for name, want := range map[string]int{"small": 3, "big": 1} {
		l, err := channel.New(cfg.Root, name, cfg.Log)
		if err != nil {
			t.Fatal(err)
		}
		segs, err := l.Segments()
		if err != nil || len(segs) != want {
			t.Fatalf("%s segments = %d, %v, want %d", name, len(segs), err, want)
		}
	}
}
