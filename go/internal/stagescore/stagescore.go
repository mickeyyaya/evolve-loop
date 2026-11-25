// Package stagescore tallies shadow-stage decisions against their real outcomes.
package stagescore

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

type Decision string

const (
	WouldStop Decision = stageevidence.FieldWouldStop
	WouldSkip Decision = stageevidence.FieldWouldSkip
	WouldHold Decision = stageevidence.FieldWouldHold
)

type Outcome string

const (
	Passed Outcome = "passed"
	Red    Outcome = "red"
)

type Record struct {
	Decision   Decision
	Outcome    Outcome
	LaneRouted bool
}

type Tally struct {
	Decisions  int
	FalseStops int
	Escapes    int
	Conflicts  int
}

func Score(records []Record) (Tally, error) {
	var t Tally
	for _, r := range records {
		switch r.Decision {
		case WouldStop:
			if r.Outcome == Passed {
				t.FalseStops++
			}
		case WouldSkip:
			if r.Outcome == Red {
				t.Escapes++
			}
		case WouldHold:
			if r.LaneRouted {
				t.Conflicts++
			}
		default:
			return Tally{}, fmt.Errorf("stagescore: unknown decision %q", r.Decision)
		}
		t.Decisions++
	}
	return t, nil
}
