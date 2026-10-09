package publisher

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const testPID = 4242

func lossless(name, route string) Channel {
	return Channel{Name: name, Route: route, QoS: QoSLossless}
}

func bestEffort(name, route string) Channel {
	return Channel{Name: name, Route: route, QoS: QoSBestEffort}
}

func testConfig(t *testing.T, channels ...Channel) Config {
	t.Helper()
	return Config{
		Root:        filepath.Join(t.TempDir(), "ch"),
		Role:        "loop",
		Channels:    channels,
		Log:         channel.Config{SegmentBytes: 1 << 20, LockDeadline: time.Minute},
		QueueEvents: 64,
		QueueBytes:  1 << 20,
	}
}

func startPublisher(t *testing.T, cfg Config, open opener) *Publisher {
	t.Helper()
	p, err := newPublisher(cfg, open)
	if err != nil {
		t.Fatalf("newPublisher = %v", err)
	}
	t.Cleanup(func() { p.Close(time.Minute) })
	return p
}

func loopEvent(seq uint64) signalcenter.Event {
	return signalcenter.Event{
		SchemaVersion: signalcenter.SchemaVersion, Seq: seq, PID: testPID, TS: "2026-10-09T17:46:02.114Z",
		Module: signalcenter.ModuleLoop, Origin: "test", Kind: signalcenter.KindLoopWave,
		Severity: signalcenter.SeverityInfo, Reason: "wave",
	}
}

func readChannel(t *testing.T, cfg Config, name string) []channel.Record {
	t.Helper()
	l, err := channel.New(cfg.Root, name, cfg.Log)
	if err != nil {
		t.Fatalf("channel.New(%s) = %v", name, err)
	}
	b, err := l.Read(0)
	if err != nil {
		t.Fatalf("Read(%s) = %v", name, err)
	}
	return b.Records
}

func signalSeqs(records []channel.Record) []uint64 {
	var seqs []uint64
	for _, r := range records {
		if r.Signal != nil {
			seqs = append(seqs, r.Signal.Seq)
		}
	}
	return seqs
}

func gapsOf(records []channel.Record) []channel.Gap {
	var gaps []channel.Gap
	for _, r := range records {
		if r.Gap != nil {
			gaps = append(gaps, *r.Gap)
		}
	}
	return gaps
}

func wantGap(reason string, severity signalcenter.Severity, first, last uint64, dropped int) channel.Gap {
	return channel.Gap{Reason: reason, Severity: severity, PID: testPID, FirstSeq: first, LastSeq: last, Dropped: dropped}
}

func assertRecords(t *testing.T, got []channel.Record, want []string) {
	t.Helper()
	shape := make([]string, len(got))
	for i, r := range got {
		shape[i] = describe(r)
	}
	if !reflect.DeepEqual(shape, want) {
		t.Fatalf("records = %q, want %q", shape, want)
	}
}

func describe(r channel.Record) string {
	if r.Gap != nil {
		return "gap:" + r.Gap.Reason
	}
	return "seq:" + strconv.FormatUint(r.Signal.Seq, 10)
}

type scriptedLog struct {
	mu      sync.Mutex
	errs    []error
	batches [][]channel.Record
	real    appender
}

func (s *scriptedLog) Append(records []channel.Record) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.batches = append(s.batches, records)
	if len(s.errs) == 0 {
		return s.real.Append(records)
	}
	err := s.errs[0]
	s.errs = s.errs[1:]
	if errors.Is(err, channel.ErrRotate) {
		_, _ = s.real.Append(records)
	}
	return 0, err
}

func scripted(name string, errs ...error) (*scriptedLog, opener) {
	s := &scriptedLog{errs: errs}
	return s, func(root, n string, cfg channel.Config) (appender, error) {
		real, err := openLog(root, n, cfg)
		if n == name {
			s.real = real
			return s, err
		}
		return real, err
	}
}

type gatedLog struct {
	real    appender
	entered chan []channel.Record
	release chan struct{}
	fail    error
	mu      sync.Mutex
	calls   int
	gates   int
}

func (g *gatedLog) Append(records []channel.Record) (int64, error) {
	g.mu.Lock()
	g.calls++
	call := g.calls
	g.mu.Unlock()
	if call <= g.gates {
		g.entered <- records
		<-g.release
	}
	if call == 1 && g.fail != nil {
		return 0, g.fail
	}
	return g.real.Append(records)
}

func gated(name string) (*gatedLog, opener) {
	g := &gatedLog{entered: make(chan []channel.Record, 1), release: make(chan struct{}), gates: 1}
	return g, func(root, n string, cfg channel.Config) (appender, error) {
		real, err := openLog(root, n, cfg)
		if n == name {
			g.real = real
			return g, err
		}
		return real, err
	}
}

func finishes(t *testing.T, done <-chan struct{}, what string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatalf("%s did not return within 5s", what)
	}
}

func expired(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}
