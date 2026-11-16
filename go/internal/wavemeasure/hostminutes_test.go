package wavemeasure_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
	"github.com/mickeyyaya/evolve-loop/go/internal/wavemeasure"
)

func withHost(c wavemeasure.CycleSpend, hostMinutes float64) wavemeasure.CycleSpend {
	c.Verdict.HostMinutesToVerdict = hostMinutes
	return c
}

func TestCompute_HostMinutesPerShippedCycleAndTheirMedianSitBesideTheLLMMinutes(t *testing.T) {
	t.Parallel()
	f := wavemeasure.Facts{Cycles: []wavemeasure.CycleSpend{
		withHost(shippedIn(40), 10),
		withHost(shippedIn(20), 30),
		withHost(failedIn(50, 30), 20),
	}}

	c := wavemeasure.Compute(f, at(0)).Cost

	if !near(c.HostMinutesPerShippedCycle, 30) {
		t.Errorf("HostMinutesPerShippedCycle = %v, want (10 + 30 + 20) / 2 shipped = 30", c.HostMinutesPerShippedCycle)
	}
	if !near(c.MedianHostMinutesToVerdict, 20) {
		t.Errorf("MedianHostMinutesToVerdict = %v, want 20 (the middle of 10, 20, 30)", c.MedianHostMinutesToVerdict)
	}
	if !near(c.MedianLLMMinutesToVerdict, 40) {
		t.Errorf("MedianLLMMinutesToVerdict = %v, want 40: the host figures must not move the LLM median", c.MedianLLMMinutesToVerdict)
	}
}

func TestCompute_AWaveWithNoHostMinutesHasNoHostFigures(t *testing.T) {
	t.Parallel()
	f := wavemeasure.Facts{Cycles: []wavemeasure.CycleSpend{shippedIn(40), failedIn(50, 30)}}

	c := wavemeasure.Compute(f, at(0)).Cost

	if c.HostMinutesPerShippedCycle != nil || c.MedianHostMinutesToVerdict != nil {
		t.Errorf("cost = %+v, want both host figures undefined (nil), not 0", c)
	}
	b, err := json.Marshal(c)
	if err != nil || !strings.Contains(string(b), `"median_host_minutes_to_verdict":null`) || !strings.Contains(string(b), `"host_minutes_per_shipped_cycle":null`) {
		t.Errorf("json = %s, %v; want each undefined host figure as null", b, err)
	}
}

func TestCompute_AWaveWithHostMinutesAndNoShipHasAMedianButNoPerShipFigure(t *testing.T) {
	t.Parallel()
	f := wavemeasure.Facts{Cycles: []wavemeasure.CycleSpend{{Verdict: cyclespend.Verdict{HostMinutesToVerdict: 12}}}}

	c := wavemeasure.Compute(f, at(0)).Cost

	if c.HostMinutesPerShippedCycle != nil || !near(c.MedianHostMinutesToVerdict, 12) {
		t.Errorf("cost = %+v, want the median 12 and no per-ship figure", c)
	}
}
