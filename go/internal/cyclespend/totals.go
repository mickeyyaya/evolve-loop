package cyclespend

type Totals struct {
	Started int
	Shipped int
	Spend   Spend
}

type PerShipped struct {
	LLMMinutes      float64
	FreshTokens     float64
	CacheReadTokens float64
}

func (t Totals) Add(s Spend, shipped bool) Totals {
	t.Started++
	if shipped {
		t.Shipped++
	}
	t.Spend = t.Spend.Add(s)
	return t
}

func (t Totals) PerShipped() (PerShipped, bool) {
	if t.Shipped == 0 {
		return PerShipped{}, false
	}
	n := float64(t.Shipped)
	return PerShipped{LLMMinutes: t.Spend.LLMMinutes() / n, FreshTokens: float64(t.Spend.FreshTokens) / n, CacheReadTokens: float64(t.Spend.CacheReadTokens) / n}, true
}

func (t Totals) AuditRoundsPerCycle() (float64, bool) {
	if t.Started == 0 {
		return 0, false
	}
	return float64(t.Spend.AuditRounds) / float64(t.Started), true
}

func (t Totals) ConvergenceRate() (float64, bool) {
	if t.Started == 0 {
		return 0, false
	}
	return float64(t.Shipped) / float64(t.Started), true
}
