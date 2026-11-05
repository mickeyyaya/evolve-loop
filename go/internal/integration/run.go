package integration

import (
	"context"
	"fmt"
	"time"
)

type Run struct {
	d      *Driver
	owner  context.Context
	cancel context.CancelFunc
	req    chan struct{}
	done   chan struct{}
	err    error
}

func (d *Driver) Start(parent context.Context, interval time.Duration) *Run {
	owner, cancel := context.WithCancel(context.WithoutCancel(parent))
	r := &Run{d: d, owner: owner, cancel: cancel, req: make(chan struct{}), done: make(chan struct{})}
	go r.loop(parent, interval)
	return r
}

func (r *Run) loop(parent context.Context, interval time.Duration) {
	defer close(r.done)
	defer r.cancel()
	defer func() {
		if p := recover(); p != nil {
			r.d.fail(&stepError{stepPanic, fmt.Errorf("%v", p)})
		}
	}()
	timer := time.NewTimer(interval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			r.timerTick(parent)
			timer.Reset(interval)
			if r.d.stopped != nil {
				return
			}
		case <-r.req:
			r.err = r.d.EndWave(r.owner)
			return
		case <-r.owner.Done():
			return
		}
	}
}

func (r *Run) timerTick(parent context.Context) {
	ctx, cancel := context.WithCancel(r.owner)
	defer cancel()
	stop := context.AfterFunc(parent, cancel)
	defer stop()
	_ = r.d.Tick(ctx)
}

func (r *Run) Finish(limit time.Duration) error {
	if r == nil {
		return nil
	}
	timer := time.AfterFunc(limit, r.cancel)
	defer timer.Stop()
	select {
	case r.req <- struct{}{}:
	case <-r.done:
	case <-r.owner.Done():
	}
	<-r.done
	return r.err
}
