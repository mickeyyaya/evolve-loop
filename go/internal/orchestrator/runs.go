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

func signalSpan(dir string) (first, last time.Time, ok bool) {
	chunk, err := signalcenter.ReadStream(filepath.Join(dir, signalcenter.StreamFileName), 0)
	if err != nil {
		return first, last, false
	}
	for _, e := range chunk.Events {
		at, _ := time.Parse(time.RFC3339Nano, e.TS)
		if first.IsZero() || at.Before(first) {
			first = at
		}
		if at.After(last) {
			last = at
		}
	}
	return first, last, !first.IsZero()
}

func LaneTimes(runsDir string, last int) ([]time.Duration, error) {
	cycles, err := cycleDirs(runsDir)
	if err != nil {
		return nil, err
	}
	var out []time.Duration
	for _, c := range cycles[max(0, len(cycles)-last):] {
		if first, end, ok := signalSpan(cycleDir(runsDir, c)); ok && end.After(first) {
			out = append(out, end.Sub(first))
		}
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

func freshest(dir string) time.Time {
	_, at, _ := signalSpan(dir)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !slices.ContainsFunc(phaseLiveSuffixes, func(s string) bool { return strings.HasSuffix(e.Name(), s) }) {
			continue
		}
		if info, err := e.Info(); err == nil && info.ModTime().After(at) {
			at = info.ModTime()
		}
	}
	return at
}

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
		if info, err := os.Stat(dir); err != nil || info.ModTime().Before(lane.Started) {
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
	found := map[string]int{}
	last := map[string]int{}
	return func(lane Lane) (int, bool) {
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
