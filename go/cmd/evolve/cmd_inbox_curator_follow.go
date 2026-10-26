package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/reader"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/wake"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

type landingSource interface {
	Next(ctx context.Context, deadline time.Time) ([]reader.Item, error)
}

var curateFollowContext = func() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func landingReader(root string, since reader.Since, output *os.File) (landingSource, error) {
	waiter, err := wake.New(output)
	if err != nil {
		return nil, fmt.Errorf("arm the kernel wake: %w", err)
	}
	cfg := reader.Config{
		Root: paths.EventChannelsRoot(filepath.Join(root, ".evolve")), Catalog: []reader.Channel{{Name: "ship"}},
		Selectors: []string{"ship"}, Since: since,
	}
	return reader.New(cfg, reader.Ports{Waiter: waiter, StartOf: proctree.StartOf, Now: time.Now}), nil
}

func (r *curatorRun) follow(output *os.File, stdout io.Writer) int {
	src, err := landingReader(r.req.root, r.req.since, output)
	if err != nil {
		fmt.Fprintf(r.stderr, "inbox curate: %v\n", err)
		return 2
	}
	ctx, stop := curateFollowContext()
	defer stop()
	return r.followFrom(ctx, src, stdout)
}

func (r *curatorRun) followFrom(ctx context.Context, src landingSource, stdout io.Writer) int {
	for {
		items, err := src.Next(ctx, time.Time{})
		switch {
		case ctx.Err() != nil || errors.Is(err, reader.ErrHangup):
			return 0
		case err != nil:
			fmt.Fprintf(r.stderr, "inbox curate: read the ship channel: %v\n", err)
			return 2
		}
		for _, it := range items {
			r.curateLanding(it, stdout)
		}
	}
}

func (r *curatorRun) curateLanding(it reader.Item, stdout io.Writer) {
	changes, err := r.landedChanges(it.Record.Signal)
	if err == nil && len(changes) == 0 {
		return
	}
	report := curateReport{}
	if err == nil {
		report, err = r.curate(changes, false)
	}
	if err != nil {
		fmt.Fprintf(r.stderr, "inbox curate: WARN the landing at %s:%d: %v\n", it.Channel, it.Next, err)
		return
	}
	printCurateReport(report, r.req.asJSON, stdout, r.stderr)
}
