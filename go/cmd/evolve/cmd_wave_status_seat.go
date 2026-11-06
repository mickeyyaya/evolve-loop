package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/panewatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const waveSeatStallWindow = 10 * time.Minute

type waveSeat struct {
	Cycle   int    `json:"cycle"`
	Phase   string `json:"phase"`
	State   string `json:"state"`
	Session string `json:"session"`
}

func collectWaveSeats(evolveDir string, now time.Time) ([]waveSeat, []string) {
	seats, warnings := []waveSeat{}, []string(nil)
	for _, run := range waveLiveRuns(evolveDir, now) {
		snaps, err := panewatch.ReadAll(run.Dir)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("seat health %s: %v", run.Dir, err))
		}
		alive, listWarnings := waveSessionsAlive(snaps)
		warnings = appendNew(warnings, listWarnings...)
		for _, s := range snaps {
			facts := panestream.SeatFacts{
				SessionAlive:      alive(s),
				RunOpen:           true,
				Busy:              s.Busy,
				DeliverableExists: waveDeliverableExists(run.Dir, s.Agent, s.StartedAt),
				SinceProgress:     now.Sub(s.ProgressAt),
				StallWindow:       waveSeatStallWindow,
			}
			seats = append(seats, waveSeat{Cycle: s.Cycle, Phase: s.Agent, State: panestream.ClassifySeat(facts).String(), Session: s.Session})
		}
	}
	return seats, warnings
}

func waveSessionsAlive(snaps []panewatch.Snapshot) (func(panewatch.Snapshot) bool, []string) {
	sockets := socketsOfSnapshots(snaps)
	act := socketActivity(sockets, io.Discard)
	var warnings []string
	for _, socket := range sockets {
		if act.unreachable[socket] && !act.serverGone[socket] {
			warnings = append(warnings, fmt.Sprintf("seat health: tmux socket %s unreachable, its seats keep their last known state: %v", socket, act.cause[socket]))
		}
	}
	return func(s panewatch.Snapshot) bool {
		if _, listed := act.drawn[s.Session]; listed {
			return true
		}
		return act.unreachable[s.Socket] && !act.serverGone[s.Socket]
	}, warnings
}

func appendNew(list []string, items ...string) []string {
	for _, item := range items {
		if !slices.Contains(list, item) {
			list = append(list, item)
		}
	}
	return list
}

func socketsOfSnapshots(snaps []panewatch.Snapshot) []string {
	seen := map[string]bool{}
	var sockets []string
	for _, socket := range snapshotSockets(snaps) {
		if socket != "" && !seen[socket] {
			seen[socket] = true
			sockets = append(sockets, socket)
		}
	}
	return sockets
}

func waveDeliverableExists(runDir, agent string, since time.Time) bool {
	info, err := os.Stat(filepath.Join(runDir, phasecontract.ArtifactFilename(agent)))
	return err == nil && !info.ModTime().Before(since)
}

func writeWaveSeatsText(w io.Writer, seats []waveSeat) {
	for _, s := range seats {
		fmt.Fprintf(w, "seat:    cycle %d %s %s (%s)\n", s.Cycle, s.Phase, s.State, s.Session)
	}
}
