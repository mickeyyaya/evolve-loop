package core

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

func TestBookkeepingExportsAreNamedByAnExecutedAssertion(t *testing.T) {
	t.Parallel()
	var kind BookkeepingKind = BookkeepingDossier
	if kind == BookkeepingInboxStamp {
		t.Errorf("the kinds %q and %q must differ", kind, BookkeepingInboxStamp)
	}
	spy := &recordingFlusher{}
	var flusher BookkeepingFlusher = spy
	entry := BookkeepingEntry{Kind: kind, Message: "m", Paths: []string{"p"}}
	if err := flusher.Flush([]BookkeepingEntry{entry}); err != nil || len(spy.got) != 1 || spy.got[0].Kind != BookkeepingDossier {
		t.Errorf("Flush = %v, got %+v", err, spy.got)
	}
}
