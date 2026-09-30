package core

func (r PhaseRequest) BridgeCompletion() CompletionContract {
	if r.CorrectionDirective == "" || r.WorktreeReadOnly || r.Worktree == "" {
		return ""
	}
	return CompletionWorktreeEvidence
}
