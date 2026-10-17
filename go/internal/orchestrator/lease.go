package orchestrator

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type Timer interface{ Stop() bool }

type Clock interface {
	Now() time.Time
	AfterFunc(d time.Duration, f func()) Timer
}

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

func (SystemClock) AfterFunc(d time.Duration, f func()) Timer { return time.AfterFunc(d, f) }

type Lane struct {
	Key     string
	Scope   []string
	Started time.Time
}

type Heartbeat func(Lane) (time.Time, bool)

type LeaseConfig struct {
	Stage    string
	Deadline time.Duration
	Stall    time.Duration
}

type Return struct {
	Stopped bool
	Late    bool
	Stops   int
	Result  fleet.Result
}

type Settle func(Lane, Return)

type lease struct {
	lane     Lane
	stop     context.CancelFunc
	late     bool
	stopped  bool
	stalled  bool
	seq      int
	deadline time.Time
}

type Leases struct {
	cfg     LeaseConfig
	clock   Clock
	beat    Heartbeat
	publish Publish

	mu    sync.Mutex
	live  map[string]*lease
	stops map[string]int
	due   dueHeap
	timer Timer
}

func NewLeases(cfg LeaseConfig, clock Clock, beat Heartbeat, publish Publish) *Leases {
	return &Leases{cfg: cfg, clock: clock, beat: beat, publish: publish, live: map[string]*lease{}, stops: map[string]int{}}
}

func (l *Leases) Watch(launch fleet.LaunchFn, settle Settle) fleet.LaunchFn {
	return func(ctx context.Context, spec fleet.CycleSpec) (int, error) {
		laneCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		lane := l.grant(spec.Scope, cancel)
		code, err := launch(laneCtx, spec)
		ret, gerr := l.give(lane, fleet.Result{ExitCode: code, Err: err})
		if settle != nil {
			settle(lane, ret)
		}
		return code, errors.Join(err, gerr)
	}
}

func (l *Leases) grant(scope []string, stop context.CancelFunc) Lane {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.clock.Now()
	lane := Lane{Key: strings.Join(scope, "+"), Scope: scope, Started: now}
	ls := &lease{lane: lane, stop: stop, deadline: now.Add(l.cfg.Deadline)}
	l.live[lane.Key] = ls
	l.schedule(ls, now, now)
	return lane
}

func (l *Leases) give(lane Lane, r fleet.Result) (Return, error) {
	l.mu.Lock()
	ls, ok := l.live[lane.Key]
	if !ok {
		l.mu.Unlock()
		return Return{Result: r}, fmt.Errorf("orchestrator: lane %s has no lease; a second lane holds the same key", lane.Key)
	}
	delete(l.live, lane.Key)
	ret := Return{Stopped: ls.stopped, Late: ls.late, Result: r}
	if ls.stopped {
		l.stops[lane.Key]++
	}
	ret.Stops = l.stops[lane.Key]
	l.rearm(l.clock.Now())
	l.mu.Unlock()
	if ret.Late {
		l.publish.emit("Leases.give", CodeLaneLateExit, signalcenter.SeverityInfo,
			"lane "+lane.Key+" exited after its deadline ("+string(r.Status())+")",
			map[string]string{"todo": lane.Key, "stage": l.cfg.Stage, "status": string(r.Status()),
				"rc": strconv.Itoa(r.ExitCode), "stopped": strconv.FormatBool(ret.Stopped)})
	}
	return ret, nil
}

func (l *Leases) fire() {
	l.mu.Lock()
	now := l.clock.Now()
	var acts []func()
	for len(l.due) > 0 && !l.due[0].at.After(now) {
		ls := l.live[heap.Pop(&l.due).(due).key]
		beat, known := l.heartbeat(ls, now)
		acts = append(acts, l.check(ls, now, beat, known)...)
		l.schedule(ls, now, beat)
	}
	l.rearm(now)
	l.mu.Unlock()
	for _, act := range acts {
		act()
	}
}

func (l *Leases) check(ls *lease, now, beat time.Time, known bool) []func() {
	var acts []func()
	if !ls.late && !now.Before(ls.deadline) {
		ls.late = true
		acts = append(acts, l.passDeadline(ls))
	}
	if known && l.cfg.Stall > 0 && !ls.stalled && now.Sub(beat) >= l.cfg.Stall {
		ls.stalled = true
		acts = append(acts, l.reportStall(ls, beat))
	}
	return acts
}

func (l *Leases) passDeadline(ls *lease) func() {
	action, stop := "recorded", func() {}
	if l.cfg.Stage == StageEnforce {
		action, stop, ls.stopped = "stopped", ls.stop, true
	}
	lane := ls.lane
	return func() {
		stop()
		l.publish.emit("Leases.fire", CodeLaneDeadline, signalcenter.SeverityWarn,
			"lane "+lane.Key+" passed its deadline of "+l.cfg.Deadline.String()+" ("+action+")",
			map[string]string{"todo": lane.Key, "stage": l.cfg.Stage, "action": action,
				"deadline_minutes": strconv.Itoa(int(l.cfg.Deadline.Minutes()))})
	}
}

func (l *Leases) reportStall(ls *lease, beat time.Time) func() {
	lane := ls.lane
	return func() {
		l.publish.emit("Leases.fire", CodeLaneStalled, signalcenter.SeverityWarn,
			"lane "+lane.Key+" has had no event since "+beat.UTC().Format(time.RFC3339),
			map[string]string{"todo": lane.Key, "stage": l.cfg.Stage, "last_heartbeat": beat.UTC().Format(time.RFC3339),
				"stall_minutes": strconv.Itoa(int(l.cfg.Stall.Minutes()))})
	}
}

func (l *Leases) heartbeat(ls *lease, now time.Time) (time.Time, bool) {
	at, ok := l.beat(ls.lane)
	if !ok {
		return now, false
	}
	if at.Before(ls.lane.Started) {
		return ls.lane.Started, true
	}
	return at, true
}

func (l *Leases) schedule(ls *lease, now, beat time.Time) {
	var next time.Time
	if !ls.late {
		next = ls.deadline
	}
	if stallAt := beat.Add(l.cfg.Stall); l.cfg.Stall > 0 && !ls.stalled && (next.IsZero() || stallAt.Before(next)) {
		next = stallAt
	}
	ls.seq++
	if !next.IsZero() {
		heap.Push(&l.due, due{at: next, key: ls.lane.Key, seq: ls.seq})
	}
	l.rearm(now)
}

func (l *Leases) rearm(now time.Time) {
	for len(l.due) > 0 {
		if ls, ok := l.live[l.due[0].key]; ok && ls.seq == l.due[0].seq {
			break
		}
		heap.Pop(&l.due)
	}
	if l.timer != nil {
		l.timer.Stop()
		l.timer = nil
	}
	if len(l.due) > 0 {
		l.timer = l.clock.AfterFunc(l.due[0].at.Sub(now), l.fire)
	}
}

type due struct {
	at  time.Time
	key string
	seq int
}

type dueHeap []due

func (h dueHeap) Len() int           { return len(h) }
func (h dueHeap) Less(i, j int) bool { return h[i].at.Before(h[j].at) }
func (h dueHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *dueHeap) Push(x any)        { *h = append(*h, x.(due)) }
func (h *dueHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}
