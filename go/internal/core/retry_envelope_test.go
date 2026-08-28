package core

// retry_envelope_test.go — RED contract for the retry ENVELOPE: what the
// deterministic policy declares LEGAL after an audit FAIL.
//
// This is a Specification: a pure predicate over (declared class, deterministic
// floor candidate, attempts, policy). Pure because the same answer must come out
// on the live, routed and resume paths, and because one table has to pin every
// branch — every I/O concern is inverted into the input struct (Dependency
// Inversion; the policy is passed, never read from disk in here).
//
// THE FINDING THIS ENCODES. ADR-0072's category table already declares:
//
//	CategoryCodeAuditFail: {Level: LevelTask, Action: ActionRetryWithFix,
//	                        FixType: "address-audit-findings", MaxRetries: 2}
//
// and `fp.Categories[...]` was read in exactly ONE place (failure_dossier.go),
// only for Level. Action, MaxRetries and FixType were consumed NOWHERE. ADR-0092
// then built a parallel `max_audit_repair_attempts` knob and a disposition-prose
// eligibility rule beside it — including the same cap of 2. This envelope makes
// the existing declarative policy the single retry authority and deletes the
// parallel one.

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
			// Gate 1 is absolute and is evaluated BEFORE any policy lookup, so a
			// broken pipeline cannot buy a retry with a friendly declared class.
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
			// The ordinary case this whole redesign exists for.
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
			// MaxRetries is 2 in the table; at 2 the budget is spent.
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
			// H4 (architect review): the envelope must NOT mint a new halt
			// authority from an agent-declared class. Both pre-existing floor
			// gates require IsFloor, and gate 2 was DELIBERATELY narrowed to lose
			// against a contradicting disposition. Halting here on prose would
			// reintroduce exactly the disease this redesign cures — and worse,
			// non-floor system categories (transport-hang, non-progress) would
			// stop the batch on an auditor's word alone.
			//
			// So a system-level declared class DECLINES to retro, where the full
			// two-gate floor — with its corroboration — decides. Only the
			// DETERMINISTIC candidate halts here.
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
			// Absence of evidence never grants a retry — an unrecognised class is
			// not a licence, and it must not halt the loop either.
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

// THE WIRING PROOF. The envelope must be driven by the POLICY TABLE, not by a
// constant that happens to equal it. A hardcoded 2 passes every case above; only
// changing the table can distinguish them.
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

// And a cap of 0 in the table disables retry entirely — the off switch is
// configuration, never a feature flag.
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
