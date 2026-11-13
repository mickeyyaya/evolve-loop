package core

func (r PhaseRequest) WritesSource() bool {
	return r.Fence == nil || len(r.Fence.WritablePaths()) > 0 || len(r.Fence.WritableDirPaths()) > 0
}
