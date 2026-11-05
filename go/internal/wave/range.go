package wave

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type ReadFailure = dossier.ReadFailure

type Range struct {
	Cycles   []Cycle
	Loop     []signalcenter.Event
	Warnings []string
	Failures []ReadFailure
}

func ReadRunRange(root string, floor, ceiling int) Range {
	cycles, warnings, failures := readCycleRange(root, floor, ceiling)
	loop, loopWarnings, loopFailure := readLoopStream(root)
	failures = appendFailure(failures, loopFailure.Path, loopFailure.Err)
	return Range{
		Cycles:   ApplyLaneDeadlines(cycles, loop),
		Loop:     loop,
		Warnings: append(warnings, loopWarnings...),
		Failures: failures,
	}
}
