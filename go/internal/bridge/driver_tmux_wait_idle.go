package bridge

const defaultIdleCheckpointGraceS = 120

func (s *replWaitState) observeIdleClock(elapsed int, captureOK, busy bool) {
	if captureOK && busy {
		s.idleSinceS = elapsed
	}
}

func (s *replWaitState) idleCheckpointDue(elapsed int) bool {
	return !s.nudgeSent && elapsed-max(s.idleSinceS, s.intervalStartS) >= s.idleGraceS
}

func (s *replWaitState) checkpointDue(elapsed int) bool {
	return elapsed-s.intervalStartS >= s.intervalS || s.idleCheckpointDue(elapsed)
}
