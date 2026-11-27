package bridge

import (
	"context"
	"os"
	"sync/atomic"
	"time"
)

type idleSource struct {
	now   func() time.Time
	after func(time.Duration) <-chan time.Time
	paths []string
}

type idleWatch struct {
	src   idleSource
	seen  int64
	last  time.Time
	fired atomic.Bool
}

func newIdleWatch(src idleSource) *idleWatch {
	w := &idleWatch{src: src}
	w.seen = w.logBytes()
	w.last = src.now()
	return w
}

func (w *idleWatch) logBytes() int64 {
	var total int64
	for _, path := range w.src.paths {
		if info, err := os.Stat(path); err == nil {
			total += info.Size()
		}
	}
	return total
}

func (w *idleWatch) sample() {
	if size := w.logBytes(); size != w.seen {
		w.seen = size
		w.last = w.src.now()
	}
}

func (w *idleWatch) watchIdle(timeout time.Duration, cancel context.CancelFunc, done <-chan struct{}) {
	wait := timeout
	for {
		select {
		case <-done:
			return
		case <-w.src.after(wait):
			w.sample()
			idle := w.src.now().Sub(w.last)
			if idle >= timeout {
				w.fired.Store(true)
				cancel()
				return
			}
			wait = timeout - idle
		}
	}
}

func headlessIdleBudget(cfg *Config, deps Deps) time.Duration {
	intervalS, maxExtends := artifactBudget(cfg, deps)
	return time.Duration(intervalS*(1+maxExtends)) * time.Second
}

func runIdleWatched(ctx context.Context, cfg *Config, deps Deps, run dispatchRun) (rc int, idled bool, err error) {
	if !cfg.StreamOutput {
		rc, err = deps.runDispatch(ctx, cfg, run)
		return rc, false, err
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	watch := newIdleWatch(idleSource{now: deps.ActivityNow, after: deps.After, paths: []string{cfg.StdoutLog, cfg.StderrLog}})
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		watch.watchIdle(headlessIdleBudget(cfg, deps), cancel, done)
	}()
	rc, err = deps.runDispatch(runCtx, cfg, run)
	close(done)
	<-finished
	return rc, watch.fired.Load(), err
}
