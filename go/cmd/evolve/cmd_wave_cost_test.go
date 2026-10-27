package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/wavemeasure"
)

func f64(v float64) *float64 { return &v }

func TestWaveStatus_TextPrintsOneCostLine(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		cost wavemeasure.Cost
		want string
	}{
		"shipped": {wavemeasure.Cost{Started: 3, Shipped: 2, LLMMinutesPerShippedCycle: f64(115), FreshTokensPerShippedCycle: f64(3_419_121), CacheReadTokensPerShippedCycle: f64(66_215_915), AuditRoundsPerCycle: f64(2), ConvergenceRate: f64(2.0 / 3), UnmeasuredCalls: 3},
			"cost:    115.0 LLM min, 3.42 M fresh and 66.22 M cache-read tokens per shipped cycle (2 of 3 shipped) · audit rounds 2.00 per cycle · convergence 0.67 · 3 call(s) unmeasured\n"},
		"no ship": {wavemeasure.Cost{Started: 1, AuditRoundsPerCycle: f64(4), ConvergenceRate: f64(0)},
			"cost:    not defined (0 of 1 cycle(s) shipped) · audit rounds 4.00 per cycle · convergence 0.00\n"},
		"no cycle": {wavemeasure.Cost{}, "cost:    no cycle\n"},
		"hand-offs": {wavemeasure.Cost{Started: 2, AuditRoundsPerCycle: f64(1), ConvergenceRate: f64(0), HandoffStops: 1, HandoffWouldStops: 2, FalseStops: 1},
			"cost:    not defined (0 of 2 cycle(s) shipped) · audit rounds 1.00 per cycle · convergence 0.00 · hand-off stops 1, would stops 2 · false stops 1\n"},
		"spend limit": {wavemeasure.Cost{Started: 1, AuditRoundsPerCycle: f64(4), ConvergenceRate: f64(0), SpendLimitStops: 1, SpendLimitWouldStops: 0},
			"cost:    not defined (0 of 1 cycle(s) shipped) · audit rounds 4.00 per cycle · convergence 0.00 · spend-limit stops 1, would stops 0\n"},
	} {
		var b strings.Builder
		writeWaveMeasuresText(&b, wavemeasure.Measures{Cost: tc.cost})
		lines := strings.SplitAfter(b.String(), "\n")
		if len(lines) != 3 || lines[1] != tc.want {
			t.Errorf("%s: text = %q, want the second line %q", name, b.String(), tc.want)
		}
	}
}

func TestWaveStatus_JSONCarriesTheCostOfTheWave(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedMeasuredRuns()
	p.write(".evolve/runs/cycle-1835/llm-calls.ndjson", `{"phase":"build","duration_ms":3000000,"usage_status":"measured","tokens":{"input":100,"output":50,"cache_read":4000,"cache_write":50}}`+"\n")
	p.write(".evolve/runs/cycle-1837/llm-calls.ndjson", `{"phase":"build","duration_ms":600000,"usage_status":"unavailable","tokens":{}}`+"\n")
	h := newWaveHarness()

	h.run("status", "--json", "--project-root", p.root)

	var got struct {
		Measures struct {
			Cost map[string]any `json:"cost"`
		} `json:"measures"`
	}
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	c := got.Measures.Cost
	if c["started_cycles"] != 3.0 || c["shipped_cycles"] != 2.0 || c["llm_minutes_per_shipped_cycle"] != 30.0 || c["fresh_tokens_per_shipped_cycle"] != 100.0 ||
		c["cache_read_tokens_per_shipped_cycle"] != 2000.0 || c["unmeasured_calls"] != 1.0 {
		t.Errorf("cost = %v, want 3 started, 2 shipped, (50 + 10) / 2 = 30 LLM minutes, 100 fresh and 2000 cache-read tokens per ship, 1 unmeasured call", c)
	}
}

func withoutCost(m wavemeasure.Measures) wavemeasure.Measures {
	m.Cost = wavemeasure.Cost{}
	return m
}

func TestQualityReport_TheWaveScopePublishesTheCostPerShippedCycle(t *testing.T) {
	t.Parallel()
	root := pipelineProject(t)
	for id, ms := range map[int]int{1: 60_000, 2: 120_000} {
		p := filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", id), "llm-calls.ndjson")
		if err := os.WriteFile(p, []byte(fmt.Sprintf(`{"phase":"build","duration_ms":%d,"usage_status":"measured","tokens":{"input":1}}`+"\n", ms)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	env := qualityEnv{scopes: map[string][]qualityReader{"wave": {qualityScopes["wave"][5]}}, newCenter: publishingQualityCenter}

	rc, _ := runQualityReport(t, env, root)

	got := map[string]map[string]string{}
	for _, e := range channelEvents(t, root, "quality.reading") {
		got[e.Fields["index"]+" "+e.Fields["window"]] = e.Fields
	}
	if rc != 0 || got["PQ5 waves:1-2"]["value"] != "3.000" || got["PQ5 waves:1-2"]["stage"] != "report" || got["PQ5 wave:2"]["status"] != "no_data" {
		t.Errorf("rc %d readings %v; want PQ5 3 LLM minutes for the one ship of waves 1-2 at stage report, and no_data for wave 2 with no ship", rc, got)
	}
}
