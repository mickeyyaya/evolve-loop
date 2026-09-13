package signalcenter

// flush_test.go — Flush (design §6.1, S2): an emitter that finds a drain in
// progress returns before its event is delivered; a producer on an exit path
// calls Flush to wait until the queue is empty and no drain is running.

import (
	"sync"
	"testing"
	"time"
)

func TestFlush_ReturnsOnlyAfterEveryQueuedEventIsDelivered(t *testing.T) {
	t.Parallel()
	c := New()
	entered, release := make(chan struct{}), make(chan struct{})
	var mu sync.Mutex
	delivered := 0
	c.Subscribe(func(e Event) {
		if e.Reason == "slow" {
			entered <- struct{}{}
			<-release
		}
		mu.Lock()
		delivered++
		mu.Unlock()
	})
	go c.Emit(infoEvent("slow")) // this goroutine becomes the drainer and blocks inside the listener
	<-entered
	c.Emit(infoEvent("queued while draining")) // enqueues and returns
	flushed := make(chan struct{})
	go func() { c.Flush(); close(flushed) }()
	select {
	case <-flushed:
		t.Fatal("Flush returned while an event was still queued behind a running drain")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case <-flushed:
	case <-time.After(2 * time.Second):
		t.Fatal("Flush did not return after the drain finished")
	}
	mu.Lock()
	defer mu.Unlock()
	if delivered != 2 {
		t.Errorf("both events delivered before Flush returned, got %d", delivered)
	}
}

func TestFlush_IdleAndNilCentersReturnImmediately(t *testing.T) {
	t.Parallel()
	done := make(chan struct{})
	go func() {
		New().Flush()
		var nilCenter *Center
		nilCenter.Flush()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Flush must not block when nothing is queued")
	}
}
