package core_test

import (
	"os"
	"strings"
	"testing"
)

// This test pins post-dispatch recording only: a pre-dispatch failure
// (transition resolution, Build-context sealing, the cycle-state write, the
// boundary checkpoint) legitimately returns bare, since no phase ran and
// inventing an outcome for one would be a false record.
func TestResumePostDispatchExits_RecordTheirOutcome(t *testing.T) {
	body, err := os.ReadFile("resume_execution.go")
	if err != nil {
		t.Fatalf("read resume_execution.go: %v", err)
	}
	src := string(body)

	// Each entry names a post-dispatch terminal exit and the text that proves
	// it records before returning, pinned so a future edit cannot quietly
	// drop one back to a bare return.
	for _, pin := range []struct{ what, evidence string }{
		{"the non-canonical-verdict exit", `phaseOutcomeFrom(next, resp, attempts, ferr.Error(), cs.PhaseStartedAt)`},
		{"the deliverable-review exit", `phaseOutcomeFrom(next, resp, attempts, err.Error(), cs.PhaseStartedAt)`},
		{"the stranded-successor exit (cycle-637)", `phaseOutcomeFrom(next, PhaseResponse{}, 0, noRunnerErr.Error(), "")`},
	} {
		if !strings.Contains(src, pin.evidence) {
			t.Errorf("%s no longer records a phase outcome before returning — an unrecorded terminal exit classifies the cycle FAILED_UNEXPLAINED and pages an operator with no diagnosable reason", pin.what)
		}
	}

	// The post-Build refresh exit needs a positional check, not a substring:
	// its record uses phaseErr.Error(), the same spelling as the
	// dispatch-error exit above it, so a substring pin would pass on that
	// sibling's call and never catch a regression here. Assert the record
	// sits between this exit's own wrapper and its return instead.
	wrapper := strings.Index(src, `"resume refresh Build explanation after %s: %w"`)
	if wrapper < 0 {
		t.Fatal("the post-Build refresh exit no longer exists in the form this pin tracks — re-derive the pin rather than deleting it")
	}
	tail := src[wrapper:]
	record, ret := strings.Index(tail, "o.recordPhaseOutcome("), strings.Index(tail, "return result, phaseErr")
	if record < 0 || ret < 0 || record > ret {
		t.Error("the post-Build explanation refresh exit returns without recording a phase outcome — an unrecorded terminal exit classifies the cycle FAILED_UNEXPLAINED and pages an operator with no diagnosable reason")
	}

	// The bounded-loop escape records through the shared primitive rather than
	// a second implementation, so fresh and resume cannot drift apart again.
	if !strings.Contains(src, "timingOwner.recordChokepointEscape(") {
		t.Error("resume's iteration-limit exit no longer routes through recordChokepointEscape — the fresh path's C1 escape guard (the cycle-492 fix) is not mirrored here")
	}
}
