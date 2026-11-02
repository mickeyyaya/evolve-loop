package main

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
	"github.com/mickeyyaya/evolve-loop/go/internal/wavemeasure"
)

func TestWaveCycleText_ShowsTheLLMMinutesToTheVerdictTheFailPointTheWasteAndTheRatio(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		cycle wave.Cycle
		want  string
	}{
		"failed with a fail point": {
			wave.Cycle{ID: 7, Verdict: "FAIL", LLMVerdict: cyclespend.Verdict{LLMMinutesToVerdict: 50, FailPointMinutes: f64(30), WasteMinutes: f64(20), FailFastRatio: f64(0.6)}},
			"cycle 7: verdict FAIL, not shipped, 50.0 LLM min to verdict, fail point 30.0 min, waste 20.0 min, fail-fast 0.60"},
		"shipped": {
			wave.Cycle{ID: 8, Verdict: "PASS", Shipped: true, LLMVerdict: cyclespend.Verdict{LLMMinutesToVerdict: 40}},
			"cycle 8: verdict PASS, shipped, 40.0 LLM min to ship"},
		"failed with no fail point": {
			wave.Cycle{ID: 9, Verdict: "FAIL", LLMVerdict: cyclespend.Verdict{LLMMinutesToVerdict: 80}},
			"cycle 9: verdict FAIL, not shipped, 80.0 LLM min to verdict"},
		"no ledger": {
			wave.Cycle{ID: 10, Verdict: "FAIL"},
			"cycle 10: verdict FAIL, not shipped"},
	} {
		if got := waveCycleText(tc.cycle); got != tc.want {
			t.Errorf("%s: text = %q, want %q", name, got, tc.want)
		}
	}
}

func TestWaveStatus_TheCostLineShowsTheWaveMedians(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		cost wavemeasure.Cost
		want string
	}{
		"both medians": {wavemeasure.Cost{Started: 1, AuditRoundsPerCycle: f64(4), ConvergenceRate: f64(0), MedianLLMMinutesToVerdict: f64(45), MedianFailFastRatio: f64(0.4)},
			"cost:    not defined (0 of 1 cycle(s) shipped) · audit rounds 4.00 per cycle · convergence 0.00 · median 45.0 LLM min to verdict · median fail-fast 0.40\n"},
		"minutes only": {wavemeasure.Cost{Started: 1, AuditRoundsPerCycle: f64(4), ConvergenceRate: f64(0), MedianLLMMinutesToVerdict: f64(45)},
			"cost:    not defined (0 of 1 cycle(s) shipped) · audit rounds 4.00 per cycle · convergence 0.00 · median 45.0 LLM min to verdict\n"},
	} {
		var b strings.Builder
		writeWaveMeasuresText(&b, wavemeasure.Measures{Cost: tc.cost})
		lines := strings.SplitAfter(b.String(), "\n")
		if len(lines) != 3 || lines[1] != tc.want {
			t.Errorf("%s: text = %q, want the second line %q", name, b.String(), tc.want)
		}
	}
}

func verdictCallRow(endedAgo, minutes int) string {
	ended := waveTestClock.Add(-time.Duration(endedAgo) * time.Minute)
	return fmt.Sprintf(`{"phase":"build","started_at":%q,"ended_at":%q,"duration_ms":%d,"usage_status":"measured","tokens":{"input":1,"output":1}}`+"\n",
		ended.Add(-time.Duration(minutes)*time.Minute).Format(time.RFC3339Nano), ended.Format(time.RFC3339Nano), minutes*60_000)
}

func (p wavePlane) seedVerdictCycles() {
	p.t.Helper()
	p.seedMeasuredRuns()
	ago := func(m int) time.Time { return waveTestClock.Add(-time.Duration(m) * time.Minute) }
	p.write(".evolve/runs/cycle-1835/llm-calls.ndjson", verdictCallRow(105, 15)+verdictCallRow(65, 25))
	p.write(".evolve/runs/cycle-1837/signals.ndjson",
		waveSignalLine(ago(110), "ledger.appended", "", "")+
			waveSignalLine(ago(80), "gate.rejected", "ORCHESTRATOR_BUILD_FLOOR", "")+
			waveSignalLine(ago(50), "cycle.sealed", "", `"final_verdict":"FAIL"`))
	p.write(".evolve/runs/cycle-1837/llm-calls.ndjson", verdictCallRow(80, 30)+verdictCallRow(50, 20))
}

func ratioNear(got *float64, want float64) bool { return got != nil && math.Abs(*got-want) < 1e-9 }

func TestWaveStatus_JSONCarriesTheLLMVerdictOfEachCycleAndTheWaveMedians(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedVerdictCycles()
	h := newWaveHarness()

	rc := h.run("status", "--json", "--project-root", p.root)

	var got waveStatusReport
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil || rc != 0 {
		t.Fatalf("rc = %d, decode %v:\n%s", rc, err, h.stdout.String())
	}
	byID := map[int]cyclespend.Verdict{}
	for _, c := range got.Cycles {
		byID[c.ID] = c.LLMVerdict
	}
	shipped, failed := byID[1835], byID[1837]
	if shipped.LLMMinutesToVerdict != 40 || shipped.FailPointMinutes != nil || shipped.FailFastRatio != nil {
		t.Errorf("cycle 1835 verdict = %+v, want 40 minutes to ship and no fail point", shipped)
	}
	if failed.LLMMinutesToVerdict != 50 || !ratioNear(failed.FailPointMinutes, 30) || !ratioNear(failed.WasteMinutes, 20) || !ratioNear(failed.FailFastRatio, 0.6) {
		t.Errorf("cycle 1837 verdict = %+v, want 50 minutes, fail point 30, waste 20, ratio 0.6", failed)
	}
	if c := got.Measures.Cost; !ratioNear(c.MedianLLMMinutesToVerdict, 45) || !ratioNear(c.MedianFailFastRatio, 0.6) {
		t.Errorf("cost medians = %v and %v, want 45 and 0.6", c.MedianLLMMinutesToVerdict, c.MedianFailFastRatio)
	}
}

func TestWaveStatus_TextListsThePerCycleValuesAndTheWaveMedian(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedVerdictCycles()
	h := newWaveHarness()

	rc := h.run("status", "--project-root", p.root)

	out := h.stdout.String()
	for _, want := range []string{
		"40.0 LLM min to ship",
		"50.0 LLM min to verdict, fail point 30.0 min, waste 20.0 min, fail-fast 0.60",
		"median 45.0 LLM min to verdict · median fail-fast 0.60",
	} {
		if rc != 0 || !strings.Contains(out, want) {
			t.Errorf("rc = %d; stdout lacks %q:\n%s", rc, want, out)
		}
	}
}
