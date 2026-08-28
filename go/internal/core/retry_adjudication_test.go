package core

// retry_adjudication_test.go — RED contract for the CLAMP: how an adjudicator's
// choice is bound by the deterministic envelope.
//
// The adjudicator is a deep-tier phase that reviews a failure architecturally and
// proposes a path. It exists because choosing BETWEEN legal options is judgment
// work (Core Agent Rule 5). It must never become another proxy-as-verdict, so two
// properties are pinned here:
//
//   - STRATEGY: adjudicated and deterministic paths are interchangeable — the
//     caller gets an action either way and never branches on which ran.
//   - NULL OBJECT: an absent, malformed, or out-of-vocabulary adjudication
//     degrades to the policy default. It is an ENHANCEMENT, never a precondition.
//     This is precisely the ADR-0092 defect designed out: there, the retry
//     depended on an agent artifact, and was measured reachable on 3 of 16
//     failures because the artifact was usually missing.
//
// The envelope is ORDERED by preference, so the default is simply Legal[0] — the
// most thorough legal action. No second table of defaults to drift.

import "testing"

func TestClampAdjudication(t *testing.T) {
	retryEnv := retryEnvelope{
		Legal:  []retryAction{retryActionRetryTDD, retryActionRetryBuild, retryActionDecline},
		Reason: "policy code-audit-fail ⇒ retry-with-fix, attempt 1/2",
	}
	declineEnv := declineOnly("retry budget spent")

	tests := []struct {
		name      string
		env       retryEnvelope
		adj       *adjudication
		want      retryAction
		wantClamp bool
	}{
		{
			name: "a legal choice is honored",
			env:  retryEnv,
			adj:  &adjudication{Action: retryActionRetryBuild, Justification: "defect is in the change, not the tests"},
			want: retryActionRetryBuild,
		},
		{
			name: "declining a retry policy permits is legal — the adjudicator may be more conservative",
			env:  retryEnv,
			adj:  &adjudication{Action: retryActionDecline, Justification: "defect is environmental; a rebuild re-earns it"},
			want: retryActionDecline,
		},
		{
			// THE SAFETY PROPERTY: it can never widen the envelope.
			name:      "an action outside the envelope is clamped to the policy default",
			env:       declineEnv,
			adj:       &adjudication{Action: retryActionRetryTDD, Justification: "I would like another go"},
			want:      retryActionDecline,
			wantClamp: true,
		},
		{
			name:      "an out-of-vocabulary action is clamped",
			env:       retryEnv,
			adj:       &adjudication{Action: retryAction("ship-it-anyway"), Justification: "looks fine to me"},
			want:      retryActionRetryTDD,
			wantClamp: true,
		},
		{
			// NULL OBJECT: no adjudication at all still yields a working decision.
			name: "an absent adjudication falls back to the policy default",
			env:  retryEnv,
			adj:  nil,
			want: retryActionRetryTDD,
		},
		{
			name: "an absent adjudication on a decline-only envelope declines",
			env:  declineEnv,
			adj:  nil,
			want: retryActionDecline,
		},
		{
			// An adjudication with no justification is not trustworthy input: the
			// whole point of the phase is the reasoning, not the verdict word.
			name:      "an unjustified choice is clamped",
			env:       retryEnv,
			adj:       &adjudication{Action: retryActionRetryBuild},
			want:      retryActionRetryTDD,
			wantClamp: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, clamped := clampAdjudication(tc.env, tc.adj)

			if got != tc.want {
				t.Errorf("action = %q, want %q", got, tc.want)
			}
			if clamped != tc.wantClamp {
				t.Errorf("clamped = %v, want %v", clamped, tc.wantClamp)
			}
		})
	}
}

// A halting envelope offers nothing, and no adjudication may resurrect it. This is
// the ADR-0072 boundary: the floor is not negotiable by an agent.
func TestClampAdjudication_HaltingEnvelopeIsNotNegotiable(t *testing.T) {
	halt := retryEnvelope{Halt: true, Reason: "deterministic floor candidate infra-systemic"}

	got, clamped := clampAdjudication(halt, &adjudication{
		Action:        retryActionRetryTDD,
		Justification: "the pipeline looks fine to me, let us try again",
	})

	if got != retryActionDecline {
		t.Errorf("action = %q on a halting envelope, want decline — an agent cannot overturn the floor", got)
	}
	if !clamped {
		t.Error("clamping a floor-halt proposal must be recorded")
	}
}

// The dispatch guard: judgment costs a deep-tier model, so it is only paid where
// there is genuinely a choice. One legal action is not a choice.
func TestAdjudicationNeeded(t *testing.T) {
	tests := []struct {
		name string
		env  retryEnvelope
		want bool
	}{
		{name: "multiple legal actions need a decision", env: retryEnvelope{Legal: []retryAction{retryActionRetryTDD, retryActionDecline}}, want: true},
		{name: "one legal action needs no decision", env: declineOnly("budget spent"), want: false},
		{name: "a halting envelope needs no decision", env: retryEnvelope{Halt: true}, want: false},
		{name: "an empty envelope needs no decision", env: retryEnvelope{}, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := adjudicationNeeded(tc.env); got != tc.want {
				t.Errorf("adjudicationNeeded = %v, want %v", got, tc.want)
			}
		})
	}
}
