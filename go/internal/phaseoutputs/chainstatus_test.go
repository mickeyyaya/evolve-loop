package phaseoutputs

// chainstatus_test.go — the totalization that fixes a real measurement flaw:
// the chain-compliance dataset's "absent" conflated "the auditor ran and did
// not comply" with "the audit never ran". With 1 present / 3 absent in the
// first wave, the reported 25% was only accidentally right — the denominator
// was unverified. A rate whose denominator is not what it claims is the D1
// finding again (a measurement that cannot move is not a measurement), from
// the other direction.
//
// Pure: the caller supplies what it read; this decides what it means.

import (
	"encoding/json"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditchain"
)

func TestCycleChainStatus_HasExactlyOneMeaningPerState(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		auditRan bool
		reading  RecordReading
		want     ChainStatus
	}{
		{"audit never ran: out of the denominator entirely", false, RecordReading{}, ChainAuditNotRun},
		{"audit ran but wrote no record: instrumentation gap, not non-compliance", true, RecordReading{}, ChainRecordMissing},
		{"audit ran, record unparseable: a THIRD fact, not a missing record", true, RecordReading{Corrupt: true}, ChainRecordCorrupt},
		{"audit ran, record says no chain: the auditor did not comply", true, RecordReading{View: &ShadowView{ChainPresent: false}}, ChainAbsent},
		{"audit ran, chain present: compliant", true, RecordReading{View: &ShadowView{ChainPresent: true}}, ChainPresent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := CycleChainStatus(tc.auditRan, tc.reading); got != tc.want {
				t.Errorf("CycleChainStatus(%v, %+v) = %s, want %s", tc.auditRan, tc.reading, got, tc.want)
			}
		})
	}
	// A record without an audit run is a pipeline impossibility worth naming
	// loudly rather than classifying quietly — parseable or not.
	if got := CycleChainStatus(false, RecordReading{View: &ShadowView{ChainPresent: true}}); got != ChainInconsistent {
		t.Errorf("a shadow record from a cycle whose audit never ran must read as INCONSISTENT, got %s", got)
	}
	if got := CycleChainStatus(false, RecordReading{Corrupt: true}); got != ChainInconsistent {
		t.Errorf("a corrupt record from a cycle whose audit never ran must read as INCONSISTENT, got %s", got)
	}
}

// InDenominator is the one question the compliance rate asks, so it lives
// beside the states rather than being re-derived at each call site.
func TestChainStatus_DenominatorMembership(t *testing.T) {
	t.Parallel()
	in := []ChainStatus{ChainPresent, ChainAbsent, ChainRecordMissing, ChainRecordCorrupt}
	out := []ChainStatus{ChainAuditNotRun, ChainInconsistent}
	for _, s := range in {
		if !s.InDenominator() {
			t.Errorf("%s belongs in the compliance denominator (the audit ran)", s)
		}
	}
	for _, s := range out {
		if s.InDenominator() {
			t.Errorf("%s must not inflate the denominator", s)
		}
	}
}

// TestShadowView_DecodesTheAuditchainWireTag pins the cross-package contract:
// ShadowView deliberately re-declares auditchain's `chain_present` tag (a
// narrow view instead of a whole-package dependency), which means a rename in
// auditchain would silently decode to false here and every audited cycle would
// read as non-compliant. This test marshals the REAL producer type and decodes
// it through the view, so that rename breaks a test instead of a wave.
func TestShadowView_DecodesTheAuditchainWireTag(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(auditchain.ShadowRecord{ChainPresent: true})
	if err != nil {
		t.Fatal(err)
	}
	var v ShadowView
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	if !v.ChainPresent {
		t.Error("ShadowView did not decode auditchain.ShadowRecord's chain_present tag — the view has drifted from the producer")
	}
}
