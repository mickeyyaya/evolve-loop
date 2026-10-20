package gcpolicy

const (
	defaultGuardsLogMaxMB   = 10
	defaultVerdictCacheKeep = 50
	defaultLedgerSealKeep   = 20000
)

func (p Policy) GuardsLogMaxBytes() int64 {
	return int64(positiveOr(p.GuardsLogMaxMB, defaultGuardsLogMaxMB)) * bytesPerMB
}

func (p Policy) VerdictCacheKeep() int {
	return positiveOr(p.VerdictCacheKeepEntries, defaultVerdictCacheKeep)
}

func (p Policy) LedgerSealKeep() int { return positiveOr(p.LedgerSealKeepLines, defaultLedgerSealKeep) }

func (p Policy) LedgerSealEnforced() bool { return p.LedgerSeal == "enforce" }

func positiveOr(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}
