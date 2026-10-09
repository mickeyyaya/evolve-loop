package wake

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

type kernelStep struct {
	took time.Duration
	wake Wake
	err  error
}

type fakeKernel struct {
	mu        sync.Mutex
	now       time.Time
	steps     []kernelStep
	lefts     []time.Duration
	armed     []Targets
	armGone   []int
	armErr    error
	entered   chan struct{}
	unblock   chan struct{}
	cancelErr error
	closeErr  error
}

func newFakeKernel(steps ...kernelStep) *fakeKernel {
	return &fakeKernel{now: time.Unix(1_800_000_000, 0), steps: steps, entered: make(chan struct{}, 1), unblock: make(chan struct{})}
}

func (k *fakeKernel) clock() time.Time {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.now
}

func (k *fakeKernel) arm(t Targets) ([]int, error) {
	k.armed = append(k.armed, t)
	return k.armGone, k.armErr
}

func (k *fakeKernel) wait(left time.Duration) (Wake, error) {
	k.mu.Lock()
	k.lefts = append(k.lefts, left)
	if len(k.steps) == 0 {
		k.mu.Unlock()
		k.entered <- struct{}{}
		<-k.unblock
		return Wake{}, nil
	}
	step := k.steps[0]
	k.steps = k.steps[1:]
	k.now = k.now.Add(step.took)
	k.mu.Unlock()
	return step.wake, step.err
}

func (k *fakeKernel) cancel() error {
	close(k.unblock)
	return k.cancelErr
}

func (k *fakeKernel) close() error { return k.closeErr }

func (k *fakeKernel) waiter() *Waiter { return newWaiter(k, k.clock) }

func (k *fakeKernel) recordedLefts() []time.Duration {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.lefts
}

func TestWaiter_AnEINTRRetriesWithTheTimeLeft(t *testing.T) {
	t.Parallel()
	k := newFakeKernel(kernelStep{took: 300 * time.Millisecond, err: syscall.EINTR}, kernelStep{wake: Wake{Changed: true}})
	w := k.waiter()

	got, err := w.Wait(context.Background(), k.clock().Add(time.Second))

	if err != nil || !got.Changed {
		t.Errorf("Wait = %+v, %v, want Changed after the retry", got, err)
	}
	if want := []time.Duration{time.Second, 700 * time.Millisecond}; !reflect.DeepEqual(k.recordedLefts(), want) {
		t.Errorf("kernel timeouts = %v, want %v: the retry waits only for the time left", k.recordedLefts(), want)
	}
}

func TestWaiter_AnEmptyKernelReturnBeforeTheDeadlineWaitsAgain(t *testing.T) {
	t.Parallel()
	k := newFakeKernel(kernelStep{took: 200 * time.Millisecond}, kernelStep{wake: Wake{Hangup: true}})
	w := k.waiter()

	got, err := w.Wait(context.Background(), k.clock().Add(time.Second))

	if err != nil || !got.Hangup || got.Deadline {
		t.Errorf("Wait = %+v, %v, want Hangup from the second kernel call", got, err)
	}
	if want := []time.Duration{time.Second, 800 * time.Millisecond}; !reflect.DeepEqual(k.recordedLefts(), want) {
		t.Errorf("kernel timeouts = %v, want %v", k.recordedLefts(), want)
	}
}

func TestWaiter_AKernelTimeoutAtTheDeadlineReturnsDeadline(t *testing.T) {
	t.Parallel()
	k := newFakeKernel(kernelStep{took: time.Second})
	w := k.waiter()

	got, err := w.Wait(context.Background(), k.clock().Add(time.Second))

	if err != nil || !reflect.DeepEqual(got, Wake{Deadline: true}) || len(k.recordedLefts()) != 1 {
		t.Errorf("Wait = %+v, %v after %d kernel calls, want only Deadline after one call", got, err, len(k.recordedLefts()))
	}
}

func TestWaiter_APastDeadlineReturnsWithNoKernelCall(t *testing.T) {
	t.Parallel()
	k := newFakeKernel()
	w := k.waiter()

	got, err := w.Wait(context.Background(), k.clock())

	if err != nil || !got.Deadline || len(k.recordedLefts()) != 0 {
		t.Errorf("Wait at the deadline = %+v, %v after %d kernel calls, want Deadline and no call", got, err, len(k.recordedLefts()))
	}
}

func TestWaiter_NoDeadlineWaitsWithoutATimeout(t *testing.T) {
	t.Parallel()
	k := newFakeKernel(kernelStep{took: time.Hour, wake: Wake{Exited: []int{4242}}})
	w := k.waiter()

	got, err := w.Wait(context.Background(), time.Time{})

	if err != nil || !reflect.DeepEqual(got.Exited, []int{4242}) {
		t.Errorf("Wait = %+v, %v, want the exit of 4242", got, err)
	}
	if want := []time.Duration{noTimeout}; !reflect.DeepEqual(k.recordedLefts(), want) {
		t.Errorf("kernel timeouts = %v, want %v: a wait with no deadline has no timeout", k.recordedLefts(), want)
	}
}

func TestWaiter_ACancelDuringTheKernelWaitReturnsTheContextError(t *testing.T) {
	t.Parallel()
	k := newFakeKernel()
	w := k.waiter()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-k.entered
		cancel()
	}()

	got, err := w.Wait(ctx, time.Time{})

	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, Wake{}) || len(k.recordedLefts()) != 1 {
		t.Errorf("Wait = %+v, %v after %d kernel calls, want context.Canceled from the one blocked call", got, err, len(k.recordedLefts()))
	}
}

func TestWaiter_AFailedCancelTriggerIsReported(t *testing.T) {
	t.Parallel()
	k := newFakeKernel()
	k.cancelErr = syscall.EBADF
	w := k.waiter()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-k.entered
		cancel()
	}()

	_, err := w.Wait(ctx, time.Time{})

	if !errors.Is(err, context.Canceled) || !errors.Is(err, syscall.EBADF) {
		t.Errorf("Wait = %v, want context.Canceled joined with the EBADF of the trigger", err)
	}
}

func TestWaiter_ACanceledContextReturnsBeforeTheKernel(t *testing.T) {
	t.Parallel()
	k := newFakeKernel(kernelStep{wake: Wake{Changed: true}})
	w := k.waiter()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := w.Wait(ctx, time.Time{})

	if !errors.Is(err, context.Canceled) || got.Changed || len(k.recordedLefts()) != 0 {
		t.Errorf("Wait = %+v, %v after %d kernel calls, want context.Canceled and no call", got, err, len(k.recordedLefts()))
	}
}

func TestWaiter_AKernelErrorIsReturned(t *testing.T) {
	t.Parallel()
	k := newFakeKernel(kernelStep{err: syscall.EBADF}, kernelStep{wake: Wake{Changed: true}})
	w := k.waiter()

	got, err := w.Wait(context.Background(), time.Time{})

	if !errors.Is(err, syscall.EBADF) || got.Changed {
		t.Errorf("Wait = %+v, %v, want EBADF and no retry", got, err)
	}
}

func TestWaiter_APidGoneAtTheArmIsTheNextWake(t *testing.T) {
	t.Parallel()
	k := newFakeKernel(kernelStep{wake: Wake{Changed: true}})
	k.armGone = []int{4242}
	w := k.waiter()
	targets := Targets{Dirs: []string{"/x"}, Pids: []int{4242}}

	armErr := w.Arm(targets)
	first, firstErr := w.Wait(context.Background(), time.Time{})
	k.armGone = nil
	second, secondErr := w.Wait(context.Background(), time.Time{})

	if armErr != nil || firstErr != nil || !reflect.DeepEqual(first, Wake{Exited: []int{4242}}) {
		t.Errorf("Arm = %v, first Wait = %+v, %v, want the exit of 4242 with no kernel call", armErr, first, firstErr)
	}
	if secondErr != nil || !second.Changed || len(second.Exited) != 0 {
		t.Errorf("second Wait = %+v, %v, want the kernel wake and no repeated exit", second, secondErr)
	}
	if !reflect.DeepEqual(k.armed, []Targets{targets}) {
		t.Errorf("armed = %+v, want %+v", k.armed, targets)
	}
}

func TestWaiter_ArmAndCloseReturnTheKernelErrors(t *testing.T) {
	t.Parallel()
	k := newFakeKernel()
	k.armErr = ErrRefused
	k.closeErr = syscall.EIO
	w := k.waiter()

	if err := w.Arm(Targets{}); !errors.Is(err, ErrRefused) {
		t.Errorf("Arm = %v, want %v", err, ErrRefused)
	}
	if err := w.Close(); !errors.Is(err, syscall.EIO) {
		t.Errorf("Close = %v, want EIO", err)
	}
}

func TestNewWaiter_RefusesASystemWithoutABackend(t *testing.T) {
	t.Parallel()
	err := refuseSystem("plan9")

	if !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), "plan9") {
		t.Errorf("refuseSystem(plan9) = %v, want %v naming plan9", err, ErrRefused)
	}
}

func TestHangupFD_ArmsAPipeOrATerminalOnly(t *testing.T) {
	t.Parallel()
	r, pipe, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer pipe.Close()
	regular, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer regular.Close()
	device, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer device.Close()
	never := func(int) bool { return false }
	always := func(int) bool { return true }
	cases := []struct {
		name     string
		output   *os.File
		terminal func(int) bool
		want     bool
	}{
		{"no output", nil, always, false},
		{"pipe", pipe, never, true},
		{"regular file", regular, always, false},
		{"device that is not a terminal", device, never, false},
		{"terminal", device, always, true},
	}
	for _, c := range cases {
		fd, armed, err := hangupFD(c.output, c.terminal)
		if err != nil || armed != c.want || (armed && fd != int(c.output.Fd())) {
			t.Errorf("hangupFD(%s) = %d, %v, %v, want armed=%v on its descriptor", c.name, fd, armed, err, c.want)
		}
	}
}

func TestHangupFD_AnUnreadableOutputIsAnError(t *testing.T) {
	t.Parallel()
	f, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	_, armed, err := hangupFD(f, func(int) bool { return true })

	if err == nil || armed {
		t.Errorf("hangupFD(closed) = %v, %v, want an error and no hangup", armed, err)
	}
}

type racingKernel struct {
	mu        sync.Mutex
	cancelCtx context.CancelFunc
	entered   chan struct{}
	gate      chan struct{}
	calls     []string
}

func (k *racingKernel) record(call string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.calls = append(k.calls, call)
}

func (k *racingKernel) arm(Targets) ([]int, error) { return nil, nil }

func (k *racingKernel) wait(time.Duration) (Wake, error) {
	k.cancelCtx()
	return Wake{Changed: true}, nil
}

func (k *racingKernel) cancel() error {
	k.entered <- struct{}{}
	<-k.gate
	k.record("cancel")
	return syscall.EBADF
}

func (k *racingKernel) close() error {
	k.record("close")
	return nil
}

func TestWaiter_AWakeAtTheCancelInstantWaitsForTheTriggerBeforeItReturns(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	k := &racingKernel{cancelCtx: cancel, entered: make(chan struct{}, 1), gate: make(chan struct{})}
	w := newWaiter(k, time.Now)
	type result struct {
		wake Wake
		err  error
	}
	done := make(chan result, 1)

	go func() {
		got, err := w.Wait(ctx, time.Time{})
		done <- result{got, err}
	}()
	<-k.entered
	quiet := time.NewTimer(100 * time.Millisecond)
	defer quiet.Stop()
	var r result
	select {
	case r = <-done:
		t.Errorf("Wait = %+v, %v while its cancel trigger runs, want Wait to wait for the trigger", r.wake, r.err)
		close(k.gate)
	case <-quiet.C:
		close(k.gate)
		r = <-done
	}
	closeErr := w.Close()

	if r.err != nil || !r.wake.Changed || closeErr != nil {
		t.Errorf("Wait = %+v, %v, Close = %v, want the Changed wake and no error: the trigger error joins only a context error", r.wake, r.err, closeErr)
	}
	if want := []string{"cancel", "close"}; !reflect.DeepEqual(k.calls, want) {
		t.Errorf("kernel calls = %v, want %v: no cancel after Close", k.calls, want)
	}
}
