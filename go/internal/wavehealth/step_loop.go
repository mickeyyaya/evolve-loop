package wavehealth

var skippedIDs = []string{"A2", "A3", "A4", "A5", "A6", "A7", "A8-disk", "A8-load", "A8-seats", "A9", "A10"}

const loopThreshold = "not running ends Part A; running with the brake held is BRAKE-HELD"

func loopStep(s Snapshot) Step {
	if !s.Loop.Known {
		return notGathered("A1", loopThreshold)
	}
	st := Step{ID: "A1", Threshold: loopThreshold, Result: ResultOK}
	switch l := s.Loop.Value; {
	case !l.Running:
		st.Reading, st.Action = "loop not running", "start a wave only at a boundary"
	case l.BrakeHeld:
		st.Reading, st.Action = "loop running, brake held (BRAKE-HELD)", "the running cycles finish; start no wave"
	default:
		st.Reading = "loop running, brake not engaged"
	}
	return st
}

func skippedSteps() []Step {
	out := make([]Step, len(skippedIDs))
	for i, id := range skippedIDs {
		out[i] = Step{ID: id, Reading: "no wave runs", Threshold: "Part A ended at A1", Result: ResultSkip}
	}
	return out
}
