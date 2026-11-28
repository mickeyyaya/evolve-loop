package acsassert

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

var ErrSubprocessBound = errors.New(acsverdict.SubprocessBoundMarker)

var subprocessCap = 300 * time.Second

const goTool = "go"

const goTestTimedOut = "panic: test timed out"

func laneEnd() (time.Time, bool) {
	end, err := time.Parse(time.RFC3339Nano, os.Getenv(ipcenv.ACSLaneEndKey))
	return end, err == nil
}

func boundDeadline(extra ...time.Time) time.Time {
	deadline := time.Now().Add(subprocessCap)
	if end, ok := laneEnd(); ok {
		extra = append(extra, end)
	}
	for _, candidate := range extra {
		if candidate.Before(deadline) {
			deadline = candidate
		}
	}
	return deadline
}

func boundContext(extra ...time.Time) (context.Context, context.CancelFunc) {
	return context.WithDeadline(context.Background(), boundDeadline(extra...))
}

func innerTimeout(ctx context.Context) time.Duration {
	deadline, _ := ctx.Deadline()
	remaining := time.Until(deadline)
	return remaining - remaining/10
}

func boundHit(ctx context.Context, name, output string) bool {
	return ctx.Err() != nil || (name == goTool && strings.Contains(output, goTestTimedOut))
}

func boundLine(name string, callerEnded bool) string {
	earlier := "the earlier of the lane end and " + subprocessCap.String()
	if callerEnded {
		earlier = "the caller deadline"
	}
	return "\n" + acsverdict.SubprocessBoundMarker + ": " + name + " did not finish inside " + earlier + "\n"
}

func boundNote(ctx context.Context, callerDeadlines ...time.Time) string {
	if ctx.Err() == nil {
		return ""
	}
	deadline, _ := ctx.Deadline()
	for _, candidate := range callerDeadlines {
		if candidate.Equal(deadline) {
			return boundLine(goTool, true)
		}
	}
	return boundLine(goTool, false)
}
