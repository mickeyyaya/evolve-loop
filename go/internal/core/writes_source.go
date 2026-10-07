package core

func (r PhaseRequest) WritesSource() bool {
	return !r.WorktreeReadOnly || len(r.WorktreeWritablePaths) > 0
}
