package core

// audit_repair_grant_test.go — RED contract for the audit-repair BOUND.
//
// The eligibility rule refuses to repair once Attempts >= MaxAttempts, but that
// refusal is only as real as the counter behind it. If nothing increments
// AuditRepairAttempts, Attempts is 0 forever and "bounded at 2" is decoration:
// a cycle that keeps failing its audit would rebuild without limit. This file
// pins the increment itself.
//
// It mirrors consumeBookkeepingRegradeGrant deliberately — same shape, same
// reason-prefix keying, same two branch surfaces. That comment names the exact
// hazard being guarded here: "the ONE primitive both branch surfaces call, so
// the bound cannot drift out of one of them", the recordFloorVerdictFailure
// recurrence class where a live-loop-only consumption silently reproduces the
// storm for resumed cycles.

import (
	"os"
	"strings"
	"testing"
)

func TestConsumeAuditRepairGrant_IncrementsOnlyOnARepairReason(t *testing.T) {
	tests := []struct {
		name   string
		reason string
		start  int
		want   int
	}{
		{
			name:   "a repair grant increments",
			reason: auditRepairReasonPrefix + "task-level rejection, attempt 1/2",
			start:  0,
			want:   1,
		},
		{
			name:   "a second repair grant increments again, reaching the cap",
			reason: auditRepairReasonPrefix + "task-level rejection, attempt 2/2",
			start:  1,
			want:   2,
		},
		{
			name:   "a system-failure-floor reason does not increment",
			reason: "system-failure-floor: infra-systemic",
			start:  0,
			want:   0,
		},
		{
			name:   "a proceed reason does not increment",
			reason: "proceed: fluent mode",
			start:  1,
			want:   1,
		},
		{
			// The bookkeeping regrade is a DIFFERENT once-per-cycle bound that
			// also re-dispatches. It must not spend the repair budget.
			name:   "a bookkeeping regrade grant does not spend a repair attempt",
			reason: BookkeepingRegradeReasonPrefix + "verdict conflict",
			start:  0,
			want:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cs := CycleState{AuditRepairAttempts: tc.start}

			consumeAuditRepairGrant(&cs, tc.reason)

			if cs.AuditRepairAttempts != tc.want {
				t.Errorf("AuditRepairAttempts = %d, want %d (reason %q)", cs.AuditRepairAttempts, tc.want, tc.reason)
			}
		})
	}
}

// The reason string the branch emits MUST be the one the consumer keys on. A
// literal on either side that drifts from the other silently disables the
// bound — the branch would grant repairs that nothing ever counts.
func TestAuditRepairReasonPrefix_IsSingleSourced(t *testing.T) {
	body, err := os.ReadFile("decision_branch.go")
	if err != nil {
		t.Fatalf("read decision_branch.go: %v", err)
	}
	if strings.Contains(string(body), `"audit-repair: "`) {
		t.Error("decision_branch.go builds the repair reason from a LITERAL; it must use auditRepairReasonPrefix so the emitter and the consumer cannot drift")
	}
	if !strings.Contains(string(body), "auditRepairReasonPrefix") {
		t.Error("decision_branch.go does not reference auditRepairReasonPrefix; the repair grant would be unattributable")
	}
}

// BOTH branch surfaces must consume the grant. cyclerun_record.go is the live
// loop; resume.go is the crash-resume path. A resume that skips the increment
// hands a resumed cycle a fresh, unbounded repair budget — which is precisely
// how the bookkeeping bound was documented to have failed before.
func TestAuditRepairGrant_ConsumedOnBothBranchSurfaces(t *testing.T) {
	for _, f := range []string{"cyclerun_record.go", "resume.go"} {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if !strings.Contains(string(body), "consumeAuditRepairGrant(") {
			t.Errorf("%s does not call consumeAuditRepairGrant; the repair bound drifts out of this surface", f)
		}
	}
}

// TestBlockerBreaker_RepairAttemptsWithinOneCycleCountOnce pins the invariant the
// audit-repair loop DEPENDS on: EvaluateBlockerBreaker keys its counted set by
// CYCLE number, so several failure digests produced inside one cycle collapse to
// a single entry and cannot inflate the consecutive-cycle run.
//
// Without this property, a cycle that repaired twice would emit three audit
// failures and trip the ceiling-3 consecutive-failures breaker BY ITSELF —
// turning a feature that exists to rescue a cycle into one that halts the batch.
// The behaviour is pre-existing; this pins it as a contract now that a feature
// relies on it, so a future change to the counting key fails here rather than
// silently halting live batches.
func TestBlockerBreaker_RepairAttemptsWithinOneCycleCountOnce(t *testing.T) {
	// One cycle, three distinct audit rejections: the original plus two repairs.
	v := EvaluateBlockerBreaker([]FailureDigest{
		dg(1573, "audit|verdict-fail|aaa", "verdict-fail"),
		dg(1573, "audit|verdict-fail|bbb", "verdict-fail"),
		dg(1573, "audit|verdict-fail|ccc", "verdict-fail"),
	}, defaultBreakerCfg())

	if v.Halt {
		t.Fatalf("repair attempts inside ONE cycle must not trip the consecutive-cycle breaker, got %+v", v)
	}
}
