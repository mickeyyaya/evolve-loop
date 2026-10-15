package phaseoutputs

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
	if got := CycleChainStatus(false, RecordReading{View: &ShadowView{ChainPresent: true}}); got != ChainInconsistent {
		t.Errorf("a shadow record from a cycle whose audit never ran must read as INCONSISTENT, got %s", got)
	}
	if got := CycleChainStatus(false, RecordReading{Corrupt: true}); got != ChainInconsistent {
		t.Errorf("a corrupt record from a cycle whose audit never ran must read as INCONSISTENT, got %s", got)
	}
}

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
