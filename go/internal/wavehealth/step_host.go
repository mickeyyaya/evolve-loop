package wavehealth

import (
	"fmt"
	"slices"
	"time"
)

const bytesPerGB = 1e9

func diskStep(s Snapshot) Step {
	floor := float64(s.Settings.DiskFloorBytes) / bytesPerGB
	threshold := fmt.Sprintf("free under %.1f GB is FIX-LOCAL", floor)
	if !s.DiskFree.Known {
		return notGathered("A8-disk", threshold)
	}
	st := Step{ID: "A8-disk", Threshold: threshold, Result: ResultOK, Reading: fmt.Sprintf("free %.1f GB, floor %.1f GB", float64(s.DiskFree.Value)/bytesPerGB, floor)}
	if s.DiskFree.Value < s.Settings.DiskFloorBytes {
		st.Result = ResultFixLocal
		st.Action = "run evolve gc; when free stays under the floor, run go clean -cache"
	}
	return st
}

func loadStep(s Snapshot) Step {
	c := s.Settings
	threshold := fmt.Sprintf("every sample of %dm at or over %.2f per core is a BRAKE", c.LoadSustainedMinutes, c.LoadPerCoreLimit)
	if !s.LoadSamples.Known {
		return notGathered("A8-load", threshold)
	}
	window := time.Duration(c.LoadSustainedMinutes) * time.Minute
	var inWindow []float64
	for _, sample := range s.LoadSamples.Value {
		if !sample.At.After(s.Now) && !sample.At.Before(s.Now.Add(-window)) {
			inWindow = append(inWindow, sample.PerCore)
		}
	}
	st := Step{ID: "A8-load", Threshold: threshold, Result: ResultOK}
	if len(inWindow) < minLoadSamples {
		st.Reading = fmt.Sprintf("%d sample(s) in %dm; %d are needed", len(inWindow), c.LoadSustainedMinutes, minLoadSamples)
		st.Result, st.Action = ResultGap, "read the host load and check again"
		return st
	}
	lowest := slices.Min(inWindow)
	st.Reading = fmt.Sprintf("%d sample(s) in %dm, lowest %.2f per core", len(inWindow), c.LoadSustainedMinutes, lowest)
	if lowest >= c.LoadPerCoreLimit {
		st.Result, st.Action = ResultBrake, brakeAction
	}
	return st
}

const minLoadSamples = 2
