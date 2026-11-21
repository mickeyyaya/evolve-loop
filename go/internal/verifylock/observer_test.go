package verifylock

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func lockWaitEvents(c *signalcenter.Center) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range c.Recent() {
		if e.Code == signalcenter.CodeHostVerifyLockWait {
			out = append(out, e)
		}
	}
	return out
}

func TestObserverAcquireWithin_AWaitBehindAHeldLockIsMeasuredInOneSignal(t *testing.T) {
	root := t.TempDir()
	hold, err := Acquire(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(300 * time.Millisecond)
		hold()
	}()
	center := signalcenter.New()

	release, err := Observer{Signals: center, Cycle: 7, RunID: "r7", Source: "build-floor"}.AcquireWithin(context.Background(), root, MaxWait, nil)
	if err != nil {
		t.Fatal(err)
	}
	release()

	events := lockWaitEvents(center)
	if len(events) != 1 {
		t.Fatalf("got %d lock wait signals, want exactly 1", len(events))
	}
	e := events[0]
	waitMS, perr := strconv.Atoi(e.Fields["wait_ms"])
	if perr != nil || waitMS < 250 || e.Fields["outcome"] != "acquired" || e.Fields["source"] != "build-floor" || e.Cycle != 7 || e.Module != signalcenter.ModuleHost || e.Severity != signalcenter.SeverityInfo {
		t.Errorf("event = %+v, want an INFO host signal of source build-floor, outcome acquired and wait_ms of at least 250", e)
	}
}

func TestObserverAcquireWithin_AWaitThatEndsAtTheBoundIsAWarnOfOutcomeUnserialized(t *testing.T) {
	root := t.TempDir()
	hold, err := Acquire(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer hold()
	center := signalcenter.New()

	_, err = Observer{Signals: center, Cycle: 3, Source: "acs-suite"}.AcquireWithin(context.Background(), root, 100*time.Millisecond, nil)

	events := lockWaitEvents(center)
	if err == nil || len(events) != 1 || events[0].Fields["outcome"] != "unserialized" || events[0].Severity != signalcenter.SeverityWarn {
		t.Fatalf("err = %v, events = %+v, want the bound error and one WARN signal of outcome unserialized", err, events)
	}
}

func TestObserverAcquireWithin_ANilCenterStillTakesTheLock(t *testing.T) {
	release, err := Observer{Source: "acs-suite"}.AcquireWithin(context.Background(), t.TempDir(), MaxWait, nil)
	if err != nil {
		t.Fatalf("a nil center must not stop the lock: %v", err)
	}
	release()
}
