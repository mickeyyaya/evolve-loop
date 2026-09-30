package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

const laneScopedItem = "claimable-lane-item" // writeClaimableInbox's first item

// runLaneCycle runs one composed cycle for a lane scoped to laneScopedItem over
// an inbox holding it plus one more claimable item (another lane's work).
func runLaneCycle(t *testing.T, decision string, prepare func(t *testing.T, root string)) (CycleResult, map[Phase]PhaseRunner) {
	t.Helper()
	return runLaneCycleWith(t, triageDecisionRunner{verdict: VerdictPASS, decision: decision}, prepare)
}

// runLaneCycleWith is runLaneCycle with the triage runner supplied.
func runLaneCycleWith(t *testing.T, triage PhaseRunner, prepare func(t *testing.T, root string)) (CycleResult, map[Phase]PhaseRunner) {
	t.Helper()
	root := writeClaimableInbox(t, 2)
	if prepare != nil {
		prepare(t, root)
	}
	runners := buildRunners(nil)
	runners[PhaseTriage] = triage
	worktree := &fakeWorktree{path: t.TempDir()}
	orchestrator := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithWorktreeProvisioner(worktree))
	result, err := orchestrator.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: root,
		GoalHash:    "lane-goal",
		Env:         map[string]string{ipcenv.FleetScopeKey: laneScopedItem},
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	return result, runners
}

func TestRunCycle_LaneThatAnswersForItsScopeEndsPlannedNoWork(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		decision string
		prepare  func(t *testing.T, root string)
	}{
		{name: "dropped with a reason (the cycle-1682 shape)", decision: `{"top_n":[],"dropped":[{"id":"claimable-lane-item","reason":"already-shipped-cycle-1679"}]}`},
		{name: "skip_shipped with its sha", decision: `{"top_n":[],"skip_shipped":[{"task_id":"claimable-lane-item","git_sha":"abc123"}]}`},
		{name: "escalated to the console", decision: `{"top_n":[],"escalate_block":[{"task_id":"claimable-lane-item","reason":"console-routed"}]}`},
		{
			// Consumed by an earlier ship before this lane's triage ended:
			// nothing in the lane's scope is claimable, whatever else is queued.
			name:     "the scoped item is no longer pending",
			decision: `{"top_n":[]}`,
			prepare: func(t *testing.T, root string) {
				if err := os.Remove(filepath.Join(root, ".evolve", "inbox", "2026-09-13T00-00-00Z-claimable-lane-item.json")); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, runners := runLaneCycle(t, tc.decision, tc.prepare)
			assertNoImplementationDispatch(t, runners, result)
			if result.TerminationReason != CycleTerminationTriageNoWork {
				t.Errorf("TerminationReason = %q, want %q — the lane's triage answered for its scope; another lane's queued work is not this lane's claim failure", result.TerminationReason, CycleTerminationTriageNoWork)
			}
			if result.FinalVerdict != CycleOutcomeSkippedUnknown {
				t.Errorf("FinalVerdict = %q, want %q (planned no-work, never FAIL)", result.FinalVerdict, CycleOutcomeSkippedUnknown)
			}
			if !IsTriageNoWorkResult(result) {
				t.Errorf("IsTriageNoWorkResult = false; result=%+v", result)
			}
		})
	}
}

func TestRunCycle_LaneThatLeavesItsScopeUnansweredStaysClaimFailed(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, decision string }{
		{name: "silent empty decision", decision: `{"top_n":[]}`},
		{name: "a deferral narrates the claim failure", decision: `{"top_n":[],"deferred":[{"id":"claimable-lane-item"}]}`},
		{name: "a drop with no reason", decision: `{"top_n":[],"dropped":[{"id":"claimable-lane-item","reason":""}]}`},
		{name: "answered for another lane's item", decision: `{"top_n":[],"dropped":[{"id":"claimable-lane-item-b","reason":"duplicate"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, runners := runLaneCycle(t, tc.decision, nil)
			assertNoImplementationDispatch(t, runners, result)
			assertNamedNonNoWorkTermination(t, result)
		})
	}
}

// decidingClaimTriageRunner performs triage's own claim through the REAL
// inboxmover.Claim (the persona's Step 0a.4 — RealInboxForTest, as
// claimingTriageRunner does in continuation_adopt_test.go) and then writes its
// decision: the scoped item leaves the inbox root for processing/cycle-<N>/,
// as it does live.
type decidingClaimTriageRunner struct {
	triageDecisionRunner
	root, taskID string
}

func (r decidingClaimTriageRunner) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	if err := RealInboxForTest.Claim(r.root, r.taskID, itoa(req.Cycle)); err != nil {
		return PhaseResponse{}, err
	}
	return r.triageDecisionRunner.Run(ctx, req)
}

const laneScopedFile = "2026-09-13T00-00-00Z-claimable-lane-item.json"

func TestRunCycle_ALaneThatClaimsItsScopeIsJudgedWhereTheClaimLeftIt(t *testing.T) {
	t.Parallel()
	claimThenDecide := func(t *testing.T, decision string) (CycleResult, map[Phase]PhaseRunner) {
		t.Helper()
		triage := &decidingClaimTriageRunner{triageDecisionRunner: triageDecisionRunner{verdict: VerdictPASS, decision: decision}, taskID: laneScopedItem}
		return runLaneCycleWith(t, triage, func(_ *testing.T, root string) { triage.root = root })
	}
	t.Run("claimed then omitted stays claim-failed", func(t *testing.T) {
		result, runners := claimThenDecide(t, `{"top_n":[]}`)
		assertNoImplementationDispatch(t, runners, result)
		assertNamedNonNoWorkTermination(t, result)
	})
	t.Run("claimed and answered ends planned no-work", func(t *testing.T) {
		result, runners := claimThenDecide(t, `{"top_n":[],"dropped":[{"id":"claimable-lane-item","reason":"stale: closed by #535"}]}`)
		assertNoImplementationDispatch(t, runners, result)
		if result.TerminationReason != CycleTerminationTriageNoWork || !IsTriageNoWorkResult(result) {
			t.Errorf("a claimed and answered scoped item is planned no-work: %+v", result)
		}
	})
}

func TestRunCycle_AnUnrelatedMalformedItemDoesNotFailALane(t *testing.T) {
	t.Parallel()
	malformed := func(t *testing.T, root string) {
		if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox", "2026-09-13T00-00-09Z-broken.json"), []byte("{not json"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	answered := `{"top_n":[],"dropped":[{"id":"claimable-lane-item","reason":"already-shipped-cycle-1679"}]}`
	if result, _ := runLaneCycle(t, answered, malformed); result.TerminationReason != CycleTerminationTriageNoWork {
		t.Errorf("an answered lane ends no-work despite an unrelated malformed item: %q", result.TerminationReason)
	}
	missing := func(t *testing.T, root string) {
		malformed(t, root)
		if err := os.Remove(filepath.Join(root, ".evolve", "inbox", laneScopedFile)); err != nil {
			t.Fatal(err)
		}
	}
	result, _ := runLaneCycle(t, `{"top_n":[]}`, missing)
	assertNamedNonNoWorkTermination(t, result)
}

func TestRunCycle_ASequentialTriageThatClaimsThenCommitsNothingStaysClaimFailed(t *testing.T) {
	t.Parallel()
	root := writeClaimableInbox(t, 1)
	runners := buildRunners(nil)
	runners[PhaseTriage] = &decidingClaimTriageRunner{triageDecisionRunner: triageDecisionRunner{verdict: VerdictPASS, decision: `{"top_n":[]}`}, root: root, taskID: laneScopedItem}
	orchestrator := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}))
	result, err := orchestrator.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "sequential"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	assertNoImplementationDispatch(t, runners, result)
	assertNamedNonNoWorkTermination(t, result)
}
