package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func runChannelTail(ctx context.Context, opts tailOptions, projectRoot string, poll time.Duration, stdout, stderr io.Writer) int {
	log, err := channel.New(paths.EventChannelsRoot(paths.EvolveDirOf(projectRoot)), opts.channel, channel.Config{})
	if err != nil {
		fmt.Fprintf(stderr, "signals tail: %v\nusage: %s\n", err, signalsTailUsage)
		return exitUsage
	}
	t := &signalsTail{opts: opts, stdout: stdout, stderr: stderr}
	var next int64
	for {
		batch, err := log.Read(next)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(stderr, "signals tail: channel %s: %v\n", opts.channel, err)
			return exitIO
		}
		next = max(next, batch.Next)
		if err := t.emit(channelSignals(opts.channel, batch.Records, stderr)); err != nil {
			fmt.Fprintf(stderr, "signals tail: write: %v\n", err)
			return exitIO
		}
		if !opts.follow || !waitPoll(ctx, poll) {
			return 0
		}
	}
}

func channelSignals(name string, records []channel.Record, stderr io.Writer) []signalcenter.Event {
	var out []signalcenter.Event
	for _, r := range records {
		switch {
		case r.Signal != nil:
			out = append(out, *r.Signal)
		case r.Gap != nil:
			fmt.Fprintf(stderr, "signals tail: gap in channel %s: %s, seq %d-%d, %d dropped\n", name, r.Gap.Reason, r.Gap.FirstSeq, r.Gap.LastSeq, r.Gap.Dropped)
		}
	}
	return out
}
