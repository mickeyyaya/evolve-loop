package wavemeasure_test

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
	"github.com/mickeyyaya/evolve-loop/go/internal/wavemeasure"
)

func minutesPtr(v float64) *float64 { return &v }

func shippedIn(minutes float64) wavemeasure.CycleSpend {
	return wavemeasure.CycleSpend{Shipped: true, Verdict: cyclespend.Verdict{LLMMinutesToVerdict: minutes}}
}

func failedIn(minutes, failPoint float64) wavemeasure.CycleSpend {
	return wavemeasure.CycleSpend{Verdict: cyclespend.Verdict{
		LLMMinutesToVerdict: minutes, FailPointMinutes: minutesPtr(failPoint), WasteMinutes: minutesPtr(minutes - failPoint), FailFastRatio: minutesPtr(failPoint / minutes),
	}}
}

func near(got *float64, want float64) bool { return got != nil && math.Abs(*got-want) < 1e-9 }

func TestCompute_TheWaveMedianOfLLMMinutesToVerdictAndOfTheFailFastRatio(t *testing.T) {
	t.Parallel()
	cx := wavemeasure.Facts{Cycles: []wavemeasure.CycleSpend{shippedIn(40), shippedIn(20), failedIn(50, 30), failedIn(100, 20)}}

	c := wavemeasure.Compute(cx, at(0)).Cost

	if !near(c.MedianLLMMinutesToVerdict, 45) {
		t.Errorf("MedianLLMMinutesToVerdict = %v, want 45 (the middle of 20, 40, 50, 100)", c.MedianLLMMinutesToVerdict)
	}
	if !near(c.MedianFailFastRatio, 0.4) {
		t.Errorf("MedianFailFastRatio = %v, want 0.4 (the middle of 0.2 and 0.6)", c.MedianFailFastRatio)
	}
}

func TestCompute_AnOddCountOfCyclesTakesTheMiddleValue(t *testing.T) {
	t.Parallel()
	cx := wavemeasure.Facts{Cycles: []wavemeasure.CycleSpend{shippedIn(90), shippedIn(10), failedIn(30, 15)}}

	c := wavemeasure.Compute(cx, at(0)).Cost

	if !near(c.MedianLLMMinutesToVerdict, 30) || !near(c.MedianFailFastRatio, 0.5) {
		t.Errorf("medians = %v and %v, want 30 and 0.5", c.MedianLLMMinutesToVerdict, c.MedianFailFastRatio)
	}
}

func TestCompute_ACycleWithNoLLMTimeAndACycleWithNoFailPointStayOutOfTheMedians(t *testing.T) {
	t.Parallel()
	noLedger := wavemeasure.CycleSpend{}
	failedWithoutFailPoint := wavemeasure.CycleSpend{Verdict: cyclespend.Verdict{LLMMinutesToVerdict: 80}}
	cx := wavemeasure.Facts{Cycles: []wavemeasure.CycleSpend{noLedger, failedWithoutFailPoint, shippedIn(40)}}

	c := wavemeasure.Compute(cx, at(0)).Cost

	if !near(c.MedianLLMMinutesToVerdict, 60) {
		t.Errorf("MedianLLMMinutesToVerdict = %v, want 60 (40 and 80; the cycle with no ledger is out)", c.MedianLLMMinutesToVerdict)
	}
	if c.MedianFailFastRatio != nil {
		t.Errorf("MedianFailFastRatio = %v, want nil: no cycle has a fail point", *c.MedianFailFastRatio)
	}
}

func TestCompute_AWaveWithNoVerdictMinutesHasNullMedians(t *testing.T) {
	t.Parallel()
	c := wavemeasure.Compute(wavemeasure.Facts{Cycles: []wavemeasure.CycleSpend{{Shipped: true}}}, at(0)).Cost

	if c.MedianLLMMinutesToVerdict != nil || c.MedianFailFastRatio != nil {
		t.Errorf("cost = %+v, want both medians undefined (nil), not 0", c)
	}
	b, err := json.Marshal(c)
	if err != nil || !strings.Contains(string(b), `"median_llm_minutes_to_verdict":null`) || !strings.Contains(string(b), `"median_fail_fast_ratio":null`) {
		t.Errorf("json = %s, %v; want both medians as null", b, err)
	}
}
