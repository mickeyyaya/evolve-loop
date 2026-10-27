package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/filter"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/reader"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/wake"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const eventWireSchema = "evolve.event/1"

type eventsWatchEnv struct {
	waiter     func(output *os.File) (reader.Waiter, error)
	newContext func() (context.Context, context.CancelFunc)
	startOf    func(pid int) (string, error)
	events     policy.EventsPolicy
}

func defaultEventsWatchEnv() eventsWatchEnv {
	return eventsWatchEnv{
		waiter:     newKernelWaiter,
		newContext: watchSignalContext,
		startOf:    proctree.StartOf,
		events:     policy.EventsDefaults(),
	}
}

func newKernelWaiter(output *os.File) (reader.Waiter, error) { return wake.New(output) }

func watchSignalContext() (context.Context, context.CancelFunc) {
	signal.Notify(make(chan os.Signal, 1), syscall.SIGPIPE)
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

type eventsWatchArgs struct {
	root, channels, filter, since, until string
	asJSON                               bool
}

type eventsWatch struct {
	match, until filter.Filter
	hasUntil     bool
	asJSON       bool
	stdout       io.Writer
}

type eventWire struct {
	Schema  string `json:"schema_version"`
	ID      string `json:"id"`
	Channel string `json:"channel"`
	Cursor  *int64 `json:"cursor,omitempty"`
	channel.Record
}

func runEventsWatch(args []string, stdout, stderr io.Writer) int {
	return runEventsWatchWith(defaultEventsWatchEnv(), args, stdout, stderr)
}

func runEventsWatchWith(env eventsWatchEnv, args []string, stdout, stderr io.Writer) int {
	var a eventsWatchArgs
	values := map[string]*string{"--project-root": &a.root, "--channel": &a.channels, "--filter": &a.filter, "--since": &a.since, "--until": &a.until}
	operands, err := (cliFlags{values: values, bools: map[string]*bool{"--json": &a.asJSON}}).parse(args)
	if err == nil && (len(operands) > 0 || strings.TrimSpace(a.channels) == "") {
		err = errors.New("watch needs --channel C[,C] and no operand")
	}
	if err != nil {
		fmt.Fprintf(stderr, "evolve events watch: %v\n%s\n", err, eventsUsage)
		return cliExitUsage
	}
	src, w, rc := openEventsWatch(env, a, stdout, stderr)
	if rc != cliExitOK {
		return rc
	}
	ctx, stop := env.newContext()
	defer stop()
	return w.follow(ctx, src, stderr)
}

func openEventsWatch(env eventsWatchEnv, a eventsWatchArgs, stdout, stderr io.Writer) (eventSource, eventsWatch, int) {
	w := eventsWatch{hasUntil: strings.TrimSpace(a.until) != "", asJSON: a.asJSON, stdout: stdout}
	cat := filter.RegisteredCatalog()
	since, err := reader.ParseSince(a.since)
	if err == nil {
		w.match, err = parseWatchFilter("--filter", a.filter, cat, stderr)
	}
	if err == nil {
		w.until, err = parseWatchFilter("--until", a.until, cat, stderr)
	}
	if err != nil {
		return nil, w, eventsWatchExit(err, stderr)
	}
	root, err := loopStopRoot(a.root, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "evolve events watch: cwd: %v\n", err)
		return nil, w, cliExitIO
	}
	channels, err := eventCatalog(env.events, cat)
	if err != nil {
		fmt.Fprintf(stderr, "evolve events watch: %v\n", err)
		return nil, w, cliExitIO
	}
	output, _ := stdout.(*os.File)
	waiter, err := env.waiter(output)
	if err != nil {
		return nil, w, eventsWatchExit(err, stderr)
	}
	cfg := reader.Config{Root: paths.EventChannelsRoot(paths.EvolveDirOf(root)), Catalog: channels, Selectors: strings.Split(a.channels, ","), Since: since}
	return reader.New(cfg, reader.Ports{Waiter: waiter, StartOf: env.startOf, Now: time.Now}), w, cliExitOK
}

func parseWatchFilter(flag, expr string, cat filter.Catalog, stderr io.Writer) (filter.Filter, error) {
	f, warnings, err := filter.Parse(expr, cat)
	for _, w := range warnings {
		fmt.Fprintf(stderr, "evolve events watch: %s: WARN %s\n", flag, w)
	}
	if err != nil {
		return f, fmt.Errorf("%s: %w", flag, err)
	}
	return f, nil
}

func (w eventsWatch) follow(ctx context.Context, src eventSource, stderr io.Writer) int {
	for {
		items, err := src.Next(ctx, time.Time{})
		if ctx.Err() != nil {
			return cliExitOK
		}
		if err != nil {
			return eventsWatchExit(err, stderr)
		}
		for _, it := range items {
			done, rc := w.emit(it, stderr)
			if done || rc != cliExitOK {
				return rc
			}
		}
	}
}

func (w eventsWatch) emit(it reader.Item, stderr io.Writer) (bool, int) {
	rec := filter.Record{Source: it.Record.Source, Signal: it.Record.Signal}
	if !w.match.Match(rec) {
		return false, cliExitOK
	}
	line, err := w.line(it)
	if err != nil {
		fmt.Fprintf(stderr, "evolve events watch: encode the record at %s:%d: %v\n", it.Channel, it.Record.Cursor, err)
		return false, cliExitIO
	}
	if _, err := io.WriteString(w.stdout, line+"\n"); err != nil {
		fmt.Fprintf(stderr, "evolve events watch: the reader of the output is gone: %v\n", err)
		return false, cliExitRefused
	}
	return w.hasUntil && w.until.Until(rec), cliExitOK
}

func (w eventsWatch) line(it reader.Item) (string, error) {
	if w.asJSON {
		raw, err := json.Marshal(wireOf(it))
		return string(raw), err
	}
	head := fmt.Sprintf("%s %d ", it.Channel, it.Record.Cursor)
	switch {
	case it.Record.Signal != nil:
		return head + signalcenter.FormatLine(*it.Record.Signal), nil
	case it.Record.Gap != nil:
		g := it.Record.Gap
		return head + fmt.Sprintf("gap %s %s pid=%d seq=%d-%d dropped=%d from=%d to=%d", g.Reason, g.Severity, g.PID, g.FirstSeq, g.LastSeq, g.Dropped, g.From, g.To), nil
	}
	return head + "message " + string(it.Record.Message), nil
}

func wireOf(it reader.Item) eventWire {
	w := eventWire{Schema: eventWireSchema, ID: it.ID(), Channel: it.Channel, Record: it.Record}
	if it.Record.Source != channel.SourceWatch {
		cursor := it.Record.Cursor
		w.Cursor = &cursor
	}
	return w
}

func eventsWatchExit(err error, stderr io.Writer) int {
	fmt.Fprintf(stderr, "evolve events watch: %v\n", err)
	switch {
	case errors.Is(err, filter.ErrUsage):
		return cliExitUsage
	case errors.Is(err, filter.ErrRefused):
		return cliExitInput
	case errors.Is(err, wake.ErrRefused), errors.Is(err, reader.ErrHangup):
		return cliExitRefused
	}
	return cliExitIO
}
