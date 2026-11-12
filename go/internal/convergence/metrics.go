package convergence

import (
	"maps"
	"slices"
)

type LoopRounds struct {
	Loop   Loop    `json:"loop"`
	Runs   int     `json:"runs"`
	Median float64 `json:"median"`
	Max    int     `json:"max"`
}

type DeferredClosed struct {
	Deferred int     `json:"deferred"`
	Closed   int     `json:"closed"`
	Share    float64 `json:"share"`
}

type FalsificationDrop struct {
	Checked int     `json:"checked"`
	Refuted int     `json:"refuted"`
	Rate    float64 `json:"drop_rate"`
}

type Metrics struct {
	RoundsToLand   []LoopRounds      `json:"rounds_to_land"`
	DeferredClosed DeferredClosed    `json:"deferred_closed"`
	Falsification  FalsificationDrop `json:"falsification"`
}

func Measure(inputs []Input) Metrics {
	roundsByLoop := map[Loop][]int{}
	var dc DeferredClosed
	var fd FalsificationDrop
	for _, in := range inputs {
		roundsByLoop[in.Loop] = append(roundsByLoop[in.Loop], in.Round)
		deferred, closed := deferralsOf(in.Rounds)
		dc.Deferred += deferred
		dc.Closed += closed
		checked, refuted := falsificationsOf(in.Rounds)
		fd.Checked += checked
		fd.Refuted += refuted
	}
	dc.Share = ratio(dc.Closed, dc.Deferred)
	fd.Rate = ratio(fd.Refuted, fd.Checked)
	landed := []LoopRounds{}
	for _, loop := range slices.Sorted(maps.Keys(roundsByLoop)) {
		rounds := roundsByLoop[loop]
		slices.Sort(rounds)
		landed = append(landed, LoopRounds{Loop: loop, Runs: len(rounds), Median: median(rounds), Max: rounds[len(rounds)-1]})
	}
	return Metrics{RoundsToLand: landed, DeferredClosed: dc, Falsification: fd}
}

func deferralsOf(rounds []Judgment) (deferred, closed int) {
	closedByID := map[string]bool{}
	for _, j := range rounds {
		for _, f := range j.Findings {
			_, seen := closedByID[f.ID]
			switch {
			case f.Status == StatusDeferred && !seen:
				closedByID[f.ID] = false
			case f.Status == StatusFixed && seen:
				closedByID[f.ID] = true
			}
		}
	}
	for _, isClosed := range closedByID {
		if isClosed {
			closed++
		}
	}
	return len(closedByID), closed
}

func falsificationsOf(rounds []Judgment) (checked, refuted int) {
	latest := map[string]Falsification{}
	for _, j := range rounds {
		for _, f := range j.Findings {
			if f.Falsification != "" {
				latest[f.ID] = f.Falsification
			}
		}
	}
	for _, result := range latest {
		if result == FalsificationRefuted {
			refuted++
		}
	}
	return len(latest), refuted
}

func median(sorted []int) float64 {
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return float64(sorted[mid])
	}
	return float64(sorted[mid-1]+sorted[mid]) / 2
}

func ratio(part, whole int) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) / float64(whole)
}
