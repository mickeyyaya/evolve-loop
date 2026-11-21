package verifylock

import (
	"context"
	"io"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var now = time.Now

type Observer struct {
	Signals *signalcenter.Center
	Cycle   int
	RunID   string
	Source  string
}

func (o Observer) AcquireWithin(ctx context.Context, projectRoot string, wait time.Duration, warn io.Writer) (func(), error) {
	start := now()
	release, err := acquireWithin(ctx, projectRoot, wait, warn)
	o.Signals.Emit(o.waitEvent(now().Sub(start), err))
	return release, err
}

func (o Observer) waitEvent(waited time.Duration, err error) signalcenter.Event {
	outcome, severity := "acquired", signalcenter.SeverityInfo
	if err != nil {
		outcome, severity = "unserialized", signalcenter.SeverityWarn
	}
	return signalcenter.Event{
		Cycle: o.Cycle, RunID: o.RunID, Module: signalcenter.ModuleHost, Origin: "verifylock.AcquireWithin",
		Kind: signalcenter.KindLoopWarning, Code: signalcenter.CodeHostVerifyLockWait, Severity: severity,
		Reason: o.Source + " waited " + waited.Round(time.Millisecond).String() + " at the host verification lock: " + outcome,
		Fields: map[string]string{"source": o.Source, "wait_ms": strconv.FormatInt(waited.Milliseconds(), 10), "outcome": outcome},
	}
}
