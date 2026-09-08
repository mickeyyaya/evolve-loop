package core

import (
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestEvaluateRetry_QuotaExhaustionPreservesBatchDisposition(t *testing.T) {
	t.Parallel()
	for _, optional := range []bool{false, true} {
		t.Run(map[bool]string{false: "mandatory error", true: "optional warning"}[optional], func(t *testing.T) {
			quotaErr := wrapTransient(85)
			runner := &alwaysFailRunner{name: "evaluator", err: quotaErr}
			o := retryParityOrchestrator(t, runner, "evaluator", phasespec.PhaseSpec{Optional: optional})
			o.retryConfig.RetryBackoffBaseS = 0
			cr := retryParityCycleRun(o, t)

			resp, attempts, err := cr.dispatchRunnerWithRetry(Phase("evaluator"), PhaseRequest{})

			if errors.Is(err, ErrAllFamiliesExhausted) {
				t.Fatalf("batch cannot advertise a quota pause without its own checkpoint lifecycle: %v", err)
			}
			if attempts != 2 || runner.n != 2 {
				t.Fatalf("attempts=%d calls=%d, want both 2", attempts, runner.n)
			}
			if optional {
				if err != nil || resp.Verdict != VerdictWARN {
					t.Fatalf("optional quota exhaustion: verdict=%q err=%v, want WARN and nil", resp.Verdict, err)
				}
			} else if !errors.Is(err, quotaErr) {
				t.Fatalf("mandatory quota error=%v, want original error %v", err, quotaErr)
			}
		})
	}
}
