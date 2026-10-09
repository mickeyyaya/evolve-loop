package wake

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

var ErrRefused = errors.New("wake: refused")

type Targets struct {
	Dirs  []string
	Files []string
	Pids  []int
}

type Wake struct {
	Changed  bool
	Exited   []int
	Hangup   bool
	Deadline bool
}

func (w Wake) woke() bool {
	return w.Changed || w.Hangup || len(w.Exited) > 0
}

type kernel interface {
	arm(Targets) ([]int, error)
	wait(left time.Duration) (Wake, error)
	cancel() error
	close() error
}

const noTimeout time.Duration = -1

type Waiter struct {
	kernel kernel
	now    func() time.Time
	gone   []int
}

func New(output *os.File) (*Waiter, error) {
	k, err := newKernel(output)
	if err != nil {
		return nil, err
	}
	return newWaiter(k, time.Now), nil
}

func newWaiter(k kernel, now func() time.Time) *Waiter {
	return &Waiter{kernel: k, now: now}
}

func (w *Waiter) Arm(t Targets) error {
	gone, err := w.kernel.arm(t)
	w.gone = append(w.gone, gone...)
	return err
}

func (w *Waiter) Wait(ctx context.Context, deadline time.Time) (Wake, error) {
	if len(w.gone) > 0 {
		gone := w.gone
		w.gone = nil
		return Wake{Exited: gone}, nil
	}
	triggered := make(chan error, 1)
	stop := context.AfterFunc(ctx, func() { triggered <- w.kernel.cancel() })
	wake, err := w.waitKernel(ctx, deadline)
	if stop() {
		return wake, err
	}
	triggerErr := <-triggered
	if err != nil && err == ctx.Err() {
		return wake, errors.Join(err, triggerErr)
	}
	return wake, err
}

func (w *Waiter) waitKernel(ctx context.Context, deadline time.Time) (Wake, error) {
	for {
		if err := ctx.Err(); err != nil {
			return Wake{}, err
		}
		left, open := w.timeLeft(deadline)
		if !open {
			return Wake{Deadline: true}, nil
		}
		wake, err := w.kernel.wait(left)
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil || wake.woke() {
			return wake, err
		}
	}
}

func (w *Waiter) timeLeft(deadline time.Time) (time.Duration, bool) {
	if deadline.IsZero() {
		return noTimeout, true
	}
	left := deadline.Sub(w.now())
	return left, left > 0
}

func (w *Waiter) Close() error {
	return w.kernel.close()
}

func refuseSystem(goos string) error {
	return fmt.Errorf("%w: %s has no kernel wake", ErrRefused, goos)
}

func refuseWatch(what string, err error) error {
	return fmt.Errorf("%w: the kernel refused the watch of %s: %w", ErrRefused, what, err)
}

func statfsError(path string, err error) error {
	return &os.PathError{Op: "statfs", Path: path, Err: err}
}

func hangupFD(output *os.File, isTerminal func(int) bool) (int, bool, error) {
	if output == nil {
		return -1, false, nil
	}
	info, err := output.Stat()
	if err != nil {
		return -1, false, err
	}
	fd := int(output.Fd())
	mode := info.Mode()
	armed := mode&os.ModeNamedPipe != 0 || (mode&os.ModeCharDevice != 0 && isTerminal(fd))
	return fd, armed, nil
}
