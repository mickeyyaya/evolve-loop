package reader

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/filter"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/wake"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	loopPid  = 9055
	loopPid2 = 9077
	lanePid  = 7001
	procA    = "1760011223.004567"
	procB    = "1760011999.000001"
)

type script struct {
	calls     []string
	arms      []wake.Targets
	deadlines []time.Time
	wakes     []func() (wake.Wake, error)
	onArm     func(n int)
	armErr    error
	starts    map[int][]string
	startErr  error
	clockN    int
}

func newScript() *script {
	return &script{starts: map[int][]string{}}
}

func (s *script) Arm(t wake.Targets) error {
	s.calls = append(s.calls, "arm")
	s.arms = append(s.arms, t)
	if s.onArm != nil {
		s.onArm(len(s.arms))
	}
	return s.armErr
}

func (s *script) Wait(_ context.Context, deadline time.Time) (wake.Wake, error) {
	s.calls = append(s.calls, "wait")
	s.deadlines = append(s.deadlines, deadline)
	if len(s.wakes) == 0 {
		return wake.Wake{Deadline: true}, nil
	}
	next := s.wakes[0]
	s.wakes = s.wakes[1:]
	return next()
}

func (s *script) then(f func() (wake.Wake, error)) {
	s.wakes = append(s.wakes, f)
}

func (s *script) startOf(pid int) (string, error) {
	s.calls = append(s.calls, "start:"+strconv.Itoa(pid))
	if s.startErr != nil {
		return "", s.startErr
	}
	seq := s.starts[pid]
	if len(seq) == 0 {
		return "", syscall.ESRCH
	}
	if len(seq) > 1 {
		s.starts[pid] = seq[1:]
	}
	if seq[0] == "gone" {
		return "", syscall.ESRCH
	}
	return seq[0], nil
}

func (s *script) now() time.Time {
	s.clockN++
	return time.Date(2026, 10, 9, 18, 0, s.clockN, 0, time.UTC)
}

func (s *script) count(call string) int {
	n := 0
	for _, c := range s.calls {
		if c == call {
			n++
		}
	}
	return n
}

func (s *script) only(keep ...string) []string {
	var out []string
	for _, c := range s.calls {
		if slices.Contains(keep, c) {
			out = append(out, c)
		}
	}
	return out
}

func (s *script) lastArm() wake.Targets {
	return s.arms[len(s.arms)-1]
}

type fixture struct {
	t    *testing.T
	root string
	seg  int64
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	return &fixture{t: t, root: filepath.Join(t.TempDir(), "ch"), seg: 1 << 20}
}

func (f *fixture) log(name string) *channel.Log {
	f.t.Helper()
	l, err := channel.New(f.root, name, channel.Config{SegmentBytes: f.seg, LockDeadline: time.Minute})
	if err != nil {
		f.t.Fatal(err)
	}
	return l
}

func (f *fixture) append(name string, recs ...channel.Record) int64 {
	f.t.Helper()
	at, err := f.log(name).Append(recs)
	if err != nil {
		f.t.Fatalf("append to %s: %v", name, err)
	}
	return at
}

func (f *fixture) dir(name string) string {
	return filepath.Join(f.root, name)
}

func (f *fixture) segments(name string) []channel.Segment {
	f.t.Helper()
	segs, err := f.log(name).Segments()
	if err != nil {
		f.t.Fatal(err)
	}
	return segs
}

func route(t *testing.T, expr string) filter.Filter {
	t.Helper()
	flt, _, err := filter.Parse(expr, filter.RegisteredCatalog())
	if err != nil {
		t.Fatalf("route %q: %v", expr, err)
	}
	return flt
}

func testCatalog(t *testing.T) []Channel {
	t.Helper()
	return []Channel{
		{Name: "loop", Route: route(t, "module=loop")},
		{Name: "cycle", Route: route(t, "kind=cycle.sealed")},
		{Name: "errors", Route: route(t, "severity>=WARN")},
	}
}

func (f *fixture) reader(s *script, selectors []string, since Since) *Reader {
	f.t.Helper()
	return f.readerOver(testCatalog(f.t), s, selectors, since)
}

func (f *fixture) readerOver(cat []Channel, s *script, selectors []string, since Since) *Reader {
	f.t.Helper()
	r := New(Config{Root: f.root, Catalog: cat, Selectors: selectors, Since: since}, Ports{Waiter: s, StartOf: s.startOf, Now: s.now})
	read := r.read
	r.read = func(l *channel.Log, from, maxBytes int64) (channel.Batch, error) {
		s.calls = append(s.calls, "read:"+filepath.Base(l.Dir()))
		return read(l, from, maxBytes)
	}
	return r
}

func sig(pid int, seq uint64, kind signalcenter.Kind, sev signalcenter.Severity) channel.Record {
	return channel.Record{Source: "loop", Signal: &signalcenter.Event{
		SchemaVersion: signalcenter.SchemaVersion, Seq: seq, PID: pid,
		TS:     fmt.Sprintf("2026-10-09T17:46:%02d.000Z", seq%60),
		Module: signalcenter.ModuleLoop, Origin: "test", Kind: kind, Severity: sev, Reason: "r",
	}}
}

func sealed(seq uint64) channel.Record {
	r := sig(4242, seq, signalcenter.KindCycleSealed, signalcenter.SeverityInfo)
	r.Signal.Module = signalcenter.ModuleOrchestrator
	return r
}

func loopStarted(pid int, seq uint64, procStart string) channel.Record {
	r := sig(pid, seq, kindLoopStarted, signalcenter.SeverityInfo)
	r.Signal.RunID = "run-" + strconv.Itoa(pid)
	r.Signal.Fields = map[string]string{"proc_start": procStart}
	return r
}

func loopExit(pid int, seq uint64) channel.Record {
	return sig(pid, seq, kindLoopExit, signalcenter.SeverityInfo)
}

func incidentGap(pid int, reason string, sev signalcenter.Severity) channel.Record {
	return channel.Record{Source: "loop", Gap: &channel.Gap{Reason: reason, Severity: sev, PID: pid, FirstSeq: 5, LastSeq: 5, Dropped: 1}}
}

func next(t *testing.T, r *Reader) []Item {
	t.Helper()
	items, err := r.Next(context.Background(), time.Time{})
	if err != nil {
		t.Fatalf("Next = %v", err)
	}
	return items
}

func drain(t *testing.T, r *Reader) []Item {
	t.Helper()
	var all []Item
	for {
		items, err := r.Next(context.Background(), time.Time{})
		all = append(all, items...)
		if err == ErrDeadline {
			return all
		}
		if err != nil {
			t.Fatalf("Next = %v", err)
		}
	}
}

func describe(items []Item) string {
	var parts []string
	for _, it := range items {
		switch {
		case it.Record.Gap != nil:
			parts = append(parts, fmt.Sprintf("%s:gap(%s %d->%d)", it.Channel, it.Record.Gap.Reason, it.Record.Gap.From, it.Record.Gap.To))
		default:
			parts = append(parts, fmt.Sprintf("%s:%s#%d", it.Channel, it.Record.Signal.Kind, it.Record.Signal.Seq))
		}
	}
	return strings.Join(parts, " ")
}

func losses(items []Item) []Item {
	var out []Item
	for _, it := range items {
		if it.Record.Signal != nil && it.Record.Signal.Kind == kindLoopLost {
			out = append(out, it)
		}
	}
	return out
}

func mustRemove(t *testing.T, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
}

func since(t *testing.T, value string) Since {
	t.Helper()
	s, err := ParseSince(value)
	if err != nil {
		t.Fatalf("ParseSince(%q) = %v", value, err)
	}
	return s
}
