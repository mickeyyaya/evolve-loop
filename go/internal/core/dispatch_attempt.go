package core

func (cr *cycleRun) countDispatch(phase Phase, req PhaseRequest) PhaseRequest {
	cr.dispatchMu.Lock()
	defer cr.dispatchMu.Unlock()
	if cr.phaseDispatches == nil {
		cr.phaseDispatches = map[Phase]int{}
	}
	cr.phaseDispatches[phase]++
	req.Attempt = cr.phaseDispatches[phase]
	return req
}
