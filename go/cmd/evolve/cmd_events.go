package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/filter"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/reader"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type eventSource interface {
	Next(ctx context.Context, deadline time.Time) ([]reader.Item, error)
}

const eventsUsage = `usage: evolve events channels [--json] [--project-root P]
       evolve events kinds [--json] [--project-root P]
       evolve events watch --channel C[,C] [--filter EXPR] [--since all|new|last|CURSORS|RFC3339] [--until EXPR] [--json] [--project-root P]`

func runEvents(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	handlers := map[string]func([]string, io.Writer, io.Writer) int{
		"channels": runEventsChannels, "kinds": runEventsKinds, "watch": runEventsWatch,
	}
	if len(args) == 0 {
		fmt.Fprintln(stderr, eventsUsage)
		return cliExitUsage
	}
	run, ok := handlers[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "evolve events: unknown verb %q\n%s\n", args[0], eventsUsage)
		return cliExitUsage
	}
	return run(args[1:], stdout, stderr)
}

type eventsReadArgs struct {
	root   string
	asJSON bool
}

func parseEventsReadArgs(verb string, args []string, stderr io.Writer) (eventsReadArgs, bool) {
	var a eventsReadArgs
	operands, err := (cliFlags{values: map[string]*string{"--project-root": &a.root}, bools: map[string]*bool{"--json": &a.asJSON}}).parse(args)
	if err != nil || len(operands) > 0 {
		fmt.Fprintf(stderr, "evolve events %s: want no operand and no other flag\n%s\n", verb, eventsUsage)
		return a, false
	}
	return a, true
}

type eventChannelRow struct {
	Name      string `json:"name"`
	Route     string `json:"route"`
	QoS       string `json:"qos"`
	SegmentMB int    `json:"segment_mb"`
	End       int64  `json:"end"`
}

func runEventsChannels(args []string, stdout, stderr io.Writer) int {
	a, ok := parseEventsReadArgs("channels", args, stderr)
	if !ok {
		return cliExitUsage
	}
	root, err := loopStopRoot(a.root, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "evolve events channels: cwd: %v\n", err)
		return cliExitIO
	}
	chRoot := paths.EventChannelsRoot(paths.EvolveDirOf(root))
	rows := []eventChannelRow{}
	for _, ch := range policy.EventsDefaults().Channels {
		end, err := channelEnd(chRoot, ch.Name)
		if err != nil {
			fmt.Fprintf(stderr, "evolve events channels: %v\n", err)
			return cliExitIO
		}
		rows = append(rows, eventChannelRow{Name: ch.Name, Route: ch.Route, QoS: ch.QoS, SegmentMB: ch.SegmentMB, End: end})
	}
	if a.asJSON {
		return printVerbJSON("events channels", map[string]any{"root": chRoot, "channels": rows}, stdout, stderr)
	}
	for _, r := range rows {
		fmt.Fprintf(stdout, "%-20s %-12s end=%-10d %s\n", r.Name, r.QoS, r.End, r.Route)
	}
	return cliExitOK
}

func channelEnd(root, name string) (int64, error) {
	l, err := channel.New(root, name, channel.Config{})
	if err != nil {
		return 0, err
	}
	end, err := l.End()
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	return end, err
}

func runEventsKinds(args []string, stdout, stderr io.Writer) int {
	a, ok := parseEventsReadArgs("kinds", args, stderr)
	if !ok {
		return cliExitUsage
	}
	cat := filter.RegisteredCatalog()
	if a.asJSON {
		return printVerbJSON("events kinds", cat, stdout, stderr)
	}
	for _, m := range cat.Modules {
		fmt.Fprintf(stdout, "module %s\n", m)
	}
	for _, k := range cat.Kinds {
		fmt.Fprintf(stdout, "kind %s\n", k)
	}
	for _, c := range cat.Codes {
		fmt.Fprintf(stdout, "code %s\n", c)
	}
	return cliExitOK
}

func eventCatalog(ev policy.EventsPolicy, cat filter.Catalog) ([]reader.Channel, error) {
	out := make([]reader.Channel, 0, len(ev.Channels))
	for _, ch := range ev.Channels {
		route, _, err := filter.Parse(ch.Route, cat)
		if err != nil {
			return nil, fmt.Errorf("the route of channel %s: %w", ch.Name, err)
		}
		out = append(out, reader.Channel{Name: ch.Name, Route: route})
	}
	return out, nil
}
