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
		rollup  map[string]any
		want    Outcome
		wantSub string
	}{
		{
			name: "shipped",
			timing: []timingFixture{
				{Phase: "build", Verdict: "PASS"}, {Phase: "audit", Verdict: "PASS"}, {Phase: "ship", Verdict: "PASS"},
			},
			want: OutcomeShipped,
		},
		{
			name: "failed_explained_via_abort_reason",
			timing: []timingFixture{
				{Phase: "build", Verdict: "PASS"},
				{Phase: "audit", Verdict: "FAIL", AbortReason: "audit verdict FAIL — coverage floors unmet"},
			},
			want:    OutcomeFailedExplained,
			wantSub: "coverage floors",
		},
		{
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
			name: "failed_explained_via_fail_verdict_without_abort",
			timing: []timingFixture{
				{Phase: "build", Verdict: "PASS"},
				{Phase: "audit", Verdict: "FAIL"},
				{Phase: "retro", Verdict: "FAIL"},
			},
			want:    OutcomeFailedExplained,
			wantSub: "audit",
		},
		{
			name: "abort_reason_beats_prior_fail_verdict",
			timing: []timingFixture{
				{Phase: "mutation-gate", Verdict: "FAIL"},
				{Phase: "audit", Verdict: "FAIL", AbortReason: "observer killed mid-audit"},
			},
			want:    OutcomeFailedExplained,
			wantSub: "observer killed",
		},
		{
			name:   "failed_unexplained",
			timing: []timingFixture{{Phase: "build", Verdict: "PASS"}},
			want:   OutcomeFailedUnexplained,
		},
		{
			name: "ship_pass_outranks_earlier_abort",
			timing: []timingFixture{
				{Phase: "build", Verdict: "FAIL", AbortReason: "first attempt died"},
				{Phase: "build", Verdict: "PASS"},
				{Phase: "ship", Verdict: "PASS"},
			},
			want: OutcomeShipped,
		},
		{
			name:    "empty_workspace",
			timing:  nil,
			want:    OutcomeFailedExplained,
			wantSub: "initialization failed",
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
