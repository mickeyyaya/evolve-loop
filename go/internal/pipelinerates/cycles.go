package pipelinerates

import (
	"encoding/json"
	"errors"
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

type Cycle struct {
	ID, Wave     int
	Shipped      bool
	Sealed       bool
	LaneDeadline bool
	SystemFail   bool
	Item, Commit string
}

func (c Cycle) DeadlineEnd() bool { return !c.Sealed || c.LaneDeadline }

type wave struct {
	Number     int `json:"number"`
	CycleFloor int `json:"cycle_floor"`
}

func LoadCycles(root string) ([]Cycle, error) {
	waves, err := loadWaves(filepath.Join(paths.EvolveDirOf(root), "waves"))
	if err != nil {
		return nil, err
	}
	runs := filepath.Join(paths.EvolveDirOf(root), "runs")
	ids, err := cycleIDs(runs)
	if err != nil {
		return nil, err
	}
	var out []Cycle
	for _, id := range ids {
		dir := filepath.Join(runs, "cycle-"+strconv.Itoa(id))
		if _, running := runlease.LiveOwner(dir, time.Now()); running {
			continue
		}
		if w := waveOf(waves, id); w > 0 {
			c, err := readCycle(dir, id, w)
			if err != nil {
				return nil, err
			}
			out = append(out, c)
		}
	}
	return out, nil
}

func loadWaves(dir string) ([]wave, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "wave-*.json"))
	if err == nil && len(matches) == 0 {
		err = &fs.PathError{Op: "glob", Path: dir, Err: fs.ErrNotExist}
	}
	if err != nil {
		return nil, err
	}
	var waves []wave
	for _, m := range matches {
		var w wave
		if err := readJSON(m, &w); err != nil {
			return nil, err
		}
		if w.Number > 0 {
			waves = append(waves, w)
		}
	}
	sort.Slice(waves, func(i, j int) bool { return waves[i].CycleFloor < waves[j].CycleFloor })
	return waves, nil
}

func waveOf(waves []wave, id int) int {
	found := 0
	for _, w := range waves {
		if id >= w.CycleFloor {
			found = w.Number
		}
	}
	return found
}

func cycleIDs(runs string) ([]int, error) {
	entries, err := os.ReadDir(runs)
	if err != nil {
		return nil, err
	}
	var ids []int
	for _, e := range entries {
		digits, ok := strings.CutPrefix(e.Name(), "cycle-")
		n, err := strconv.Atoi(digits)
		if e.IsDir() && ok && err == nil && n > 0 {
			ids = append(ids, n)
		}
	}
	sort.Ints(ids)
	return ids, nil
}

func readCycle(dir string, id, w int) (Cycle, error) {
	c := Cycle{ID: id, Wave: w}
	var run struct {
		Shipped bool `json:"shipped"`
	}
	var binding struct {
		Commit string `json:"commit_sha"`
	}
	var triage struct {
		TopN []struct {
			ID string `json:"id"`
		} `json:"top_n"`
	}
	for _, f := range []struct {
		name string
		into any
	}{{"run.json", &run}, {"ship-binding.json", &binding}, {"triage-decision.json", &triage}} {
		if err := readJSON(filepath.Join(dir, f.name), f.into); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return Cycle{}, err
		}
	}
	c.Shipped, c.Commit = run.Shipped, binding.Commit
	if len(triage.TopN) > 0 {
		c.Item = triage.TopN[0].ID
	}
	return withSignals(c, filepath.Join(dir, signalcenter.StreamFileName))
}

func withSignals(c Cycle, path string) (Cycle, error) {
	chunk, err := signalcenter.ReadStream(path, 0)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Cycle{}, err
	}
	for _, e := range chunk.Events {
		switch e.Kind {
		case signalcenter.KindShipLanded:
			c.Shipped = true
		case signalcenter.KindCycleSealed:
			c.Sealed = true
			c.LaneDeadline = c.LaneDeadline || strings.Contains(e.Fields["termination_reason"], "lane deadline")
		}
		c.SystemFail = c.SystemFail || systemFailCode(e.Code)
	}
	return c, nil
}

func systemFailCode(code signalcenter.Code) bool {
	s := string(code)
	return s == "ORCHESTRATOR_SYSTEM_FAILURE" || s == "LOOP_SYSTEM_FAILURE_HALT" || (strings.HasPrefix(s, "BRIDGE_EXIT_") && strings.HasSuffix(s, "_TIMEOUT"))
}

func readJSON(path string, into any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, into)
}
