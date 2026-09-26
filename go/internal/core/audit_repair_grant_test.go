package core

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
