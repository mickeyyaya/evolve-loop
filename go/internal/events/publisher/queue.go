package publisher

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
)

type pushResult int

const (
	pushed pushResult = iota
	full
	closed
)

type queue struct {
	maxEvents int
	maxBytes  int
	deadline  time.Duration
	marshal   func(any) ([]byte, error)
	wake      chan struct{}
	space     chan struct{}
	stopping  chan struct{}
	done      chan struct{}
	stopOnce  sync.Once
	mu        sync.Mutex
	items     []channel.Record
	bytes     int
	closed    bool
	stopCtx   context.Context
}

func newQueue(cfg Config) *queue {
	return &queue{
		maxEvents: cfg.QueueEvents,
		maxBytes:  cfg.QueueBytes,
		deadline:  cfg.EnqueueDeadline,
		marshal:   json.Marshal,
		wake:      make(chan struct{}, 1),
		space:     make(chan struct{}, 1),
		stopping:  make(chan struct{}),
		done:      make(chan struct{}),
	}
}

func (q *queue) push(rec channel.Record) (string, bool) {
	line, err := q.marshal(rec)
	if err != nil {
		return channel.ReasonWriteError, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), q.deadline)
	defer cancel()
	for {
		switch q.tryPush(rec, len(line)) {
		case pushed:
			return "", true
		case closed:
			return channel.ReasonQueueFull, false
		}
		select {
		case <-q.space:
		case <-ctx.Done():
			return channel.ReasonQueueFull, false
		}
	}
}

func (q *queue) tryPush(rec channel.Record, size int) pushResult {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return closed
	}
	if len(q.items) >= q.maxEvents || q.bytes+size > q.maxBytes {
		return full
	}
	q.items = append(q.items, rec)
	q.bytes += size
	notify(q.wake)
	if len(q.items) < q.maxEvents && q.bytes < q.maxBytes {
		notify(q.space)
	}
	return pushed
}

func (q *queue) take() []channel.Record {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.stopCtx != nil && q.stopCtx.Err() != nil {
		return nil
	}
	batch := q.items
	q.items, q.bytes = nil, 0
	notify(q.space)
	return batch
}

func (q *queue) closeQueue() []channel.Record {
	q.mu.Lock()
	defer q.mu.Unlock()
	left := q.items
	q.items, q.bytes, q.closed = nil, 0, true
	return left
}

func (q *queue) stop(ctx context.Context) {
	q.stopOnce.Do(func() {
		q.mu.Lock()
		q.stopCtx = ctx
		q.mu.Unlock()
		close(q.stopping)
	})
}

func notify(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (r *route) start() {
	if r.queue != nil {
		go r.run()
	}
}

func (r *route) run() {
	q := r.queue
	defer close(q.done)
	for {
		select {
		case <-q.wake:
			r.flush()
		case <-q.stopping:
			for r.flush() {
			}
			r.lose(channel.ReasonQueueFull, q.closeQueue())
			return
		}
	}
}

func (r *route) flush() bool {
	batch := r.queue.take()
	if len(batch) == 0 {
		return false
	}
	r.write(batch)
	return true
}

func (r *route) stop(ctx context.Context) {
	if r.queue != nil {
		r.queue.stop(ctx)
	}
}

func (r *route) awaitWriter() {
	if r.queue != nil {
		<-r.queue.done
	}
}
