package publisher

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/filter"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var severities = map[QoS]signalcenter.Severity{
	QoSLossless:   signalcenter.SeverityIncident,
	QoSBestEffort: signalcenter.SeverityWarn,
}

type loss struct {
	reason string
	pid    int
	first  uint64
	last   uint64
}

func (l loss) dropped() int { return int(l.last - l.first + 1) }

type route struct {
	filter   filter.Filter
	log      appender
	stamp    channel.Record
	severity signalcenter.Severity
	queue    *queue
	appendMu sync.Mutex
	lossMu   sync.Mutex
	losses   []loss
}

func newRoute(cfg Config, ch Channel, cat filter.Catalog, stamp channel.Record, open opener) (*route, error) {
	severity, ok := severities[ch.QoS]
	if !ok {
		return nil, fmt.Errorf("publisher: channel %s: qos %q is not lossless or best_effort", ch.Name, ch.QoS)
	}
	f, _, err := filter.Parse(ch.Route, cat)
	if err != nil {
		return nil, fmt.Errorf("publisher: channel %s: route %q: %w", ch.Name, ch.Route, err)
	}
	logCfg := cfg.Log
	if ch.SegmentBytes > 0 {
		logCfg.SegmentBytes = ch.SegmentBytes
	}
	log, err := open(cfg.Root, ch.Name, logCfg)
	if err != nil {
		return nil, fmt.Errorf("publisher: %w", err)
	}
	r := &route{filter: f, log: log, stamp: stamp, severity: severity}
	if ch.QoS == QoSBestEffort {
		r.queue = newQueue(cfg)
	}
	return r, nil
}

func (r *route) publish(rec channel.Record) {
	if r.queue == nil {
		r.write([]channel.Record{rec})
		return
	}
	if reason, ok := r.queue.push(rec); !ok {
		r.lose(reason, []channel.Record{rec})
	}
}

func (r *route) write(signals []channel.Record) {
	r.appendMu.Lock()
	defer r.appendMu.Unlock()
	pending := r.takeLosses()
	_, err := r.log.Append(append(r.gaps(pending), signals...))
	if !written(err) {
		r.restore(mergeLosses(pending, lossOf(lossReason(err), signals)))
	}
}

func (r *route) flushGaps(ctx context.Context) int {
	r.appendMu.Lock()
	defer r.appendMu.Unlock()
	pending := r.takeLosses()
	if len(pending) > 0 && ctx.Err() == nil {
		if _, err := r.log.Append(r.gaps(pending)); written(err) {
			pending = nil
		}
	}
	return r.restore(pending)
}

func (r *route) lose(reason string, signals []channel.Record) {
	r.lossMu.Lock()
	defer r.lossMu.Unlock()
	r.losses = mergeLosses(r.losses, lossOf(reason, signals))
}

func (r *route) takeLosses() []loss {
	r.lossMu.Lock()
	defer r.lossMu.Unlock()
	pending := r.losses
	r.losses = nil
	return pending
}

func (r *route) restore(older []loss) int {
	r.lossMu.Lock()
	defer r.lossMu.Unlock()
	r.losses = mergeLosses(older, r.losses)
	lost := 0
	for _, l := range r.losses {
		lost += l.dropped()
	}
	return lost
}

func (r *route) gaps(losses []loss) []channel.Record {
	records := make([]channel.Record, 0, len(losses)+1)
	for _, l := range losses {
		rec := r.stamp
		rec.Gap = &channel.Gap{Reason: l.reason, Severity: r.severity, PID: l.pid, FirstSeq: l.first, LastSeq: l.last, Dropped: l.dropped()}
		records = append(records, rec)
	}
	return records
}

func written(err error) bool {
	return err == nil || errors.Is(err, channel.ErrRotate)
}

func lossReason(err error) string {
	if errors.Is(err, flock.ErrLockDeadline) {
		return channel.ReasonLockDeadline
	}
	return channel.ReasonWriteError
}

func lossOf(reason string, signals []channel.Record) []loss {
	losses := make([]loss, 0, len(signals))
	for _, rec := range signals {
		losses = append(losses, loss{reason: reason, pid: rec.Signal.PID, first: rec.Signal.Seq, last: rec.Signal.Seq})
	}
	return losses
}

func mergeLosses(older, newer []loss) []loss {
	all := append(append([]loss(nil), older...), newer...)
	sort.SliceStable(all, func(i, j int) bool { return lossBefore(all[i], all[j]) })
	out := make([]loss, 0, len(all))
	for _, l := range all {
		n := len(out)
		if n > 0 && sameRun(out[n-1], l) {
			out[n-1].last = max(out[n-1].last, l.last)
			continue
		}
		out = append(out, l)
	}
	return out
}

func lossBefore(a, b loss) bool {
	if a.pid != b.pid {
		return a.pid < b.pid
	}
	return a.first < b.first
}

func sameRun(prev, next loss) bool {
	return prev.reason == next.reason && prev.pid == next.pid && next.first <= prev.last+1
}
