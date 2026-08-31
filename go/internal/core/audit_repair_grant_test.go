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
	"context"
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

// The reason string the emitter produces MUST be the one the consumer keys on.
// A literal on either side that drifts from the other silently disables the bound:
// the branch would grant repairs that nothing ever counts.
//
// This test previously read decision_branch.go — where the emitter USED to live.
// The redesign moved it to audit_fail_decision.go and the test kept passing its own
// stale target until it didn't, which is the same false-confidence shape it was
// written to prevent. It now targets the emitter and, more importantly, is backed by
// TestOrchestrator_AuditFailRetriesTheDevCycle and
// TestResumePath_ReachesTheAuditFailDisposition, which prove the bound end-to-end
// rather than by inspection.
func TestAuditRepairReasonPrefix_IsSingleSourced(t *testing.T) {
	body, err := os.ReadFile("audit_fail_decision.go")
	if err != nil {
		t.Fatalf("read audit_fail_decision.go: %v", err)
	}
	if strings.Contains(string(body), `"audit-repair: "`) {
		t.Error("the emitter builds the repair reason from a LITERAL; it must use auditRepairReasonPrefix so emitter and consumer cannot drift")
	}
	if !strings.Contains(string(body), "auditRepairReasonPrefix") {
		t.Error("the emitter does not reference auditRepairReasonPrefix; a granted repair would be unattributable to the consumer")
	}
}

// BOTH dispatch surfaces must actually REACH the audit-FAIL disposition.
//
// The previous version of this test was a strings.Contains grep for
// "consumeAuditRepairGrant(" in each file. It passed while resume.go's copy was
// DEAD CODE and while resume.go had no audit-FAIL branch at all — the feature was
// unreachable on the entire resume surface and a green suite said nothing. That is
// the false-confidence shape this codebase keeps producing, so this now drives the
// resume path and asserts the disposition it actually reaches.
func TestResumePath_ReachesTheAuditFailDisposition(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(map[Phase]string{PhaseAudit: VerdictFAIL, PhaseRetro: VerdictFAIL})
	runners[PhaseAudit] = &classDeclaringAuditRunner{t: t}
	o := NewOrchestrator(st, led, runners)
	root := t.TempDir()

	res, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: root},
		&ResumePoint{Phase: string(PhaseAudit), CycleID: 1577})
	if err != nil {
		t.Fatalf("resume cycle: %v", err)
	}

	// A resumed audit FAIL must re-enter the dev cycle exactly as the live loop
	// does, not fall through to the terminal retro.
	reentered := false
	for _, p := range res.PhasesRun {
		if p == PhaseTDD || p == PhaseBuild {
			reentered = true
		}
	}
	if !reentered {
		t.Errorf("a resumed audit FAIL never re-entered the dev cycle; phases=%v", res.PhasesRun)
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
