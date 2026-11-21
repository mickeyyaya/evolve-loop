package shipwindow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

const DefaultMaxHold = 60 * time.Minute

const (
	lostForeignToken = "the lease file is gone or names another holder"
	lostHardCap      = "the hold passed the hard cap"
)

var renameLease = os.Rename

type Ticker func(d time.Duration) (<-chan time.Time, func())

type LostError struct{ Reason string }

func (e *LostError) Error() string { return "shipwindow: lease lost: " + e.Reason }

func (e *LostError) Fields() map[string]string {
	return map[string]string{"role": "holder", "reason": e.Reason}
}

func realTicker(d time.Duration) (<-chan time.Time, func()) {
	t := time.NewTicker(d)
	return t.C, t.Stop
}

type beat struct {
	opts       Options
	acquiredAt time.Time
	lost       atomic.Pointer[LostError]
	stopOnce   sync.Once
	stopCh     chan struct{}
	exited     chan struct{}
}

func newBeat(o Options) *beat {
	return &beat{opts: o, acquiredAt: o.Now(), stopCh: make(chan struct{}), exited: make(chan struct{})}
}

func (l *Lease) startBeat(ctx context.Context, o Options) {
	l.beat = newBeat(o)
	ticks, stopTicker := l.beat.opts.Ticker(l.beat.opts.TTL / 3)
	go l.beatLoop(ctx, ticks, stopTicker)
}

func (l *Lease) beatLoop(ctx context.Context, ticks <-chan time.Time, stopTicker func()) {
	defer close(l.beat.exited)
	defer stopTicker()
	for {
		select {
		case <-ctx.Done():
			return
		case <-l.beat.stopCh:
			return
		case <-ticks:
			if !l.pulse() {
				return
			}
		}
	}
}

func (l *Lease) pulse() bool {
	if l.beat.opts.Now().Sub(l.beat.acquiredAt) >= l.beat.opts.MaxHold {
		_ = l.removeIfOwned()
		l.markLost(lostHardCap)
		return false
	}
	if rec, ok, err := readRecord(l.path); err != nil || !ok || rec.Token != l.token {
		l.markLost(lostForeignToken)
		return false
	}
	if err := l.rewriteHeartbeat(); err != nil {
		l.markLost(err.Error())
		return false
	}
	return true
}

func (l *Lease) rewriteHeartbeat() error {
	b, err := marshalRecord(record{OwnerPID: l.beat.opts.OwnerPID, HeartbeatAt: l.beat.opts.Now().UTC().Format(time.RFC3339Nano), Token: l.token})
	if err != nil {
		return fmt.Errorf("heartbeat marshal: %w", err)
	}
	tmp := l.path + ".beat." + l.token
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("heartbeat stage: %w", err)
	}
	if err := renameLease(tmp, l.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("heartbeat replace: %w", err)
	}
	return nil
}

func (l *Lease) markLost(reason string) {
	e := &LostError{Reason: reason}
	if l.beat.lost.CompareAndSwap(nil, e) && l.beat.opts.OnLost != nil {
		l.beat.opts.OnLost(e)
	}
}

func (l *Lease) stopBeat() {
	l.beat.stopOnce.Do(func() { close(l.beat.stopCh) })
	<-l.beat.exited
}

func (l *Lease) removeIfOwned() error {
	rec, ok, err := readRecord(l.path)
	if err != nil && !ok {
		return fmt.Errorf("shipwindow: release read: %w", err)
	}
	if err != nil || !ok || rec.Token != l.token {
		return nil
	}
	if rerr := os.Remove(l.path); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
		return fmt.Errorf("shipwindow: release: %w", rerr)
	}
	return nil
}

func (l *Lease) Err() error {
	if l.beat == nil {
		return nil
	}
	if e := l.beat.lost.Load(); e != nil {
		return e
	}
	return nil
}

func HeldErr(ctx context.Context) error {
	l, _ := ctx.Value(heldKey{}).(*Lease)
	if l == nil {
		return nil
	}
	return l.Err()
}
