package signalcenter

import (
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"sync"
	"time"
)

const CodeExpectedAbsent Code = "SIGNAL_EXPECTED_ABSENT"

func init() {
	RegisterCode(ModuleSignal, CodeExpectedAbsent, "a stage emitted its start signal and its end signal did not follow before the deadline from policy; fields.pair names the pair, start_kind and end_kind the two kinds, started the start time, deadline_seconds the limit and key the open start")
}

type ExpectedPair struct {
	Name        string
	Start       Kind
	Ends        []Kind
	DeadlineKey string
	Key         func(Event) string
	StartWhen   func(Event) bool
	EndWhen     func(Event) bool
}

func (p ExpectedPair) ends(k Kind) bool {
	for _, end := range p.Ends {
		if end == k {
			return true
		}
	}
	return false
}

func (p ExpectedPair) endNames() string {
	names := make([]string, len(p.Ends))
	for i, end := range p.Ends {
		names[i] = string(end)
	}
	return strings.Join(names, ",")
}

const (
	OriginPhaseDispatched = "Orchestrator.emitPhaseDispatched"
	OriginPhaseOutcome    = "Orchestrator.recordPhaseOutcome"
)

func orchestratorDispatch(e Event) bool { return e.Cycle != 0 && e.Origin == OriginPhaseDispatched }

func orchestratorOutcome(e Event) bool { return e.Origin == OriginPhaseOutcome }

func keyByRunID(e Event) string { return e.Fields["run_id"] }

func keyByCyclePhase(e Event) string { return fmt.Sprintf("%d|%s", e.Cycle, e.Phase) }

func DefaultExpectedPairs() []ExpectedPair {
	return []ExpectedPair{
		{Name: "wave", Start: KindWaveStarted, Ends: []Kind{KindWaveEnded}, DeadlineKey: "expected_signal.wave_seconds", Key: keyByRunID},
		{Name: "phase", Start: KindPhaseDispatched, Ends: []Kind{KindPhaseOutcome, KindPhaseAborted}, DeadlineKey: "expected_signal.phase_seconds", Key: keyByCyclePhase, StartWhen: orchestratorDispatch, EndWhen: orchestratorOutcome},
	}
}

func ExpectedKinds() map[Kind]bool {
	kinds := map[Kind]bool{KindExpectedAbsent: true}
	for _, p := range DefaultExpectedPairs() {
		kinds[p.Start] = true
		for _, end := range p.Ends {
			kinds[end] = true
		}
	}
	return kinds
}

type openStart struct {
	pair    ExpectedPair
	started time.Time
	event   Event
}

type ExpectedSignals struct {
	mu       sync.Mutex
	center   *Center
	pairs    []ExpectedPair
	deadline func(key string) (time.Duration, bool)
	now      func() time.Time
	open     map[string]openStart
	offset   int64
}

func NewExpectedSignals(c *Center, pairs []ExpectedPair, deadline func(string) (time.Duration, bool), now func() time.Time) *ExpectedSignals {
	return &ExpectedSignals{center: c, pairs: pairs, deadline: deadline, now: now, open: map[string]openStart{}}
}

func (x *ExpectedSignals) Poll(path string) error {
	x.mu.Lock()
	from := x.offset
	x.mu.Unlock()
	chunk, err := ReadStream(path, from)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	x.mu.Lock()
	x.offset = chunk.Next
	x.mu.Unlock()
	for _, e := range chunk.Events {
		x.track(e)
	}
	for _, due := range x.expire() {
		x.center.Emit(due)
	}
	return nil
}

func (x *ExpectedSignals) track(e Event) {
	x.mu.Lock()
	defer x.mu.Unlock()
	if e.Kind == KindExpectedAbsent {
		delete(x.open, e.Fields["key"])
		return
	}
	for _, p := range x.pairs {
		switch {
		case e.Kind == p.Start:
			if p.StartWhen != nil && !p.StartWhen(e) {
				continue
			}
			x.open[openKey(p, e)] = openStart{pair: p, started: x.startedAt(e), event: e}
		case p.ends(e.Kind):
			if p.EndWhen != nil && !p.EndWhen(e) {
				continue
			}
			delete(x.open, openKey(p, e))
		}
	}
}

func (x *ExpectedSignals) startedAt(e Event) time.Time {
	if at, err := time.Parse(time.RFC3339Nano, e.TS); err == nil {
		return at
	}
	return x.now()
}

func (x *ExpectedSignals) expire() []Event {
	x.mu.Lock()
	defer x.mu.Unlock()
	now := x.now()
	var due []Event
	for key, o := range x.open {
		limit, ok := x.deadline(o.pair.DeadlineKey)
		if !ok || now.Sub(o.started) < limit {
			continue
		}
		delete(x.open, key)
		due = append(due, absentEvent(key, o, limit))
	}
	return due
}

func absentEvent(key string, o openStart, limit time.Duration) Event {
	return Event{
		Module: ModuleSignal, Origin: "ExpectedSignals.expire", Kind: KindExpectedAbsent,
		Code: CodeExpectedAbsent, Severity: SeverityWarn,
		Reason: fmt.Sprintf("%s: %s was not followed by %s within %s", o.pair.Name, o.pair.Start, o.pair.endNames(), limit),
		Cycle:  o.event.Cycle, RunID: o.event.RunID, Phase: o.event.Phase,
		Fields: map[string]string{
			"pair": o.pair.Name, "start_kind": string(o.pair.Start), "end_kind": o.pair.endNames(),
			"started": o.started.UTC().Format(time.RFC3339Nano), "deadline_seconds": strconv.Itoa(int(limit.Seconds())), "key": key,
		},
	}
}

func openKey(p ExpectedPair, e Event) string { return p.Name + "|" + p.Key(e) }
