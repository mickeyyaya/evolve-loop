package core

// resumeCursor is the persisted suffix's execution position. It centralizes
// first-phase replay, explicit repair scheduling, and ordinary transition-plan
// lookup without changing any branch policy.
type resumeCursor struct {
	current       Phase
	lastVerdict   string
	scheduledNext Phase
	termination   triageTermination
	first         bool
	reachedEnd    bool
}

func newResumeCursor(start Phase) resumeCursor {
	return resumeCursor{current: start, lastVerdict: VerdictPASS, first: true}
}

func (c *resumeCursor) next(o *Orchestrator, cs CycleState) (Phase, error) {
	if c.first {
		c.first = false
		return c.current, nil
	}
	if c.current == PhaseTriage {
		c.termination = o.triageTermination(cs.WorkspacePath, cs.CompletedPhases, c.lastVerdict)
		if c.termination.stop {
			return PhaseEnd, nil
		}
	}
	if c.scheduledNext != "" {
		next := c.scheduledNext
		c.scheduledNext = ""
		return next, nil
	}
	return o.resolveResumeNext(cs, c.current, c.lastVerdict)
}

func (c *resumeCursor) advance(phase Phase, verdict string) {
	c.current = phase
	c.lastVerdict = verdict
}

func (c *resumeCursor) schedule(next Phase) {
	c.scheduledNext = next
}

func (c *resumeCursor) finish() {
	c.reachedEnd = true
}
