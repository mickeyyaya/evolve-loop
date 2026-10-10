//go:build integration && (darwin || linux)

package reader

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/wake"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const kernelBound = 10 * time.Second

type hostWaiter struct {
	*wake.Waiter
	onWait func()
}

func (h *hostWaiter) Wait(ctx context.Context, deadline time.Time) (wake.Wake, error) {
	if h.onWait != nil {
		go h.onWait()
		h.onWait = nil
	}
	return h.Waiter.Wait(ctx, deadline)
}

func hostReader(t *testing.T, f *fixture, h *hostWaiter, selectors []string) *Reader {
	t.Helper()
	w, err := wake.New(nil)
	if err != nil {
		t.Fatalf("wake.New = %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })
	h.Waiter = w
	return New(Config{Root: f.root, Catalog: testCatalog(t), Selectors: selectors}, Ports{Waiter: h, StartOf: proctree.StartOf, Now: time.Now})
}

func TestReader_ARealAppendWakesTheReader(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	h := &hostWaiter{}
	r := hostReader(t, f, h, []string{"cycle"})
	log := f.log("cycle")
	appended := make(chan error, 1)
	h.onWait = func() {
		_, err := log.Append([]channel.Record{sealed(1)})
		appended <- err
	}

	items, err := r.Next(context.Background(), time.Now().Add(kernelBound))

	if appendErr := <-appended; appendErr != nil {
		t.Fatalf("append = %v", appendErr)
	}
	if err != nil || describe(items) != "cycle:cycle.sealed#1" {
		t.Errorf("Next = %s, %v, want the appended record", describe(items), err)
	}
}

func TestLastWill_ARealLoopThatDiesGetsALastWill(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := sysexec.Command(ctx, "/bin/cat")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	start, err := proctree.StartOf(pid)
	if err != nil {
		t.Fatalf("StartOf(child) = %v", err)
	}
	f := newFixture(t)
	f.append("loop", loopStarted(pid, 1, start))
	h := &hostWaiter{}
	r := hostReader(t, f, h, []string{"loop"})
	h.onWait = func() { _ = stdin.Close() }

	items, err := r.Next(context.Background(), time.Now().Add(kernelBound))
	_ = stdin.Close()
	waitErr := cmd.Wait()

	if err != nil || waitErr != nil {
		t.Fatalf("Next = %s, %v; child = %v", describe(items), err, waitErr)
	}
	wantLoss(t, items, pid, "crash", false)
}
