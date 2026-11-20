package core

import (
	"context"
	"testing"
)

func TestConsumeAuditRepairGrant_IncrementsOnlyOnARepairReason(t *testing.T) {
	t.Parallel()

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

func TestAuditRepairGrant_ConsumesReasonEmittedByDecideAfterAuditFail(t *testing.T) {
	t.Parallel()

	cs := CycleState{CycleID: 1828, WorkspacePath: t.TempDir(), AuditFailReasons: []string{tSkillsDrift1828, tConflict1828}}
	next, reason, sig := floorOrchestrator(fixedNextStrategy{next: "end"}).decideAfterAuditFail(cs)
	if next != PhaseBuild || sig != nil {
		t.Fatalf("decideAfterAuditFail = (next=%s, sig=%+v), want PhaseBuild with nil signal", next, sig)
	}

	consumeAuditRepairGrant(&cs, reason)
	if cs.AuditRepairAttempts != 1 {
		t.Errorf("AuditRepairAttempts = %d after consuming emitted reason %q, want 1", cs.AuditRepairAttempts, reason)
	}
}

func TestResumePath_ReachesTheAuditFailDisposition(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}, cycleState: CycleState{CycleID: 1577, WorkspacePath: t.TempDir()}}
	led := &fakeLedger{}
	runners := buildRunners(map[Phase]string{PhaseAudit: VerdictFAIL, PhaseRetro: VerdictFAIL})
	runners[PhaseAudit] = &classDeclaringAuditRunner{t: t}
	o := NewOrchestrator(st, led, runners)

	res, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: root},
		&ResumePoint{Phase: string(PhaseAudit), CycleID: 1577})
	if err != nil {
		t.Fatalf("resume cycle: %v", err)
	}

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

func TestBlockerBreaker_RepairAttemptsWithinOneCycleCountOnce(t *testing.T) {
	t.Parallel()

	v := EvaluateBlockerBreaker([]FailureDigest{
		dg(1573, "audit|verdict-fail|aaa", "verdict-fail"),
		dg(1573, "audit|verdict-fail|bbb", "verdict-fail"),
		dg(1573, "audit|verdict-fail|ccc", "verdict-fail"),
	}, defaultBreakerCfg())

	if v.Halt {
		t.Fatalf("repair attempts inside ONE cycle must not trip the consecutive-cycle breaker, got %+v", v)
	}
}
