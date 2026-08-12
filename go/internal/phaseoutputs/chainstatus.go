package phaseoutputs

// chainstatus.go — the totalization of "did this cycle produce a reasoning
// chain", with each state carrying exactly one meaning.
//
// The flaw this fixes was live: the chain-compliance dataset recorded "absent"
// both when the auditor ran and did not comply AND when the audit phase never
// ran at all. A compliance rate over that column has an unverified denominator
// — the same defect as a disagreement column that cannot move, arriving from
// the other direction. The states are a typed enum rather than strings so a
// consumer cannot invent a new meaning, and denominator membership lives
// beside the states so no call site re-derives it.

// ShadowView is the slice of the audit-chain shadow record this package needs.
// Declared here (interface-segregation shape, applied to data): depending on
// all of auditchain for one boolean would couple the survey layer to the whole
// chain vocabulary. The `chain_present` tag is pinned against the producer by
// TestShadowView_DecodesTheAuditchainWireTag — a silent rename upstream would
// otherwise decode every audited cycle as non-compliant.
type ShadowView struct {
	ChainPresent bool `json:"chain_present"`
}

// RecordReading is what the caller's read of the shadow record produced. A
// read has THREE outcomes — absent, unparseable, parsed — and collapsing the
// middle one into either neighbour re-creates the conflation this file exists
// to end (an existing-but-truncated record is an instrumentation defect, not
// a missing one; review finding).
type RecordReading struct {
	View *ShadowView
	// Corrupt marks a record that exists on disk but did not parse.
	Corrupt bool
}

// ChainStatus is one cycle's chain outcome.
type ChainStatus string

const (
	// ChainAuditNotRun — the audit phase never executed; there was nothing to
	// comply with. Out of the denominator.
	ChainAuditNotRun ChainStatus = "audit-not-run"
	// ChainRecordMissing — the audit ran but no shadow record exists: an
	// instrumentation gap, distinct from non-compliance. In the denominator
	// (the audit happened) and separately actionable (the recorder failed).
	ChainRecordMissing ChainStatus = "record-missing"
	// ChainRecordCorrupt — the audit ran and a record exists but does not
	// parse (e.g. a truncated best-effort write). In the denominator, and
	// actionable at the recorder — NOT the same fact as no record at all.
	ChainRecordCorrupt ChainStatus = "record-corrupt"
	// ChainAbsent — the audit ran, the record exists, and the auditor emitted
	// no chain. THE non-compliance signal.
	ChainAbsent ChainStatus = "chain-absent"
	// ChainPresent — the auditor emitted a chain.
	ChainPresent ChainStatus = "chain-present"
	// ChainInconsistent — a shadow record (parseable or not) exists for a
	// cycle whose audit never ran: a pipeline impossibility named loudly
	// instead of classified quietly into one of the honest states.
	ChainInconsistent ChainStatus = "inconsistent"
)

// CycleChainStatus decides one cycle's status from what the caller read.
func CycleChainStatus(auditRan bool, reading RecordReading) ChainStatus {
	switch {
	case !auditRan && (reading.View != nil || reading.Corrupt):
		return ChainInconsistent
	case !auditRan:
		return ChainAuditNotRun
	case reading.Corrupt:
		return ChainRecordCorrupt
	case reading.View == nil:
		return ChainRecordMissing
	case reading.View.ChainPresent:
		return ChainPresent
	default:
		return ChainAbsent
	}
}

// InDenominator reports whether this status counts toward the compliance
// rate's denominator: only cycles whose audit actually ran can be asked
// whether the auditor complied.
func (s ChainStatus) InDenominator() bool {
	switch s {
	case ChainPresent, ChainAbsent, ChainRecordMissing, ChainRecordCorrupt:
		return true
	}
	return false
}
