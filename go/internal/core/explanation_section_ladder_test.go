package core

import (
	"context"
	"strings"
	"testing"
)

// TestAuditContractRejection_RedispatchesTheAuditorWithTheReason is the core
// half of "a missing ## Explanation Documentation section is a correction, not
// a terminal FAIL" (cycles 1601/1603): when the deliverable reviewer rejects
// the AUDIT report with the conditional-section reason, the ladder re-dispatches
// the audit runner once with that reason as its correction directive, and the
// cycle proceeds on the approved second report. The reviewer side is proven in
// deliverable (the real Reviewer at enforce); this proves the ladder accepts an
// audit rejection like any other phase's.
func TestAuditContractRejection_RedispatchesTheAuditorWithTheReason(t *testing.T) {
	const reason = `audit deliverable failed contract: [missing_section] required section "## Explanation Documentation" is missing (the explanation-documentation contract v1 is active for this cycle: review the Build explanation document and emit the section with Status, Build status, Document, Document SHA256 and path:line Evidence)`
	runners := buildRunners(nil)
	gate := &escalationProbe{phase: string(PhaseAudit), threshold: neverDemotingThreshold, approveAfter: 1, reasonPerBlock: []string{reason}}
	o := NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 0}}, &fakeLedger{}, runners, WithReviewer(gate))
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g", DisableWorkspaceGuard: true}); err != nil {
		t.Fatalf("RunCycle must proceed after one correction: %v", err)
	}
	audit := runners[PhaseAudit].(*fakeRunner)
	if len(audit.requests) != 2 {
		t.Fatalf("audit dispatched %d time(s), want 2 (initial + the correction re-dispatch)", len(audit.requests))
	}
	if audit.requests[0].CorrectionDirective != "" {
		t.Errorf("the initial audit dispatch must carry no directive, got %q", audit.requests[0].CorrectionDirective)
	}
	got := audit.requests[1].CorrectionDirective
	if !strings.Contains(got, "REJECTED") || !strings.Contains(got, `required section "## Explanation Documentation" is missing`) || !strings.Contains(got, "path:line Evidence") {
		t.Errorf("the re-dispatch must carry the rejection and the section's contents as its directive, got %q", got)
	}
	if ship := runners[PhaseShip].(*fakeRunner); ship.calls == 0 {
		t.Error("after the corrected audit the cycle must reach ship (the section was a correction, not a terminal FAIL)")
	}
}
