package wavehealth

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const maxStallLanesShown = 5

func stallsStep(s Snapshot) Step {
	c := s.Settings
	threshold := fmt.Sprintf("%d line(s) of LOOP_ORCH_LANE_STALLED within %dm is a STALL", c.StallLines, c.StallWindowMinutes)
	if !s.Stalls.Known {
		return notGathered("A5", threshold)
	}
	inWave := stallsInWave(s)
	lines := stillSilent(inWave)
	st := Step{ID: "A5", Threshold: threshold, Result: ResultOK, Reading: fmt.Sprintf("%d LOOP_ORCH_LANE_STALLED line(s) in the wave", len(inWave))}
	if recovered := len(inWave) - len(lines); recovered > 0 {
		st.Reading += fmt.Sprintf("; %d line(s) recovered", recovered)
	}
	if len(lines) > 0 {
		st.Reading += "; lanes " + strings.Join(laneNames(lines[:min(len(lines), maxStallLanesShown)]), ", ")
	}
	if denseWindow(lines, c.StallLines, time.Duration(c.StallWindowMinutes)*time.Minute) {
		st.Reading += "; STALL"
		st.Result, st.Action = ResultBrake, brakeAction
	}
	return st
}

func stallsInWave(s Snapshot) []StallLine {
	var out []StallLine
	for _, line := range s.Stalls.Value {
		if line.At.After(s.Now) || (s.Wave.Known && line.At.Before(s.Wave.Value.Start)) {
			continue
		}
		out = append(out, line)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out
}

func stillSilent(lines []StallLine) []StallLine {
	var out []StallLine
	for _, l := range lines {
		if !l.Recovered() {
			out = append(out, l)
		}
	}
	return out
}

func laneNames(lines []StallLine) []string {
	names := make([]string, len(lines))
	for i, l := range lines {
		names[i] = l.Lane
	}
	return names
}

func denseWindow(lines []StallLine, count int, window time.Duration) bool {
	if count <= 0 {
		return false
	}
	for i := 0; i+count <= len(lines); i++ {
		if lines[i+count-1].At.Sub(lines[i].At) <= window {
			return true
		}
	}
	return false
}
