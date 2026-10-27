package pipelinerates

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

func readingFor(rs []qualitypanel.Reading, window string) (qualitypanel.Reading, bool) {
	for _, r := range rs {
		if r.Window == window {
			return r, true
		}
	}
	return qualitypanel.Reading{}, false
}

func TestCostPerShip_TheSpendOfEveryCycleOfAWaveIsSharedByItsShips(t *testing.T) {
	t.Parallel()
	cycles := []Cycle{
		{ID: 1, Wave: 141, Shipped: true, Spend: cyclespend.Spend{LLMMillis: 1_800_000, FreshTokens: 100, CacheReadTokens: 1000, AuditRounds: 1}},
		{ID: 2, Wave: 141, Shipped: true, Spend: cyclespend.Spend{LLMMillis: 2_400_000, FreshTokens: 300, CacheReadTokens: 3000, AuditRounds: 1, UnmeasuredCalls: 1}},
		{ID: 3, Wave: 141, Spend: cyclespend.Spend{LLMMillis: 9_600_000, FreshTokens: 600, CacheReadTokens: 6000, AuditRounds: 4, UnmeasuredCalls: 2}},
	}

	rs := CostPerShip(cycles)

	for window, want := range map[string]struct {
		value float64
		n     int
	}{
		"wave:141": {115, 2}, "wave:141/fresh-tokens": {500, 2}, "wave:141/cache-read-tokens": {5000, 2},
		"wave:141/audit-rounds": {2, 3}, "wave:141/unmeasured-calls": {3, 3},
	} {
		r, ok := readingFor(rs, window)
		if !ok || r.Index != "PQ5" || r.Interval != (qualitypanel.Interval{Value: want.value, Lo: want.value, Hi: want.value}) || r.N != want.n || r.Method != qualitypanel.MethodCensus {
			t.Errorf("%s = %+v (found %t), want PQ5 %v at n %d", window, r, ok, want.value, want.n)
		}
	}
	if len(rs) != 5 {
		t.Errorf("readings = %d, want 5 for one wave", len(rs))
	}
}

func TestCostPerShip_AWaveWithNoShipHasNoDataNotAZeroCost(t *testing.T) {
	t.Parallel()
	rs := CostPerShip([]Cycle{{ID: 1, Wave: 7, Spend: cyclespend.Spend{LLMMillis: 60_000}}})

	r, _ := readingFor(rs, "wave:7")
	if r.N != 0 || qualitypanel.Judge(mustIndex(t, "PQ5"), r) != qualitypanel.StatusNoData {
		t.Errorf("cost reading = %+v, want n 0, which the panel judges no_data", r)
	}
}

func mustIndex(t *testing.T, id string) qualitypanel.Index {
	t.Helper()
	ix, ok := qualitypanel.Lookup(id)
	if !ok {
		t.Fatalf("index %s is not in the registry", id)
	}
	return ix
}

func TestLoadCycles_ReadsTheLLMSpendAndTheAuditRoundsOfACycle(t *testing.T) {
	t.Parallel()
	root := wavesFixture(t)
	dir := filepath.Join(root, ".evolve", "runs", "cycle-2026")
	writeJSON(t, filepath.Join(dir, "cycle-state.json"), map[string]int{"audit_dispatches": 3})
	line := `{"phase":"build","duration_ms":120000,"usage_status":"measured","tokens":{"input":2,"output":3,"cache_read":9,"cache_write":5}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "llm-calls.ndjson"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}

	cycles, err := LoadCycles(root)

	want := cyclespend.Spend{LLMMillis: 120_000, FreshTokens: 10, CacheReadTokens: 9, AuditRounds: 3}
	if got := cycleByID(cycles, 2026).Spend; err != nil || got != want {
		t.Errorf("cycle 2026 spend = %+v, %v; want %+v", got, err, want)
	}
}

func TestLoadCycles_AnUnreadableLLMCallLedgerCountsAsAnUnmeasuredCallAndKeepsTheOtherIndexes(t *testing.T) {
	t.Parallel()
	root := wavesFixture(t)
	if err := os.Mkdir(filepath.Join(root, ".evolve", "runs", "cycle-2026", "llm-calls.ndjson"), 0o755); err != nil {
		t.Fatal(err)
	}

	cycles, err := LoadCycles(root)

	if err != nil || cycleByID(cycles, 2026).Spend.UnmeasuredCalls != 1 {
		t.Errorf("LoadCycles = cycle 2026 spend %+v, %v; want the load to hold and one unmeasured call", cycleByID(cycles, 2026).Spend, err)
	}
}
