package looppreflight

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	defaultCycleStartWaitInterval = 15 * time.Second

	DiskPauseOutcomePaused = "paused"
	DiskPauseOutcomeGaveUp = "gave_up"
)

var ErrDiskPauseExceeded = errors.New("host disk stayed below the pause line past the maximum pause")

type DiskPauseEvent struct {
	FreePct  float64
	PausePct float64
	Path     string
	Outcome  string
}

type CycleStartOptions struct {
	Trim          func() error
	Guard         *DiskGuard
	Wait          func(context.Context) error
	Warn          io.Writer
	Now           func() time.Time
	MaxPause      time.Duration
	EmitDiskPause func(DiskPauseEvent)
}

func RunCycleStart(ctx context.Context, opts CycleStartOptions) error {
	warn := opts.Warn
	if warn == nil {
		warn = io.Discard
	}
	if opts.Trim != nil {
		if err := opts.Trim(); err != nil {
			fmt.Fprintf(warn, "[cycle-start] WARN: cache trim failed: %v\n", err)
		}
	}
	if opts.Guard == nil {
		return nil
	}
	return waitOutDiskPause(ctx, opts)
}

func waitOutDiskPause(ctx context.Context, opts CycleStartOptions) error {
	wait := opts.Wait
	if wait == nil {
		wait = defaultCycleStartWait
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	maxPause := opts.MaxPause
	if maxPause <= 0 {
		maxPause = policy.Policy{}.PreflightConfig().DiskPauseMax
	}
	deadline := now().Add(maxPause)
	announced := false
	for {
		st := opts.Guard.Measure()
		if !st.Paused {
			return nil
		}
		if !announced {
			announced = true
			emitDiskPause(opts, st, DiskPauseOutcomePaused)
		}
		if !now().Before(deadline) {
			return giveUpDiskPause(opts, st, maxPause)
		}
		waitCtx, cancel := context.WithDeadline(ctx, deadline)
		err := wait(waitCtx)
		cancel()
		if err == nil {
			continue
		}
		if ctx.Err() == nil && errors.Is(err, context.DeadlineExceeded) {
			return giveUpDiskPause(opts, st, maxPause)
		}
		return err
	}
}

func giveUpDiskPause(opts CycleStartOptions, st DiskStatus, maxPause time.Duration) error {
	emitDiskPause(opts, st, DiskPauseOutcomeGaveUp)
	return fmt.Errorf("%w: %.1f%% free at %s after %s", ErrDiskPauseExceeded, st.FreePct, st.Path, maxPause)
}

func emitDiskPause(opts CycleStartOptions, st DiskStatus, outcome string) {
	if opts.EmitDiskPause == nil {
		return
	}
	opts.EmitDiskPause(DiskPauseEvent{FreePct: st.FreePct, PausePct: st.PausePct, Path: st.Path, Outcome: outcome})
}

func defaultCycleStartWait(ctx context.Context) error {
	return waitForInterval(ctx, defaultCycleStartWaitInterval)
}

func waitForInterval(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
