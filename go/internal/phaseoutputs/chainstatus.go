package phaseoutputs

type ShadowView struct {
	ChainPresent bool `json:"chain_present"`
}

type RecordReading struct {
	View    *ShadowView
	Corrupt bool
}

type ChainStatus string

const (
	ChainAuditNotRun   ChainStatus = "audit-not-run"
	ChainRecordMissing ChainStatus = "record-missing"
	ChainRecordCorrupt ChainStatus = "record-corrupt"
	ChainAbsent        ChainStatus = "chain-absent"
	ChainPresent       ChainStatus = "chain-present"
	ChainInconsistent  ChainStatus = "inconsistent"
)

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

func (s ChainStatus) InDenominator() bool {
	switch s {
	case ChainPresent, ChainAbsent, ChainRecordMissing, ChainRecordCorrupt:
		return true
	}
	return false
}
