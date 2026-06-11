// outcome_test.go — R6 (concurrency-factory plan): the cycle-outcome SLO
// classifier, computed from the ADR-0044 C1 records every terminal path
// already writes (phase-timing.json + <phase>-usage.json sidecars +
// interaction-summary.json). This is the measurement instrument for the
// R8 shadow→enforce soak: SHIPPED / SALVAGED / FAILED_EXPLAINED are the
// only acceptable cycle endings; FAILED_UNEXPLAINED must be 0 and alarmed.
//
// Precedence (first match wins): SHIPPED (ship verdict PASS) → SALVAGED
// (no ship, but a correction-ladder salvage produced the artifact) →
// FAILED_EXPLAINED (an abort_reason was recorded — the C1 chokepoint
// guarantees abort paths record one) → FAILED_UNEXPLAINED (phases ran,
// nothing explains the ending — the bucket that pages someone).
package cyclehealth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeJSON(t *testing.T, dir, name string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

type timingFixture struct {
	Phase       string  `json:"phase"`
	Verdict     string  `json:"verdict"`
	CostUSD     float64 `json:"cost_usd"`
	AbortReason string  `json:"abort_reason,omitempty"`
}

func TestClassifyOutcome(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		timing  []timingFixture
		rollup  map[string]any // interaction-summary.json; nil = absent
		want    Outcome
		wantSub string // substring expected in the detail
	}{
		{
			name: "shipped",
			timing: []timingFixture{
				{Phase: "build", Verdict: "PASS"}, {Phase: "audit", Verdict: "PASS"}, {Phase: "ship", Verdict: "PASS"},
			},
			want: OutcomeShipped,
		},
		{
			// cycle-283 shape: build ran, audit FAILed, abort recorded.
			name: "failed_explained_via_abort_reason",
			timing: []timingFixture{
				{Phase: "build", Verdict: "PASS"},
				{Phase: "audit", Verdict: "FAIL", AbortReason: "audit verdict FAIL — coverage floors unmet"},
			},
			want:    OutcomeFailedExplained,
			wantSub: "coverage floors",
		},
		{
			// no ship, no abort — but the correction ladder salvaged the
			// deliverable (rung=salvage, artifact_appeared).
			name:   "salvaged",
			timing: []timingFixture{{Phase: "build", Verdict: "PASS"}},
			rollup: map[string]any{
				"schema_version": 1,
				"by_rung":        map[string]int{"salvage": 1},
				"by_result":      map[string]int{"artifact_appeared": 1},
			},
			want: OutcomeSalvaged,
		},
		{
			// The alarm bucket: phases ran, nothing explains the ending.
			name:   "failed_unexplained",
			timing: []timingFixture{{Phase: "build", Verdict: "PASS"}},
			want:   OutcomeFailedUnexplained,
		},
		{
			// Precedence: a ship PASS outranks a recorded abort earlier in
			// the cycle (a repaired ship is still SHIPPED).
			name: "ship_pass_outranks_earlier_abort",
			timing: []timingFixture{
				{Phase: "build", Verdict: "FAIL", AbortReason: "first attempt died"},
				{Phase: "build", Verdict: "PASS"},
				{Phase: "ship", Verdict: "PASS"},
			},
			want: OutcomeShipped,
		},
		{
			// Empty/no records at all: nothing ran — unexplained.
			name:   "empty_workspace",
			timing: nil,
			want:   OutcomeFailedUnexplained,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := t.TempDir()
			if tc.timing != nil {
				writeJSON(t, ws, "phase-timing.json", tc.timing)
			}
			if tc.rollup != nil {
				writeJSON(t, ws, "interaction-summary.json", tc.rollup)
			}
			got, detail := ClassifyOutcome(ws)
			if got != tc.want {
				t.Fatalf("ClassifyOutcome = %s (%s), want %s", got, detail, tc.want)
			}
			if tc.wantSub != "" && !strings.Contains(detail, tc.wantSub) {
				t.Errorf("detail %q must contain %q", detail, tc.wantSub)
			}
		})
	}
}
