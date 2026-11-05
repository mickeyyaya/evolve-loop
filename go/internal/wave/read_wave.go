package wave

import (
	"path/filepath"
	"sort"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type WaveWindow struct {
	Floor    int
	From, To time.Time
	Stage    string
}

type WaveRead struct {
	Cycles       []Cycle
	Ships        []int
	Loop         []signalcenter.Event
	Warnings     []string
	Failures     []ReadFailure
	ShipFailures []ReadFailure
}

func ReadWave(root string, w WaveWindow) WaveRead {
	cycles, warnings, failures := readCycleRange(root, w.Floor, 0)
	ships, shipFailures := ShipsIn(root, w.Stage, w.From, w.To)
	evolveDir := paths.EvolveDirOf(root)
	resumed, resumeErrs := core.ResumedCycleIDs(root, w.From, w.To)
	for _, err := range resumeErrs {
		failures = appendFailure(failures, filepath.Join(evolveDir, signalcenter.StreamFileName), err)
	}
	held := map[int]bool{}
	for _, c := range cycles {
		held[c.ID] = true
	}
	live, liveFailures := readLive(evolveDir)
	failures = append(failures, liveFailures...)
	for _, id := range append(append([]int{}, resumed...), ships...) {
		if held[id] {
			continue
		}
		held[id] = true
		d, dossierFailures := dossierOf(root, id)
		c, warn, bad := readCycle(root, id, live, d)
		cycles = append(cycles, c)
		warnings = append(warnings, warn...)
		failures = append(append(failures, dossierFailures...), bad...)
	}
	sort.Slice(cycles, func(i, j int) bool { return cycles[i].ID < cycles[j].ID })
	for i := range cycles {
		cycles[i].Resumed = cycles[i].ID <= w.Floor
	}
	loop, _, _ := readLoopStream(root)
	return WaveRead{Cycles: ApplyLaneDeadlines(cycles, loop), Ships: ships, Loop: loop, Warnings: warnings, Failures: failures, ShipFailures: shipFailures}
}

func dossierOf(root string, id int) (*dossier.Dossier, []ReadFailure) {
	found, failures := dossier.ReadDirChecked(dossier.CyclesDir(root), id, id)
	if len(found) == 0 {
		return nil, failures
	}
	return found[0], failures
}
