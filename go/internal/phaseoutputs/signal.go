package phaseoutputs

func Signal(r Result, chain ChainStatus) (details string, abnormal bool) {
	switch chain {
	case ChainAbsent, ChainRecordMissing, ChainRecordCorrupt, ChainInconsistent:
		abnormal = true
	}
	if len(r.Gaps()) > 0 {
		abnormal = true
	}
	return r.SummaryLine() + "; chain: " + string(chain), abnormal
}
