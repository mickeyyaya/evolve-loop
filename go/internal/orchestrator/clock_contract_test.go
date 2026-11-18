package orchestrator

import (
	"errors"
	"sync"
	"testing"
	"time"
)

const contractTimeout = 2 * time.Second

type clockHarness struct {
	clock  Clock
	delay  time.Duration
	elapse func()
}

type clockContractCase struct {
	name string
	run  func(h clockHarness) error
}

func fakeHarness() clockHarness {
	fake := newFakeClock()
	delay := time.Hour
	return clockHarness{clock: fake, delay: delay, elapse: func() { fake.Advance(2 * delay) }}
}

func systemHarness() clockHarness {
	return clockHarness{clock: SystemClock{}, delay: time.Millisecond, elapse: func() {}}
}

type inlineClock struct{ pending []func() }

func (c *inlineClock) Now() time.Time { return epoch }

func (c *inlineClock) AfterFunc(_ time.Duration, f func()) Timer {
	c.pending = append(c.pending, f)
	return &inlineTimer{}
}

type inlineTimer struct{}

func (*inlineTimer) Stop() bool { return true }

func inlineHarness() clockHarness {
	c := &inlineClock{}
	return clockHarness{clock: c, delay: time.Hour, elapse: func() {
		for _, f := range c.pending {
			f()
		}
	}}
}

func awaitSignal(ch <-chan struct{}, what string) error {
	select {
	case <-ch:
		return nil
	case <-time.After(contractTimeout):
		return errors.New(what)
	}
}

func runsTheFunctionOnce(h clockHarness) error {
	fired := make(chan struct{}, 4)
	h.clock.AfterFunc(h.delay, func() { fired <- struct{}{} })
	h.elapse()
	signal := make(chan struct{})
	go func() { <-fired; close(signal) }()
	if err := awaitSignal(signal, "the function never ran after the delay"); err != nil {
		return err
	}
	select {
	case <-fired:
		return errors.New("the function ran more than once")
	case <-time.After(20 * time.Millisecond):
		return nil
	}
}

func stopBeforeTheDelayPreventsTheRun(h clockHarness) error {
	var mu sync.Mutex
	ran := false
	stopped := h.clock.AfterFunc(h.delay, func() { mu.Lock(); ran = true; mu.Unlock() })
	if !stopped.Stop() {
		return errors.New("Stop of a pending timer returned false")
	}
	if stopped.Stop() {
		return errors.New("a second Stop returned true")
	}
	later := make(chan struct{})
	h.clock.AfterFunc(2*h.delay, func() { close(later) })
	h.elapse()
	if err := awaitSignal(later, "the later timer never ran"); err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	if ran {
		return errors.New("a stopped timer still ran its function")
	}
	return nil
}

func stopAfterTheRunReportsFalse(h clockHarness) error {
	fired := make(chan struct{})
	timer := h.clock.AfterFunc(h.delay, func() { close(fired) })
	h.elapse()
	if err := awaitSignal(fired, "the function never ran after the delay"); err != nil {
		return err
	}
	if timer.Stop() {
		return errors.New("Stop after the function ran returned true")
	}
	return nil
}

func callbackMayArmAnotherTimer(h clockHarness) error {
	chained := make(chan struct{})
	armed := make(chan struct{})
	h.clock.AfterFunc(h.delay, func() {
		h.clock.AfterFunc(h.delay, func() { close(chained) })
		close(armed)
	})
	h.elapse()
	if err := awaitSignal(armed, "the first callback never armed the second timer"); err != nil {
		return err
	}
	h.elapse()
	return awaitSignal(chained, "a timer armed from inside a callback never ran")
}

func callbackTakesALockTheCallerHolds(h clockHarness) error {
	var held sync.Mutex
	ran := make(chan struct{})
	h.clock.AfterFunc(h.delay, func() {
		held.Lock()
		held.Unlock()
		close(ran)
	})
	held.Lock()
	returned := make(chan struct{})
	go func() { h.elapse(); close(returned) }()
	if err := awaitSignal(returned, "the clock blocked the caller while the callback waited on a lock the caller holds: the callback ran on the caller's goroutine"); err != nil {
		held.Unlock()
		return err
	}
	held.Unlock()
	return awaitSignal(ran, "the callback never ran after the caller released the lock")
}

var clockContract = []clockContractCase{
	{"runs the function once after the delay", runsTheFunctionOnce},
	{"stop before the delay prevents the run", stopBeforeTheDelayPreventsTheRun},
	{"stop after the run reports false", stopAfterTheRunReportsFalse},
	{"a callback may arm another timer", callbackMayArmAnotherTimer},
	{"callback takes a lock the caller holds", callbackTakesALockTheCallerHolds},
}

func TestClockContract(t *testing.T) {
	clocks := []struct {
		name    string
		harness func() clockHarness
	}{
		{"fake", fakeHarness},
		{"system", systemHarness},
	}
	for _, c := range clocks {
		t.Run(c.name, func(t *testing.T) {
			for _, tc := range clockContract {
				t.Run(tc.name, func(t *testing.T) {
					if err := tc.run(c.harness()); err != nil {
						t.Error(err)
					}
				})
			}
		})
	}
}

func TestClockContract_CatchesAClockThatRunsCallbacksOnTheCallersGoroutine(t *testing.T) {
	if err := callbackTakesALockTheCallerHolds(inlineHarness()); err == nil {
		t.Fatal("the lock case passed on a clock that runs callbacks inline: the contract cannot tell the old fake from time.AfterFunc")
	}
}
