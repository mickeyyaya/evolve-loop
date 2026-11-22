package main

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
	"github.com/mickeyyaya/evolve-loop/go/internal/wavehealth"
)

func stallLinesOf(root string, read wave.WaveRead, now time.Time) []wavehealth.StallLine {
	out := []wavehealth.StallLine{}
	lastEvent := map[string]time.Time{}
	for _, e := range read.Loop {
		if e.Code != orchestrator.CodeLaneStalled {
			continue
		}
		at, _ := time.Parse(time.RFC3339Nano, e.TS)
		lane := e.Fields["todo"]
		if _, seen := lastEvent[lane]; !seen {
			lastEvent[lane] = read.LaneLastEvent(root, lane, now)
		}
		out = append(out, wavehealth.StallLine{Lane: lane, At: at, LastEvent: lastEvent[lane]})
	}
	return out
}
