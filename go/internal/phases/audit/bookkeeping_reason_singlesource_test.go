package audit

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestBookkeepingClassifier_BindsRealProducers(t *testing.T) {
	t.Parallel()

	req := core.PhaseRequest{Workspace: t.TempDir()}
	ancestor := []defectEntry{{ID: "d0f3a7c1e59b246d8a0c4e6f13579bde2", Status: defectStatusOpen}}
	diags := dispositionPreflight(req, 1421, ancestor, map[string]defectEntry{})
	if len(diags) != 1 {
		t.Fatalf("dispositionPreflight minted %d diagnostics, want 1 (MISSING)", len(diags))
	}
	if !core.BookkeepingMetaAuditReason(diags[0].Message) {
		t.Errorf("preflight MISSING message not classified bookkeeping-meta: %q", diags[0].Message)
	}

	cdiags := closureClaimDiagnostics("Resolved: the cycle-1424 defect is closed.\n")
	if len(cdiags) == 0 {
		t.Fatal("closureClaimDiagnostics minted nothing for an uncited closure claim")
	}
	for _, d := range cdiags {
		if !core.BookkeepingMetaAuditReason(d.Message) {
			t.Errorf("closure-claim message not classified bookkeeping-meta: %q", d.Message)
		}
	}

	for _, narrative := range []string{core.VerdictPASS, core.VerdictWARN} {
		msg := verdictConflictMessage(narrative, []string{"defect-ledger"})
		if !core.BookkeepingConflictAuditReason(msg) {
			t.Errorf("conflict record (narrative=%s) not matched by the core classifier: %q", narrative, msg)
		}
		if core.BookkeepingMetaAuditReason(msg) {
			t.Errorf("conflict record must not double-classify as meta: %q", msg)
		}
	}
	if core.BookkeepingConflictAuditReason(verdictConflictMessage(core.VerdictFAIL, []string{"defect-ledger"})) {
		t.Error("a narrative=FAIL conflict record must not be regrade-conflict class")
	}

	reasons := []string{verdictConflictMessage(core.VerdictPASS, []string{"defect-ledger"}), diags[0].Message, cdiags[0].Message}
	if !core.BookkeepingRegradeEligible(reasons) {
		t.Errorf("real minted reason set not regrade-eligible: %v", reasons)
	}
}
