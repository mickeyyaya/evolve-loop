package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func cycleDirs(runsDir string) ([]int, error) {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: read the runs directory: %w", err)
	}
	var cycles []int
	for _, e := range entries {
		if n, err := strconv.Atoi(strings.TrimPrefix(e.Name(), "cycle-")); e.IsDir() && err == nil && strings.HasPrefix(e.Name(), "cycle-") {
			cycles = append(cycles, n)
		}
	}
	slices.Sort(cycles)
	return cycles, nil
}

func cycleDir(runsDir string, cycle int) string {
	return filepath.Join(runsDir, "cycle-"+strconv.Itoa(cycle))
}

type LaneRun struct {
	Elapsed time.Duration
	Shipped bool
}

func cycleSignalSpan(dir string) (first, last time.Time, shipped, ok bool) {
	chunk, err := signalcenter.ReadStream(filepath.Join(dir, signalcenter.StreamFileName), 0)
	if err != nil {
		return first, last, false, false
	}
	for _, e := range chunk.Events {
		if e.Kind == signalcenter.KindPhaseOutcome && e.Phase == "ship" && e.Fields["verdict"] == "PASS" {
			shipped = true
		}
		at, _ := time.Parse(time.RFC3339Nano, e.TS)
		if first.IsZero() || at.Before(first) {
			first = at
		}
		if at.After(last) {
			last = at
		}
	}
	return first, last, shipped, !first.IsZero()
}

func LaneRuns(runsDir string, last int) ([]LaneRun, error) {
	cycles, err := cycleDirs(runsDir)
	if err != nil {
		return nil, err
	}
	var out []LaneRun
	for _, c := range cycles[max(0, len(cycles)-last):] {
		if first, end, shipped, ok := cycleSignalSpan(cycleDir(runsDir, c)); ok && end.After(first) {
			out = append(out, LaneRun{Elapsed: end.Sub(first), Shipped: shipped})
		}
	}
	return out, nil
}

func LaneTimes(runsDir string, last int) ([]time.Duration, error) {
	runs, err := LaneRuns(runsDir, last)
	if err != nil {
		return nil, err
	}
	out := make([]time.Duration, len(runs))
	for i, r := range runs {
		out[i] = r.Elapsed
	}
	return out, nil
}

func RunsHeartbeat(runsDir string) Heartbeat {
	var mu sync.Mutex
	found := map[string]string{}
	return func(lane Lane) (time.Time, bool) {
		mu.Lock()
		dir, ok := found[lane.Key]
		if !ok {
			dir, ok = laneCycleDir(runsDir, lane)
			if ok {
				found[lane.Key] = dir
			}
		}
		mu.Unlock()
		if !ok {
			return time.Time{}, false
		}
		return freshest(dir), true
	}
}

var phaseLiveSuffixes = []string{"-pane-watch.json", "-events.ndjson", "-stdout.log"}

func freshest(dir string) time.Time { return CycleFreshest(dir, time.Time{}) }

func CycleFreshest(dir string, until time.Time) time.Time {
	var at time.Time
	note := func(t time.Time) {
		if (until.IsZero() || !t.After(until)) && t.After(at) {
			at = t
		}
	}
	if chunk, err := signalcenter.ReadStream(filepath.Join(dir, signalcenter.StreamFileName), 0); err == nil {
		for _, e := range chunk.Events {
			t, _ := time.Parse(time.RFC3339Nano, e.TS)
			note(t)
		}
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !slices.ContainsFunc(phaseLiveSuffixes, func(s string) bool { return strings.HasSuffix(e.Name(), s) }) {
			continue
		}
		if info, err := e.Info(); err == nil {
			note(info.ModTime())
		}
	}
	return at
}

const fileClockSlack = time.Second

func laneCycleDir(runsDir string, lane Lane) (string, bool) {
	cycle, ok := LaneCycle(runsDir, lane)
	return cycleDir(runsDir, cycle), ok
}

func LaneCycle(runsDir string, lane Lane) (int, bool) {
	return laneCycleAbove(runsDir, lane, 0)
}

func laneCycleAbove(runsDir string, lane Lane, floor int) (int, bool) {
	cycles, err := cycleDirs(runsDir)
	if err != nil || len(lane.Scope) == 0 {
		return 0, false
	}
	for _, c := range slices.Backward(cycles) {
		if c <= floor {
			break
		}
		dir := cycleDir(runsDir, c)
		if info, err := os.Stat(dir); err != nil || info.ModTime().Before(lane.Started.Add(-fileClockSlack)) {
			continue
		}
		if holdsTodo(dir, lane.Scope[0]) {
			return c, true
		}
	}
	return 0, false
}

func holdsTodo(dir, todo string) bool {
	return slices.Contains(committedset.LanePin(dir), todo)
}

func RunsLocator(runsDir string) func(Lane) (int, bool) {
	var mu sync.Mutex
	found := map[string]int{}
	last := map[string]int{}
	return func(lane Lane) (int, bool) {
		mu.Lock()
		defer mu.Unlock()
		life := lane.Key + "@" + lane.Started.Format(time.RFC3339Nano)
		if cycle, ok := found[life]; ok {
			return cycle, true
		}
		cycle, ok := laneCycleAbove(runsDir, lane, last[lane.Key])
		if ok {
			found[life], last[lane.Key] = cycle, cycle
		}
		return cycle, ok
	}
}
