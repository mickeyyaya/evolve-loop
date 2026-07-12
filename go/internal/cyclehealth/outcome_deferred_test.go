package cyclehealth

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTiming(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "phase-timing.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Cycle-656: an all-families quota abort must classify DEFERRED, never as a
// failed cycle — and must not disturb the existing FAILED_EXPLAINED arm.
func TestClassifyOutcome_Deferred(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		timing     string
		want       Outcome
		wantDetail string
	}{
		{
			name:   "all-families abort reason classifies DEFERRED",
			timing: `[{"phase":"build","verdict":"FAIL","abort_reason":"all-families-exhausted: phase build: core: all CLI families quota-exhausted (exit=85)"}]`,
			want:   OutcomeDeferred,
		},
		{
			name:   "other abort reason stays FAILED_EXPLAINED",
			timing: `[{"phase":"build","verdict":"FAIL","abort_reason":"phase build: bridge artifact timeout"}]`,
			want:   OutcomeFailedExplained,
		},
		{
			name:   "ship PASS wins over a deferred earlier attempt (append-merge log)",
			timing: `[{"phase":"build","verdict":"FAIL","abort_reason":"all-families-exhausted: drained"},{"phase":"ship","verdict":"PASS","abort_reason":""}]`,
			want:   OutcomeShipped,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeTiming(t, dir, tc.timing)
			got, detail := ClassifyOutcome(dir)
			if got != tc.want {
				t.Errorf("outcome=%s (detail=%q), want %s", got, detail, tc.want)
			}
		})
	}
}
