package triage

import (
	"context"
	"testing"
)

func TestCarryforwardCandidatesSection_EmptyDirIsEmpty(t *testing.T) {
	if got := CarryforwardCandidatesSection(context.Background(), "", "main"); got != "" {
		t.Errorf("CarryforwardCandidatesSection(empty dir) = %q, want \"\"", got)
	}
	if got := CarryforwardCandidatesSection(context.Background(), "/tmp", ""); got != "" {
		t.Errorf("CarryforwardCandidatesSection(empty base) = %q, want \"\"", got)
	}
}
