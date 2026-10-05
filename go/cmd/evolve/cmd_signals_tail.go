package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type tailEnv struct {
	ProjectRoot string
	Poll        time.Duration
}

type tailOptions struct {
	cycle  int
	kinds  map[signalcenter.Kind]bool
	codes  map[signalcenter.Code]bool
	follow bool
	json   bool
}

type tailLane struct {
	cycle  int
	path   string
	next   int64
	sealed bool
	gone   bool
	done   bool
	primed bool
}

type signalsTail struct {
	opts     tailOptions
	project  string
	root     *tailLane
	lanes    []*tailLane
	fallback bool
	stdout   io.Writer
	stderr   io.Writer
}

func runSignalsTail(ctx context.Context, args []string, env tailEnv, stdout, stderr io.Writer) int {
	opts, ok := parseTailArgs(args, stderr)
	if !ok {
		return 10
	}
	poll := env.Poll
	if poll <= 0 {
		poll = time.Second
	}
	t := &signalsTail{opts: opts, project: env.ProjectRoot, stdout: stdout, stderr: stderr}
	if opts.cycle > 0 {
		t.lanes = []*tailLane{t.lane(opts.cycle)}
	} else if opts.follow {
		t.root = &tailLane{path: filepath.Join(paths.EvolveDirOf(env.ProjectRoot), signalcenter.StreamFileName)}
	}
	for first := true; ; first = false {
		if rc, stop := t.poll(first); stop {
			return rc
		}
		if !opts.follow || t.finished() || !waitPoll(ctx, poll) {
			return 0
		}
	}
}

func parseTailArgs(args []string, stderr io.Writer) (tailOptions, bool) {
	var o tailOptions
	var kinds, codes string
	fset := flag.NewFlagSet("signals tail", flag.ContinueOnError)
	fset.SetOutput(stderr)
	fset.IntVar(&o.cycle, "cycle", 0, "tail one cycle's stream (default: the live wave)")
	fset.StringVar(&kinds, "kind", "", "comma-separated kinds to print")
	fset.StringVar(&codes, "code", "", "comma-separated codes to print")
	fset.BoolVar(&o.follow, "follow", false, "keep printing appended events until the cycle or wave seals")
	fset.BoolVar(&o.json, "json", false, "print each event as one signal/1.0 JSON line")
	if err := fset.Parse(args); err != nil {
		return o, false
	}
	cycleSet := false
	fset.Visit(func(f *flag.Flag) { cycleSet = cycleSet || f.Name == "cycle" })
	var err error
	switch {
	case fset.NArg() > 0:
		err = fmt.Errorf("unexpected argument %q", fset.Arg(0))
	case cycleSet && o.cycle < 1:
		err = fmt.Errorf("--cycle must be >= 1, got %d", o.cycle)
	}
	if err == nil {
		o.kinds, err = parseTailSet("--kind", kinds, signalcenter.Kind.Known)
	}
	if err == nil {
		o.codes, err = parseTailSet("--code", codes, signalcenter.Code.Valid)
	}
	if err != nil {
		fmt.Fprintf(stderr, "signals tail: %v\nusage: %s\n", err, signalsTailUsage)
		return o, false
	}
	return o, true
}

func parseTailSet[T ~string](name, raw string, valid func(T) bool) (map[T]bool, error) {
	if raw == "" {
		return nil, nil
	}
	set := map[T]bool{}
	for _, item := range strings.Split(raw, ",") {
		if !valid(T(item)) {
			return nil, fmt.Errorf("%s: invalid value %q", name, item)
		}
		set[T(item)] = true
	}
	return set, nil
}

func (t *signalsTail) lane(cycle int) *tailLane {
	return &tailLane{cycle: cycle, path: filepath.Join(paths.RunWorkspace(t.project, cycle), signalcenter.StreamFileName)}
}

func (t *signalsTail) runsDir() string {
	return filepath.Join(paths.EvolveDirOf(t.project), "runs")
}

func (t *signalsTail) poll(first bool) (int, bool) {
	if t.opts.cycle == 0 {
		if rc, stop := t.refreshWave(first); stop {
			return rc, true
		}
	}
	var batches [][]signalcenter.Event
	for _, l := range t.streams() {
		events, rc, stop := t.read(l, first)
		if stop {
			return rc, true
		}
		batches = append(batches, events)
	}
	if err := t.emit(signalcenter.MergeByTS(batches...)); err != nil {
		fmt.Fprintf(t.stderr, "signals tail: write: %v\n", err)
		return 2, true
	}
	return 0, false
}

func (t *signalsTail) streams() []*tailLane {
	var out []*tailLane
	if t.root != nil {
		out = append(out, t.root)
	}
	for _, l := range t.lanes {
		if !l.done {
			out = append(out, l)
		}
	}
	return out
}

func (t *signalsTail) read(l *tailLane, first bool) ([]signalcenter.Event, int, bool) {
	chunk, err := signalcenter.ReadStream(l.path, l.next)
	switch {
	case errors.Is(err, fs.ErrNotExist) && first && t.opts.cycle > 0:
		fmt.Fprintf(t.stderr, "signals tail: no signal stream for cycle %d (%s)\n", t.opts.cycle, l.path)
		return nil, 1, true
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		fmt.Fprintf(t.stderr, "signals tail: %v\n", err)
		return nil, 2, true
	}
	l.next = chunk.Next
	if l == t.root && !l.primed {
		l.primed = true
		return nil, 0, false
	}
	if chunk.Skipped > 0 {
		fmt.Fprintf(t.stderr, "signals tail: skipped %d malformed line(s) in %s\n", chunk.Skipped, l.path)
	}
	for _, e := range chunk.Events {
		l.sealed = l.sealed || e.Kind == signalcenter.KindCycleSealed
	}
	l.done = l.sealed || l.gone
	return chunk.Events, 0, false
}

func (t *signalsTail) refreshWave(first bool) (int, bool) {
	entries, err := os.ReadDir(t.runsDir())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(t.stderr, "signals tail: list %s: %v\n", t.runsDir(), err)
		return 2, true
	}
	live := liveTailCycles(t.runsDir())
	if first && len(live) == 0 {
		newest := newestTailStream(t.runsDir(), entries)
		if newest == 0 {
			fmt.Fprintf(t.stderr, "signals tail: no signal streams under %s\n", t.runsDir())
			return 0, true
		}
		fallback := t.lane(newest)
		fallback.gone, t.fallback, t.lanes = true, true, []*tailLane{fallback}
		return 0, false
	}
	if t.fallback {
		return 0, false
	}
	t.joinLiveLanes(live)
	return 0, false
}

func (t *signalsTail) joinLiveLanes(live map[int]bool) {
	tracked := map[int]bool{}
	for _, l := range t.lanes {
		tracked[l.cycle] = true
		l.gone = !live[l.cycle]
	}
	for cycle := range live {
		if !tracked[cycle] {
			t.lanes = append(t.lanes, t.lane(cycle))
		}
	}
	sort.Slice(t.lanes, func(i, j int) bool { return t.lanes[i].cycle < t.lanes[j].cycle })
}

func (t *signalsTail) finished() bool {
	for _, l := range t.lanes {
		if !l.done {
			return false
		}
	}
	return len(t.lanes) > 0
}

func (t *signalsTail) emit(events []signalcenter.Event) error {
	for _, e := range events {
		if !t.opts.matches(e) {
			continue
		}
		if err := t.write(e); err != nil {
			return err
		}
	}
	return nil
}

func (t *signalsTail) write(e signalcenter.Event) error {
	if t.opts.json {
		return json.NewEncoder(t.stdout).Encode(e)
	}
	_, err := fmt.Fprintln(t.stdout, e.TS+" "+signalcenter.FormatLine(e))
	return err
}

func (o tailOptions) matches(e signalcenter.Event) bool {
	return (len(o.kinds) == 0 || o.kinds[e.Kind]) && (len(o.codes) == 0 || o.codes[e.Code])
}

func liveTailCycles(runs string) map[int]bool {
	live := map[int]bool{}
	for _, r := range runlease.LiveRuns(runs, time.Now()) {
		if n, ok := tailCycleOf(filepath.Base(r.Dir)); ok {
			live[n] = true
		}
	}
	return live
}

func newestTailStream(runs string, entries []os.DirEntry) int {
	newest := 0
	for _, e := range entries {
		n, ok := tailCycleOf(e.Name())
		if !ok || n <= newest || !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(runs, e.Name(), signalcenter.StreamFileName)); !errors.Is(err, fs.ErrNotExist) {
			newest = n
		}
	}
	return newest
}

func tailCycleOf(name string) (int, bool) {
	digits, ok := strings.CutPrefix(name, "cycle-")
	n, err := strconv.Atoi(digits)
	return n, ok && err == nil && n >= 1 && strconv.Itoa(n) == digits
}

func waitPoll(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
