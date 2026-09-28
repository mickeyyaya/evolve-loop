package core

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
