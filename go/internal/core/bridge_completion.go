package core

func (r PhaseRequest) BridgeCompletion() CompletionContract {
	if r.CorrectionDirective == "" || r.Fence != nil || r.Worktree == "" {
		return ""
	}
	return CompletionWorktreeEvidence
}
