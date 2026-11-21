package wavehealth

import "fmt"

const ciThreshold = "completed success is OK; completed failure is FIX-LOCAL; not completed is OK; any other reading is BRAKE"

var pendingCIStatuses = map[string]bool{"queued": true, "in_progress": true, "waiting": true, "pending": true, "requested": true}

func ciStep(s Snapshot) Step {
	if !s.CI.Known {
		return notGathered("A6", ciThreshold)
	}
	ci := s.CI.Value
	st := Step{ID: "A6", Threshold: ciThreshold, Result: ResultOK}
	switch {
	case !ci.Available:
		st.Reading, st.Result, st.Action = "main CI unavailable", ResultFixLocal, "run evolve doctor probe gh and read the result"
	case ci.Status == "completed" && ci.Conclusion == "success":
		st.Reading = "required.yml on main: completed success"
	case ci.Status == "completed" && ci.Conclusion == "failure":
		st.Reading, st.Result = "required.yml on main: completed failure", ResultFixLocal
		st.Action = fmt.Sprintf("run evolve ci classify %s --json; exit 0 means a retry is safe", ci.RunID)
	case pendingCIStatuses[ci.Status]:
		st.Reading, st.Action = "required.yml on main: "+ci.Status, "start no integration batch"
	default:
		st.Reading = fmt.Sprintf("unmapped reading: status %q conclusion %q", ci.Status, ci.Conclusion)
		st.Result, st.Action = ResultBrake, brakeAction
	}
	return st
}
