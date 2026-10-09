package flock

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"syscall"
	"time"
)

var ErrLockDeadline = errors.New("flock: lock deadline passed")

const (
	stateWaiting int32 = iota
	stateHanded
	stateAbandoned
)

type grant struct {
	release func()
	err     error
}

type handoff struct {
	state   atomic.Int32
	granted chan grant
}

func newHandoff() *handoff { return &handoff{granted: make(chan grant, 1)} }

func (h *handoff) offer(g grant, onSettled func()) bool {
	if !h.state.CompareAndSwap(stateWaiting, stateHanded) {
		return false
	}
	onSettled()
	h.granted <- g
	return true
}

func (h *handoff) await(expired <-chan time.Time) (grant, bool) {
	select {
	case g := <-h.granted:
		return g, true
	case <-expired:
	}
	if h.state.CompareAndSwap(stateWaiting, stateAbandoned) {
		return grant{}, false
	}
	return <-h.granted, true
}

func (h *handoff) lockFor(f *os.File, path string, onSettled func()) {
	release, err := lockFile(f, path, syscall.LOCK_EX)
	if h.offer(grant{release: release, err: err}, onSettled) {
		return
	}
	if release != nil {
		release()
	}
	onSettled()
}

var after = func(d time.Duration) (<-chan time.Time, func() bool) {
	t := time.NewTimer(d)
	return t.C, t.Stop
}

func LockWithin(path string, wait time.Duration, onSettled func()) (release func(), err error) {
	f, err := openLockFile(path)
	if err != nil {
		onSettled()
		return nil, err
	}
	h := newHandoff()
	go h.lockFor(f, path, onSettled)
	expired, stop := after(wait)
	defer stop()
	g, owned := h.await(expired)
	if !owned {
		return nil, fmt.Errorf("flock %s: %w", path, ErrLockDeadline)
	}
	return g.release, g.err
}
