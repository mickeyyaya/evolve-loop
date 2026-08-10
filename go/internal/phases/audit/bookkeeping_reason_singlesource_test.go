package audit

// bookkeeping_reason_singlesource_test.go — producer↔classifier binding for
// the bookkeeping-regrade micro-cycle (ADR-0084 I2 spirit: the reader and the
// writer of a machine-graded string must be pinned against each other).
//
// core.BookkeepingRegradeEligible classifies CycleState.AuditFailReasons by
// prefix. The reasons are minted HERE (defect_ledger.go, closure_claim.go,
// audit.go's verdict-conflict record). This test feeds REAL minted
// diagnostics through the core matchers, so a prefix drift on either side —
// a reworded "defect ledger:" mint, a reanchored matcher — reds it instead
// of silently disarming the regrade (the class that made the eval
// quality-gate vacuous, #426).

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestBookkeepingClassifier_BindsRealProducers(t *testing.T) {
	t.Parallel()

	// Producer 1: disposition preflight, MISSING branch (no file in workspace).
	req := core.PhaseRequest{Workspace: t.TempDir()}
	ancestor := []defectEntry{{ID: "d0f3a7c1e59b246d8a0c4e6f13579bde2", Status: defectStatusOpen}}
	diags := dispositionPreflight(req, 1421, ancestor, map[string]defectEntry{})
	if len(diags) != 1 {
		t.Fatalf("dispositionPreflight minted %d diagnostics, want 1 (MISSING)", len(diags))
	}
	if !core.BookkeepingMetaAuditReason(diags[0].Message) {
		t.Errorf("preflight MISSING message not classified bookkeeping-meta: %q", diags[0].Message)
	}

	// Producer 2: closure-claim citation gate.
	cdiags := closureClaimDiagnostics("Resolved: the cycle-1424 defect is closed.\n")
	if len(cdiags) == 0 {
		t.Fatal("closureClaimDiagnostics minted nothing for an uncited closure claim")
	}
	for _, d := range cdiags {
		if !core.BookkeepingMetaAuditReason(d.Message) {
			t.Errorf("closure-claim message not classified bookkeeping-meta: %q", d.Message)
		}
	}

	// Producer 3: the verdict-conflict record, PASS and WARN narratives in,
	// FAIL narrative impossible by the producer guard but pinned rejected.
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

	// End-to-end eligibility over the real minted set.
	reasons := []string{verdictConflictMessage(core.VerdictPASS, []string{"defect-ledger"}), diags[0].Message, cdiags[0].Message}
	if !core.BookkeepingRegradeEligible(reasons) {
		t.Errorf("real minted reason set not regrade-eligible: %v", reasons)
	}
}
