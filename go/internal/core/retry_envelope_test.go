package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestComputeRetryEnvelope(t *testing.T) {
	fp := policy.DefaultSystemFailurePolicy()

	tests := []struct {
		name      string
		in        retryEnvelopeInput
		wantLegal []retryAction
		wantHalt  bool
	}{
		{
			name: "deterministic floor candidate halts and offers nothing",
			in: retryEnvelopeInput{
				DeterministicFloorCandidate: policy.CategoryInfraSystemic,
				DeclaredClass:               policy.CategoryCodeAuditFail,
				Policy:                      fp,
			},
			wantLegal: nil,
			wantHalt:  true,
		},
		{
			name: "task-level audit fail under the cap offers both re-entry points",
			in: retryEnvelopeInput{
				DeclaredClass: policy.CategoryCodeAuditFail,
				Attempts:      0,
				Policy:        fp,
			},
			wantLegal: []retryAction{retryActionRetryTDD, retryActionRetryBuild, retryActionDecline},
		},
		{
			name: "last attempt under the cap still offers retry",
			in: retryEnvelopeInput{
				DeclaredClass: policy.CategoryCodeAuditFail,
				Attempts:      1,
				Policy:        fp,
			},
			wantLegal: []retryAction{retryActionRetryTDD, retryActionRetryBuild, retryActionDecline},
		},
		{
			name: "at the policy cap only decline is legal",
			in: retryEnvelopeInput{
				DeclaredClass: policy.CategoryCodeAuditFail,
				Attempts:      2,
				Policy:        fp,
			},
			wantLegal: []retryAction{retryActionDecline},
		},
		{
			name: "build fail is also retry-with-fix",
			in: retryEnvelopeInput{
				DeclaredClass: policy.CategoryCodeBuildFail,
				Policy:        fp,
			},
			wantLegal: []retryAction{retryActionRetryTDD, retryActionRetryBuild, retryActionDecline},
		},
		{
			name: "a system-level declared class declines to the existing floor gates, it does not halt here",
			in: retryEnvelopeInput{
				DeclaredClass: policy.CategoryInfraSystemic,
				Policy:        fp,
			},
			wantLegal: []retryAction{retryActionDecline},
			wantHalt:  false,
		},
		{
			name: "a non-floor system category also declines rather than halting on prose",
			in: retryEnvelopeInput{
				DeclaredClass: policy.CategoryNonProgress,
				Policy:        fp,
			},
			wantLegal: []retryAction{retryActionDecline},
			wantHalt:  false,
		},
		{
			// defer-or-quarantine is task-level but NOT retryable: rebuilding
			// cannot fix a malformed intent.
			name: "defer-or-quarantine offers only decline",
			in: retryEnvelopeInput{
				DeclaredClass: policy.CategoryIntentMalformed,
				Policy:        fp,
			},
			wantLegal: []retryAction{retryActionDecline},
		},
		{
			name: "unknown class is conservative: decline, no halt",
			in: retryEnvelopeInput{
				DeclaredClass: "something-nobody-declared",
				Policy:        fp,
			},
			wantLegal: []retryAction{retryActionDecline},
		},
		{
			name: "absent declared class is conservative",
			in: retryEnvelopeInput{
				Policy: fp,
			},
			wantLegal: []retryAction{retryActionDecline},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeRetryEnvelope(tc.in)

			if got.Halt != tc.wantHalt {
				t.Errorf("Halt = %v, want %v (reason %q)", got.Halt, tc.wantHalt, got.Reason)
			}
			if !sameActions(got.Legal, tc.wantLegal) {
				t.Errorf("Legal = %v, want %v", got.Legal, tc.wantLegal)
			}
			if got.Reason == "" {
				t.Error("Reason is empty; every envelope must explain itself to an operator")
			}
		})
	}
}

// The envelope must be driven by the policy table, not by a constant that
// happens to equal it: a hardcoded 2 passes every case above, only changing
// the table can distinguish them.
func TestComputeRetryEnvelope_CapComesFromThePolicyTable(t *testing.T) {
	fp := policy.DefaultSystemFailurePolicy()
	cat := fp.Categories[policy.CategoryCodeAuditFail]
	cat.MaxRetries = 5
	fp.Categories[policy.CategoryCodeAuditFail] = cat

	got := computeRetryEnvelope(retryEnvelopeInput{
		DeclaredClass: policy.CategoryCodeAuditFail,
		Attempts:      3, // over the compiled default of 2, under the table's 5
		Policy:        fp,
	})

	if !containsAction(got.Legal, retryActionRetryTDD) {
		t.Errorf("Legal = %v at attempt 3 with MaxRetries=5; the cap is not being read from the policy table", got.Legal)
	}
}

func TestComputeRetryEnvelope_ZeroCapDisablesRetry(t *testing.T) {
	fp := policy.DefaultSystemFailurePolicy()
	cat := fp.Categories[policy.CategoryCodeAuditFail]
	cat.MaxRetries = 0
	fp.Categories[policy.CategoryCodeAuditFail] = cat

	got := computeRetryEnvelope(retryEnvelopeInput{DeclaredClass: policy.CategoryCodeAuditFail, Policy: fp})

	if containsAction(got.Legal, retryActionRetryTDD) || containsAction(got.Legal, retryActionRetryBuild) {
		t.Errorf("Legal = %v, want decline-only when the table caps retries at 0", got.Legal)
	}
}

func sameActions(a, b []retryAction) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsAction(as []retryAction, want retryAction) bool {
	for _, a := range as {
		if a == want {
			return true
		}
	}
	return false
}
