package cyclespend

import "testing"

func TestTotals_PerShippedDividesTheSpendOfEveryCycleByTheShips(t *testing.T) {
	t.Parallel()
	tot := Totals{}.Add(Spend{LLMMillis: 1_800_000, FreshTokens: 10, CacheReadTokens: 100, AuditRounds: 1}, true).
		Add(Spend{LLMMillis: 4_200_000, FreshTokens: 20, CacheReadTokens: 200, AuditRounds: 5}, false)

	per, ok := tot.PerShipped()
	rounds, roundsOK := tot.AuditRoundsPerCycle()
	rate, rateOK := tot.ConvergenceRate()

	if !ok || per != (PerShipped{LLMMinutes: 100, FreshTokens: 30, CacheReadTokens: 300}) {
		t.Errorf("PerShipped = %+v, %t; want the 2 cycles divided by 1 ship", per, ok)
	}
	if !roundsOK || rounds != 3 || !rateOK || rate != 0.5 || tot.Started != 2 || tot.Shipped != 1 {
		t.Errorf("totals %+v: rounds %v %t, rate %v %t", tot, rounds, roundsOK, rate, rateOK)
	}
}

func TestTotals_NoShipAndNoCycleHaveNoDefinedFigure(t *testing.T) {
	t.Parallel()
	failed := Totals{}.Add(Spend{LLMMillis: 1}, false)
	if _, ok := failed.PerShipped(); ok {
		t.Error("a total with no ship must have no cost for each ship")
	}
	if _, ok := (Totals{}).AuditRoundsPerCycle(); ok {
		t.Error("a total with no cycle must have no rounds for each cycle")
	}
	if _, ok := (Totals{}).ConvergenceRate(); ok {
		t.Error("a total with no cycle must have no convergence rate")
	}
}
