package looppreflight

import (
	"context"
	"fmt"
	"io"
	"time"
)

const defaultCycleStartWaitInterval = 15 * time.Second

type CycleStartOptions struct {
	Trim  func() error
	Guard *DiskGuard
	Wait  func(context.Context) error
	Warn  io.Writer
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
	wait := opts.Wait
	if wait == nil {
		wait = defaultCycleStartWait
	}
	for opts.Guard.Check() {
		if err := wait(ctx); err != nil {
			return err
		}
	}
	return nil
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
