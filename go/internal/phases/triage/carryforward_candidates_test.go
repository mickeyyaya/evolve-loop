package triage

// carryforward_candidates_test.go — in-package apicover naming coverage for
// CarryforwardCandidatesSection (cycle 1325, mint-profile-driver-suffix's
// sibling task scout-carryforward-real-cherrypick-filter). The full
// behavioral suite (landability filtering, wiring-into-ComposePrompt proof)
// lives in go/acs/cycle1325/predicates_test.go — a separate package/directory
// the repo-wide apicover unnamed-export gate does not scan for coverage of
// THIS package's exports, so this file exists purely to name+exercise the
// symbol from within internal/phases/triage itself.

import (
	"context"
	"testing"
)

// TestCarryforwardCandidatesSection_EmptyDirIsEmpty exercises the real
// function with the fail-open inputs (empty dir/base) documented on the
// symbol itself — a minimal but real assertion, not a source-grep.
func TestCarryforwardCandidatesSection_EmptyDirIsEmpty(t *testing.T) {
	if got := CarryforwardCandidatesSection(context.Background(), "", "main"); got != "" {
		t.Errorf("CarryforwardCandidatesSection(empty dir) = %q, want \"\"", got)
	}
	if got := CarryforwardCandidatesSection(context.Background(), "/tmp", ""); got != "" {
		t.Errorf("CarryforwardCandidatesSection(empty base) = %q, want \"\"", got)
	}
}
