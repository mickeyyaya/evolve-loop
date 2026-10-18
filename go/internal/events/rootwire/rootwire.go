package rootwire

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/publisher"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func Attach(c *signalcenter.Center, role, evolveDir string, stderr io.Writer) func() {
	return attach(c, role, evolveDir, os.Getenv(ipcenv.DispatchIDKey), policy.EventsDefaults(), stderr)
}

func attach(c *signalcenter.Center, role, evolveDir, dispatch string, ev policy.EventsPolicy, stderr io.Writer) func() {
	p, err := newPublisher(role, evolveDir, dispatch, ev)
	if err != nil {
		fmt.Fprintf(stderr, "[events] WARN: role %s publishes no events: %v\n", role, err)
		return c.Flush
	}
	c.Subscribe(p.Listen)
	return closer(c.Flush, p.Close, deadlineOf(ev), role, stderr)
}

func newPublisher(role, evolveDir, dispatch string, ev policy.EventsPolicy) (*publisher.Publisher, error) {
	if !filepath.IsAbs(evolveDir) {
		return nil, fmt.Errorf("the evolve directory %q is not an absolute path", evolveDir)
	}
	return publisher.New(configOf(role, evolveDir, dispatch, ev))
}

func closer(flush func(), closePublisher func(time.Duration) int, deadline time.Duration, role string, stderr io.Writer) func() {
	var warned sync.Once
	return func() {
		flush()
		if lost := closePublisher(deadline); lost > 0 {
			warned.Do(func() {
				fmt.Fprintf(stderr, "[events] WARN: %d event write(s) of role %s were lost, and no gap record names them; docs/architecture/event-channels.md §5 lists the causes\n", lost, role)
			})
		}
	}
}

func deadlineOf(ev policy.EventsPolicy) time.Duration {
	return time.Duration(len(ev.Channels)) * lockDeadlineOf(ev)
}

func lockDeadlineOf(ev policy.EventsPolicy) time.Duration {
	return time.Duration(ev.LockDeadlineMS) * time.Millisecond
}

func configOf(role, evolveDir, dispatch string, ev policy.EventsPolicy) publisher.Config {
	channels := make([]publisher.Channel, len(ev.Channels))
	for i, ch := range ev.Channels {
		channels[i] = publisher.Channel{Name: ch.Name, Route: ch.Route, QoS: publisher.QoS(ch.QoS), SegmentBytes: int64(ch.SegmentMB) << 20}
	}
	return publisher.Config{
		Root:            paths.EventChannelsRoot(evolveDir),
		Role:            role,
		Dispatch:        dispatch,
		Channels:        channels,
		Log:             channel.Config{SegmentBytes: int64(ev.SegmentMB) << 20, LockDeadline: lockDeadlineOf(ev)},
		QueueEvents:     ev.QueueEvents,
		QueueBytes:      ev.QueueBytes,
		EnqueueDeadline: time.Duration(ev.EnqueueDeadlineMS) * time.Millisecond,
	}
}
