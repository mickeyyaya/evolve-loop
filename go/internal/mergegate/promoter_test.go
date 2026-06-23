package mergegate_test

// Promoter — the kernel actuator ("kernel disposes"). These tests pin the
// stage/verdict state machine WITHOUT git: the injected Executor is a fake that
// counts calls and can be forced to fail. The invariants:
//   - off / unknown stage → do nothing (not even record).
//   - shadow / advisory → record the would-be promotion, NEVER touch the Executor.
//   - enforce + verdict != PASS → blocked, Executor.Promote NOT called.
//   - enforce + PASS → promote exactly once.
//   - enforce + PASS but Promote fails → auto-rollback (Rollback called once,
//     Promoted=false). This is the safety contract for auto-merge-on-PASS.

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/mergegate"
)

// fakeExecutor stands in for the git/ship-backed Executor. The assertion below
// also names the mergegate.Executor type so apicover counts it covered.
var _ mergegate.Executor = (*fakeExecutor)(nil)

type fakeExecutor struct {
	promoteCalls  int
	rollbackCalls int
	promoteErr    error
	rollbackErr   error
}

func (f *fakeExecutor) Promote(_ context.Context, _ /*integrationBranch*/, _ /*target*/ string) error {
	f.promoteCalls++
	return f.promoteErr
}

func (f *fakeExecutor) Rollback(_ context.Context, _ /*target*/ string) error {
	f.rollbackCalls++
	return f.rollbackErr
}

func baseInput() mergegate.PromoteInput {
	return mergegate.PromoteInput{
		Stage:             "enforce",
		Verdict:           "PASS",
		IntegrationBranch: "wave-3-integration",
		TargetBranch:      "main",
		Cadence:           mergegate.CadencePerWave,
	}
}

func TestPromoter_StageVerdictMatrix(t *testing.T) {
	cases := []struct {
		name               string
		stage              string
		verdict            string
		promoteErr         error
		rollbackErr        error
		wantPromoted       bool
		wantRolledBack     bool
		wantRollbackFailed bool
		wantRecorded       bool
		wantPromoteN       int
		wantRollbackN      int
	}{
		{name: "off-does-nothing", stage: "off", verdict: "PASS"},
		{name: "unknown-stage-does-nothing", stage: "bogus", verdict: "PASS"},
		{name: "shadow-records-only", stage: "shadow", verdict: "PASS", wantRecorded: true},
		{name: "advisory-records-only", stage: "advisory", verdict: "PASS", wantRecorded: true},
		{name: "shadow-records-even-on-fail", stage: "shadow", verdict: "FAIL", wantRecorded: true},
		{name: "advisory-records-even-on-fail", stage: "advisory", verdict: "FAIL", wantRecorded: true},
		{name: "enforce-fail-blocks", stage: "enforce", verdict: "FAIL"},
		{name: "enforce-warn-blocks", stage: "enforce", verdict: "WARN"},
		{name: "enforce-pass-promotes", stage: "enforce", verdict: "PASS", wantPromoted: true, wantPromoteN: 1},
		{name: "enforce-pass-failure-rolls-back", stage: "enforce", verdict: "PASS", promoteErr: errors.New("acceptance red"),
			wantRolledBack: true, wantPromoteN: 1, wantRollbackN: 1},
		// HIGH safety case: when BOTH the promotion AND its rollback fail, the result
		// must NOT claim RolledBack (main may be in an indeterminate state) — it must
		// surface RollbackFailed loudly so the caller escalates.
		{name: "enforce-pass-promote-and-rollback-both-fail", stage: "enforce", verdict: "PASS",
			promoteErr: errors.New("acceptance red"), rollbackErr: errors.New("rollback rejected"),
			wantRolledBack: false, wantRollbackFailed: true, wantPromoteN: 1, wantRollbackN: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := &fakeExecutor{promoteErr: tc.promoteErr, rollbackErr: tc.rollbackErr}
			in := baseInput()
			in.Stage, in.Verdict = tc.stage, tc.verdict
			var got mergegate.PromoteResult = mergegate.Promoter{Exec: fx}.Promote(context.Background(), in)

			if got.Promoted != tc.wantPromoted {
				t.Errorf("Promoted = %v, want %v (reason=%q)", got.Promoted, tc.wantPromoted, got.Reason)
			}
			if got.RolledBack != tc.wantRolledBack {
				t.Errorf("RolledBack = %v, want %v (reason=%q)", got.RolledBack, tc.wantRolledBack, got.Reason)
			}
			if got.RollbackFailed != tc.wantRollbackFailed {
				t.Errorf("RollbackFailed = %v, want %v (reason=%q)", got.RollbackFailed, tc.wantRollbackFailed, got.Reason)
			}
			if got.Recorded != tc.wantRecorded {
				t.Errorf("Recorded = %v, want %v", got.Recorded, tc.wantRecorded)
			}
			if fx.promoteCalls != tc.wantPromoteN {
				t.Errorf("Executor.Promote calls = %d, want %d", fx.promoteCalls, tc.wantPromoteN)
			}
			if fx.rollbackCalls != tc.wantRollbackN {
				t.Errorf("Executor.Rollback calls = %d, want %d", fx.rollbackCalls, tc.wantRollbackN)
			}
			if got.Reason == "" {
				t.Errorf("Reason is empty; every outcome must explain itself")
			}
		})
	}
}

// TestPromoter_EnforcePassNilExecutorBlocks pins the defensive guard: an enforce
// PASS with no Executor wired must block (never panic), since this is the only
// path that would auto-merge to main.
func TestPromoter_EnforcePassNilExecutorBlocks(t *testing.T) {
	in := baseInput() // Stage=enforce, Verdict=PASS
	got := mergegate.Promoter{Exec: nil}.Promote(context.Background(), in)
	if got.Promoted || got.RolledBack || got.Recorded {
		t.Fatalf("nil-executor enforce+PASS = %+v, want all-false (blocked)", got)
	}
	if got.Reason == "" {
		t.Errorf("Reason is empty; a blocked promotion must explain itself")
	}
}
