package core

// repair_eligibility_test.go — RED contract for the audit-repair corroboration
// rule (audit-repair-loop, wave-3 cycles 1572/1573/1574).
//
// An audit FAIL is terminal today: statemachine.go gives PhaseAudit exactly two
// successors (Ship on PASS/WARN, Retro on FAIL) and nothing ever routes back to
// Build. The repair loop adds ONE decision — is this failure repairable? — and
// this file pins that decision alone. It is deliberately a pure function with no
// I/O so the rule can be exhaustively table-tested; every caller-side concern
// (reading dossiers, resolving policy floors) is inverted into the input struct.
//
// THE SHAPE THIS EXISTS FOR. Cycles 1572/1573/1574 each halted under
// applyFailureDecisionFloor gate 2 — the agent-authored failure-decision.json
// category "infra-systemic". Their failure-dossier.json all record
// floor_candidate:"" (the DETERMINISTIC gate never fired), while the SAME retro
// agent's disposition.json recorded legitimacy:"legit-rejection" in the same
// cycle. One agent contradicted itself, and the prose half won, halting three
// cycles that two deterministic signals called task-level. The rule below makes
// prose lose that argument — and ONLY that argument: a deterministic floor
// candidate is still absolute.
//
// CONSERVATIVE BY CONSTRUCTION: absence of evidence never grants repair. A
// missing/indeterminate disposition is not-eligible, not eligible-by-default.

import "testing"

func TestDecideRepairEligibility(t *testing.T) {
	tests := []struct {
		name           string
		in             repairEligibilityInput
		wantEligible   bool
		wantIncoherent bool
	}{
		{
			// The wave-3 shape, and the whole reason this rule exists.
			name: "agent-only floor claim contradicted by deterministic silence and legit-rejection",
			in: repairEligibilityInput{
				DeterministicFloorCandidate: "",
				AgentClaimedFloor:           true,
				Legitimacy:                  "legit-rejection",
				Attempts:                    0,
				MaxAttempts:                 2,
			},
			wantEligible:   true,
			wantIncoherent: true,
		},
		{
			// No contradiction to record — the ordinary repairable rejection.
			name: "legit-rejection with no agent floor claim is repairable and coherent",
			in: repairEligibilityInput{
				Legitimacy:  "legit-rejection",
				Attempts:    0,
				MaxAttempts: 2,
			},
			wantEligible:   true,
			wantIncoherent: false,
		},
		{
			// ADR-0072 gate 1 is untouchable: a broken pipeline cannot buy a
			// retry by also writing a friendly disposition.
			name: "deterministic floor candidate is absolute even with legit-rejection",
			in: repairEligibilityInput{
				DeterministicFloorCandidate: "infra-systemic",
				Legitimacy:                  "legit-rejection",
				Attempts:                    0,
				MaxAttempts:                 2,
			},
			wantEligible:   false,
			wantIncoherent: false,
		},
		{
			name: "at the cap, no further repair",
			in: repairEligibilityInput{
				Legitimacy:  "legit-rejection",
				Attempts:    2,
				MaxAttempts: 2,
			},
			wantEligible: false,
		},
		{
			name: "second attempt is still under a cap of 2",
			in: repairEligibilityInput{
				Legitimacy:  "legit-rejection",
				Attempts:    1,
				MaxAttempts: 2,
			},
			wantEligible: true,
		},
		{
			// max=0 is the off switch, expressed as configuration rather than
			// as a feature flag.
			name: "a cap of zero disables repair entirely",
			in: repairEligibilityInput{
				Legitimacy:  "legit-rejection",
				Attempts:    0,
				MaxAttempts: 0,
			},
			wantEligible: false,
		},
		{
			// The auditor was wrong, not the code — re-running the build would
			// re-earn the same false rejection.
			name: "false-rejection is not repairable",
			in: repairEligibilityInput{
				Legitimacy:  "false-rejection",
				Attempts:    0,
				MaxAttempts: 2,
			},
			wantEligible: false,
		},
		{
			name: "infra-failure is not repairable",
			in: repairEligibilityInput{
				Legitimacy:  "infra-failure",
				Attempts:    0,
				MaxAttempts: 2,
			},
			wantEligible: false,
		},
		{
			name: "indeterminate is not repairable",
			in: repairEligibilityInput{
				Legitimacy:  "indeterminate",
				Attempts:    0,
				MaxAttempts: 2,
			},
			wantEligible: false,
		},
		{
			// Absent disposition (retro never wrote one / unreadable). Absence
			// of evidence must not grant a retry.
			name: "absent legitimacy is not repairable",
			in: repairEligibilityInput{
				Attempts:    0,
				MaxAttempts: 2,
			},
			wantEligible: false,
		},
		{
			// Out-of-vocabulary value: not in validLegitimacy, so it cannot be
			// trusted as a legit-rejection.
			name: "unknown legitimacy vocabulary is not repairable",
			in: repairEligibilityInput{
				Legitimacy:  "probably-fine",
				Attempts:    0,
				MaxAttempts: 2,
			},
			wantEligible: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := decideRepairEligibility(tc.in)

			if got.Eligible != tc.wantEligible {
				t.Errorf("Eligible = %v, want %v (reason: %q)", got.Eligible, tc.wantEligible, got.Reason)
			}
			if got.Incoherent != tc.wantIncoherent {
				t.Errorf("Incoherent = %v, want %v", got.Incoherent, tc.wantIncoherent)
			}
			// A decision an operator cannot read is a decision they cannot
			// audit — every outcome must say why, in both directions.
			if got.Reason == "" {
				t.Error("Reason is empty; every eligibility decision must explain itself")
			}
		})
	}
}

// TestDecideRepairEligibility_IncoherenceRequiresContradiction pins the narrow
// meaning of Incoherent: it flags the agent contradicting the deterministic
// evidence, NOT merely "a repair happened". Without this, Incoherent could drift
// into an alias for Eligible and stop being usable as a distinct signal.
func TestDecideRepairEligibility_IncoherenceRequiresContradiction(t *testing.T) {
	// An agent floor claim that the deterministic gate CORROBORATES is not an
	// incoherence — the two agree, and gate 1 halts on its own authority.
	got := decideRepairEligibility(repairEligibilityInput{
		DeterministicFloorCandidate: "infra-systemic",
		AgentClaimedFloor:           true,
		Legitimacy:                  "legit-rejection",
		MaxAttempts:                 2,
	})

	if got.Incoherent {
		t.Error("Incoherent = true when the deterministic gate corroborates the agent; want false")
	}
	if got.Eligible {
		t.Error("Eligible = true despite a deterministic floor candidate; gate 1 must stay absolute")
	}
}
