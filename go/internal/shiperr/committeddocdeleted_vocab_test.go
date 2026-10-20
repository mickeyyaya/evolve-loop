package shiperr

import "testing"

func TestCodeCommittedDocDeleted_Vocab(t *testing.T) {
	if got, want := string(CodeCommittedDocDeleted), "COMMITTED_DOC_DELETED"; got != want {
		t.Errorf("CodeCommittedDocDeleted = %q, want %q", got, want)
	}
	if CodeCommittedDocDeleted == CodeGitStageFailed || CodeCommittedDocDeleted == CodeRepoContractGate || CodeCommittedDocDeleted == CodeUnknown {
		t.Error("CodeCommittedDocDeleted must not alias git-stage, repo-contract or the unknown fallthrough")
	}
	if SignalCode(CodeCommittedDocDeleted) != "SHIP_COMMITTED_DOC_DELETED" {
		t.Errorf("SignalCode = %q, want SHIP_COMMITTED_DOC_DELETED", SignalCode(CodeCommittedDocDeleted))
	}
}
