package reader

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	kindLoopStarted signalcenter.Kind = "loop.started"
	kindLoopExit    signalcenter.Kind = "loop.exit"
	kindLoopLost    signalcenter.Kind = "loop.lost"
	codeLoopLost    signalcenter.Code = "LOOP_LOST"
)

type runKey struct {
	source string
	pid    int
}

type liveKey struct {
	pid       int
	procStart string
}

type run struct {
	source    string
	pid       int
	procStart string
	runID     string
	cursor    int64
	incident  bool
}

type lastWill struct {
	log        *channel.Log
	read       func(*channel.Log, int64) (channel.Batch, error)
	startOf    func(int) (string, error)
	now        func() time.Time
	selected   bool
	start      int64
	cur        int64
	base       int64
	seeded     bool
	open       map[runKey]*run
	live       map[liveKey]*run
	unverified []liveKey
	suspects   []liveKey
	pending    []channel.Record
}

func (r *Reader) startWill() (*lastWill, error) {
	w := &lastWill{read: r.readLog, startOf: r.ports.StartOf, now: r.ports.Now, open: map[runKey]*run{}, live: map[liveKey]*run{}}
	if f := r.follower(loopChannel); f != nil {
		w.log, w.start, w.selected = f.log, f.cur, true
		return w, nil
	}
	l, err := r.newLog(loopChannel)
	if err != nil {
		return nil, err
	}
	w.log = l
	w.start, err = r.cfg.Since.willStart(l)
	return w, err
}

func (w *lastWill) seed() error {
	if err := w.scan(); err != nil {
		return err
	}
	if !w.selected && w.start < w.base {
		w.pending = append(w.pending, channel.Record{Source: channel.SourceWatch, Gap: &channel.Gap{Reason: channel.ReasonRetention, From: w.start, To: w.base}})
	}
	runs := make([]*run, 0, len(w.open))
	for _, r := range w.open {
		runs = append(runs, r)
	}
	slices.SortFunc(runs, func(a, b *run) int { return cmp.Compare(a.cursor, b.cursor) })
	for _, r := range runs {
		if err := w.classify(r); err != nil {
			return err
		}
	}
	w.seeded = true
	return nil
}

func (w *lastWill) classify(r *run) error {
	same, err := w.same(r)
	if err != nil {
		return err
	}
	if same {
		w.addLive(r)
		return nil
	}
	if w.start <= r.cursor {
		w.pending = append(w.pending, w.lost(r, true))
	}
	return nil
}

func (w *lastWill) scan() error {
	for {
		b, err := w.read(w.log, w.cur)
		if err != nil {
			return err
		}
		for _, rec := range b.Records {
			w.observe(rec)
		}
		w.cur = b.Next
		if len(b.Records) == 0 {
			return nil
		}
	}
}

func (w *lastWill) observe(rec channel.Record) {
	switch {
	case rec.Source == channel.SourceWatch:
		if rec.Gap.Reason == channel.ReasonRetention && rec.Gap.From == 0 {
			w.base = rec.Gap.To
		}
	case rec.Gap != nil:
		if r, ok := w.open[runKey{rec.Source, rec.Gap.PID}]; ok && rec.Gap.Severity == signalcenter.SeverityIncident {
			r.incident = true
		}
	case rec.Signal.Kind == kindLoopStarted:
		w.started(rec)
	case rec.Signal.Kind == kindLoopExit:
		w.ended(runKey{rec.Source, rec.Signal.PID})
	}
}

func (w *lastWill) started(rec channel.Record) {
	s := rec.Signal
	r := &run{source: rec.Source, pid: s.PID, procStart: s.Fields["proc_start"], runID: s.RunID, cursor: rec.Cursor}
	w.open[runKey{r.source, r.pid}] = r
	if w.seeded {
		w.addLive(r)
	}
}

func (w *lastWill) ended(k runKey) {
	delete(w.open, k)
	for key, r := range w.live {
		if r.source == k.source && r.pid == k.pid {
			delete(w.live, key)
		}
	}
}

func (w *lastWill) addLive(r *run) {
	key := liveKey{r.pid, r.procStart}
	if _, ok := w.live[key]; ok {
		return
	}
	w.live[key] = r
	w.unverified = append(w.unverified, key)
}

func (w *lastWill) same(r *run) (bool, error) {
	now, err := w.startOf(r.pid)
	if errors.Is(err, syscall.ESRCH) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reader: read the start of pid %d: %w", r.pid, err)
	}
	return now == r.procStart, nil
}

func (w *lastWill) verify() error {
	keys := w.unverified
	w.unverified = nil
	for _, key := range keys {
		r, ok := w.live[key]
		if !ok {
			continue
		}
		same, err := w.same(r)
		if err != nil {
			return err
		}
		if !same {
			w.suspects = append(w.suspects, key)
		}
	}
	return nil
}

func (w *lastWill) exited(pids []int) {
	for key := range w.live {
		if slices.Contains(pids, key.pid) {
			w.suspects = append(w.suspects, key)
		}
	}
}

func (w *lastWill) catchUp() ([]channel.Record, error) {
	if err := w.scan(); err != nil {
		return nil, err
	}
	for _, key := range w.suspects {
		if r, ok := w.live[key]; ok {
			w.pending = append(w.pending, w.lost(r, false))
			delete(w.live, key)
			delete(w.open, runKey{r.source, r.pid})
		}
	}
	w.suspects = nil
	out := w.pending
	w.pending = nil
	return out, nil
}

func (w *lastWill) pids() []int {
	pids := make([]int, 0, len(w.live))
	for key := range w.live {
		pids = append(pids, key.pid)
	}
	slices.Sort(pids)
	return slices.Compact(pids)
}

func (w *lastWill) lost(r *run, historical bool) channel.Record {
	exit := "crash"
	if r.incident {
		exit = "unknown"
	}
	fields := map[string]string{"pid": strconv.Itoa(r.pid), "run_id": r.runID, "exit": exit}
	if historical {
		fields["historical"] = "true"
	}
	return channel.Record{Source: channel.SourceWatch, Signal: &signalcenter.Event{
		SchemaVersion: signalcenter.SchemaVersion, PID: r.pid, TS: w.now().UTC().Format(time.RFC3339Nano),
		RunID: r.runID, Module: signalcenter.ModuleLoop, Origin: "events.reader", Kind: kindLoopLost,
		Code: codeLoopLost, Severity: signalcenter.SeverityIncident, Reason: "the loop process ended without loop.exit", Fields: fields,
	}}
}

func (w *lastWill) requeue(recs []channel.Record) {
	w.pending = recs
}
