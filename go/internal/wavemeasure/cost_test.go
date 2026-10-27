package wavemeasure_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
	"github.com/mickeyyaya/evolve-loop/go/internal/wavemeasure"
)

func cycleSpend(shipped bool, millis, fresh, cacheRead int64, unmeasured, rounds int) wavemeasure.CycleSpend {
	return wavemeasure.CycleSpend{Shipped: shipped, Spend: cyclespend.Spend{LLMMillis: millis, FreshTokens: fresh, CacheReadTokens: cacheRead, UnmeasuredCalls: unmeasured, AuditRounds: rounds}}
}

func TestCompute_TheCostOfAShippedCycleCountsTheSpendOfEveryCycleOfTheWave(t *testing.T) {
	t.Parallel()
	f := wavemeasure.Facts{Cycles: []wavemeasure.CycleSpend{
		cycleSpend(true, 1_800_000, 1_000_000, 20_000_000, 0, 1),
		cycleSpend(true, 2_400_000, 1_500_000, 25_000_000, 1, 1),
		cycleSpend(false, 9_603_300, 4_338_241, 87_431_829, 2, 4),
	}}

	c := wavemeasure.Compute(f, at(0)).Cost

	if c.Started != 3 || c.Shipped != 2 {
		t.Fatalf("cycles = %d started, %d shipped; want 3 and 2", c.Started, c.Shipped)
	}
	if c.LLMMinutesPerShippedCycle == nil || *c.LLMMinutesPerShippedCycle != 115.0 {
		t.Errorf("llm minutes per shipped cycle = %v, want (30 + 40 + 160.055) / 2 rounded to 0.1 = 115.0", c.LLMMinutesPerShippedCycle)
	}
	if *c.FreshTokensPerShippedCycle != 3_419_121 || *c.CacheReadTokensPerShippedCycle != 66_215_915 {
		t.Errorf("tokens per shipped cycle = %v fresh, %v cache-read", *c.FreshTokensPerShippedCycle, *c.CacheReadTokensPerShippedCycle)
	}
	if c.UnmeasuredCalls != 3 || *c.AuditRoundsPerCycle != 2 || *c.ConvergenceRate != 2.0/3 {
		t.Errorf("cost = %+v; want 3 unmeasured calls, 6/3 audit rounds per cycle and a convergence of 2/3", c)
	}
}

func TestCompute_AWaveWithNoShipHasNoDefinedCost(t *testing.T) {
	t.Parallel()
	f := wavemeasure.Facts{Cycles: []wavemeasure.CycleSpend{cycleSpend(false, 60_000, 1, 1, 0, 2)}}

	c := wavemeasure.Compute(f, at(0)).Cost

	if c.LLMMinutesPerShippedCycle != nil || c.FreshTokensPerShippedCycle != nil || c.CacheReadTokensPerShippedCycle != nil {
		t.Errorf("cost = %+v, want each per-ship figure undefined (nil), not 0", c)
	}
	if *c.ConvergenceRate != 0 || *c.AuditRoundsPerCycle != 2 {
		t.Errorf("cost = %+v, want convergence 0 and 2 audit rounds per cycle", c)
	}
	b, err := json.Marshal(c)
	if err != nil || !strings.Contains(string(b), `"llm_minutes_per_shipped_cycle":null`) {
		t.Errorf("json = %s, %v; want the undefined cost as null", b, err)
	}
}

func TestCompute_AWaveWithNoCycleHasNoDefinedRate(t *testing.T) {
	t.Parallel()
	c := wavemeasure.Compute(wavemeasure.Facts{}, at(0)).Cost
	if c.AuditRoundsPerCycle != nil || c.ConvergenceRate != nil || c.Started != 0 {
		t.Errorf("cost = %+v, want no rate for a wave with no cycle", c)
	}
}
