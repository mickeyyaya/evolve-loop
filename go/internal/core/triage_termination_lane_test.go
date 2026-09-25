package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// triage_termination_lane_test.go — F30 (census finding, cycle 1682): a fleet
// lane is scoped to ONE item, and triage builds its menu from the inbox root,
// so the scoped item is still pending there when triage ends
// (inboxmover.ClaimLaneScope's placement note). Cycle 1682's triage dropped its
// scoped item with a reason (already shipped by 1679); the claimable-work check
// then read the WHOLE inbox, found other lanes' work, relabelled the honest
// no-work end as a claim failure and sealed FAIL — resetting the ship streak
// for a triage that did its job. The invariant is "triage answered for every
// scoped item", not "the inbox holds no claimable work". Pinned where the
// decision is CONSUMED (the composed RunCycle), per the cycle-1623 H1 lesson.

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

// TestRunCycle_LaneThatLeavesItsScopeUnansweredStaysClaimFailed keeps the
// triage-failure detector's teeth: a lane that says nothing about its scoped
// item, defers it (cycle 1623's narrated claim failure), drops it without a
// reason, or answers for a different id has NOT answered for its scope.
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

// TestRunCycle_ALaneThatClaimsItsScopeIsJudgedWhereTheClaimLeftIt (F30
// architecture review C1): triage claims before it selects, so the scoped item
// may sit in processing/cycle-N/, not the root, when triage ends. A lane that
// claims its item and then says nothing about it has NOT answered for it —
// judging only the root would let the agent's own claim turn a loud claim
// failure into a silent no-work end. Claimed AND answered is planned no-work.
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

// TestRunCycle_AnUnrelatedMalformedItemDoesNotFailALane (F30 architecture
// review m5): a lane is judged on its own scope, so one broken file elsewhere
// in the inbox must not turn its answered no-work into a claim failure — but
// when the scoped item itself cannot be found, a malformed file might BE it,
// and the check fails closed.
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

// TestRunCycle_ASequentialTriageThatClaimsThenCommitsNothingStaysClaimFailed
// (re-review MINOR 5): the sequential shape of C1 — the whole-inbox check read
// only the root, so a triage that claimed the queue's last item and committed
// nothing emptied the root and was granted planned no-work.
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
