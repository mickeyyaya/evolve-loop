package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
	"github.com/mickeyyaya/evolve-loop/go/internal/wavemeasure"
)

func TestWaveCycleText_ShowsTheHostMinutesBesideTheLLMMinutes(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		cycle wave.Cycle
		want  string
	}{
		"failed with a fail point": {
			wave.Cycle{ID: 7, Verdict: "FAIL", LLMVerdict: cyclespend.Verdict{LLMMinutesToVerdict: 50, HostMinutesToVerdict: 12.5, FailPointMinutes: f64(30), WasteMinutes: f64(20), FailFastRatio: f64(0.6)}},
			"cycle 7: verdict FAIL, not shipped, 50.0 LLM min to verdict, 12.5 host min, fail point 30.0 min, waste 20.0 min, fail-fast 0.60"},
		"shipped": {
			wave.Cycle{ID: 8, Verdict: "PASS", Shipped: true, LLMVerdict: cyclespend.Verdict{LLMMinutesToVerdict: 40, HostMinutesToVerdict: 8}},
			"cycle 8: verdict PASS, shipped, 40.0 LLM min to ship, 8.0 host min"},
		"failed with no fail point": {
			wave.Cycle{ID: 9, Verdict: "FAIL", LLMVerdict: cyclespend.Verdict{LLMMinutesToVerdict: 80, HostMinutesToVerdict: 3}},
			"cycle 9: verdict FAIL, not shipped, 80.0 LLM min to verdict, 3.0 host min"},
	} {
		if got := waveCycleText(tc.cycle); got != tc.want {
			t.Errorf("%s: text = %q, want %q", name, got, tc.want)
		}
	}
}

func TestWaveStatus_TheCostLineShowsTheHostMinutesBesideTheLLMMinutes(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		cost wavemeasure.Cost
		want string
	}{
		"per shipped cycle and median": {
			wavemeasure.Cost{Started: 3, Shipped: 2, LLMMinutesPerShippedCycle: f64(115), HostMinutesPerShippedCycle: f64(42.5), FreshTokensPerShippedCycle: f64(3_419_121), CacheReadTokensPerShippedCycle: f64(66_215_915), AuditRoundsPerCycle: f64(2), ConvergenceRate: f64(2.0 / 3), MedianLLMMinutesToVerdict: f64(45), MedianHostMinutesToVerdict: f64(20)},
			"cost:    115.0 LLM min, 42.5 host min, 3.42 M fresh and 66.22 M cache-read tokens per shipped cycle (2 of 3 shipped) · audit rounds 2.00 per cycle · convergence 0.67 · median 45.0 LLM min to verdict · median 20.0 host min to verdict\n"},
		"no ship": {
			wavemeasure.Cost{Started: 1, AuditRoundsPerCycle: f64(4), ConvergenceRate: f64(0), MedianLLMMinutesToVerdict: f64(45), MedianHostMinutesToVerdict: f64(12)},
			"cost:    not defined (0 of 1 cycle(s) shipped) · audit rounds 4.00 per cycle · convergence 0.00 · median 45.0 LLM min to verdict · median 12.0 host min to verdict\n"},
	} {
		var b strings.Builder
		writeWaveMeasuresText(&b, wavemeasure.Measures{Cost: tc.cost})
		lines := strings.SplitAfter(b.String(), "\n")
		if len(lines) != 3 || lines[1] != tc.want {
			t.Errorf("%s: text = %q, want the second line %q", name, b.String(), tc.want)
		}
	}
}

func (p wavePlane) seedHostMinutes() {
	p.t.Helper()
	p.seedVerdictCycles()
	stepper := &waveHostClock{at: waveTestClock.Add(-120 * time.Minute)}
	s := cyclespend.NewSampler(func() float64 { return 3.5 }, stepper.now)
	for _, gate := range []string{"build-floor", "ship"} {
		if err := s.Measure(gate, func() error { return nil }); err != nil {
			p.t.Fatal(err)
		}
	}
	if err := s.Flush(filepath.Join(p.root, ".evolve", "runs", "cycle-1837")); err != nil {
		p.t.Fatal(err)
	}
}

type waveHostClock struct{ at time.Time }

func (c *waveHostClock) now() time.Time {
	t := c.at
	c.at = c.at.Add(7*time.Minute + 30*time.Second)
	return t
}

func TestWaveStatus_JSONCarriesTheHostMinutesOfEachCycleAndTheWaveMedian(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedHostMinutes()
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
	if byID[1837].HostMinutesToVerdict != 15 || byID[1837].LLMMinutesToVerdict != 50 {
		t.Errorf("cycle 1837 verdict = %+v, want 15 host minutes beside 50 LLM minutes", byID[1837])
	}
	if byID[1835].HostMinutesToVerdict != 0 {
		t.Errorf("cycle 1835 verdict = %+v, want no host minutes: it has no gate spans", byID[1835])
	}
	if !ratioNear(got.Measures.Cost.MedianHostMinutesToVerdict, 15) {
		t.Errorf("median host minutes = %v, want 15", got.Measures.Cost.MedianHostMinutesToVerdict)
	}
}

func TestWaveStatus_TextShowsTheHostMinutesBesideTheLLMMinutes(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedHostMinutes()
	h := newWaveHarness()

	rc := h.run("status", "--project-root", p.root)

	out := h.stdout.String()
	for _, want := range []string{
		"50.0 LLM min to verdict, 15.0 host min, fail point 30.0 min, waste 20.0 min, fail-fast 0.60",
		"median 45.0 LLM min to verdict · median 15.0 host min to verdict",
	} {
		if rc != 0 || !strings.Contains(out, want) {
			t.Errorf("rc = %d; stdout lacks %q:\n%s", rc, want, out)
		}
	}
}
