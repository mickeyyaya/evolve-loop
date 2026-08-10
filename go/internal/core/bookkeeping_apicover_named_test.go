package core

// bookkeeping_apicover_named_test.go — apicover named binding for the two
// exported bookkeeping-reason classifiers (issue #433 class: a new exported
// surface needs a NAMED covering test in its OWNING package; the
// phases/audit singlesource pin exercises them cross-package, which apicover
// does not count). Semantics are pinned by bookkeeping_regrade_test.go and
// the audit-package producer pin; this test binds the exported names.

import "testing"

func TestApicoverNamed_BookkeepingReasonClassifiers(t *testing.T) {
	t.Parallel()
	if !BookkeepingMetaAuditReason("defect ledger: disposition-preflight: MISSING — x") {
		t.Error("BookkeepingMetaAuditReason rejected a canonical defect-ledger reason")
	}
	if BookkeepingMetaAuditReason("EGPS verdict FAIL: red_count=1") {
		t.Error("BookkeepingMetaAuditReason accepted a non-bookkeeping reason")
	}
	if !BookkeepingConflictAuditReason("verdict-conflict: auditor narrative=PASS but 1 deterministic gate(s) forced FAIL [defect-ledger]") {
		t.Error("BookkeepingConflictAuditReason rejected a canonical PASS-narrative conflict record")
	}
	if BookkeepingConflictAuditReason("verdict-conflict: auditor narrative=FAIL but 1 deterministic gate(s) forced FAIL [defect-ledger]") {
		t.Error("BookkeepingConflictAuditReason accepted a FAIL-narrative record")
	}
}
