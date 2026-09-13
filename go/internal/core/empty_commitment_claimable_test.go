package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// empty_commitment_claimable_test.go — cycle 1652 RED contract for the inbox
// item triage-empty-commitment-still-dispatches-spine (the split-out fourth
// acceptance line of the cycle-1623 P0).
//
// Cycle 1623: triage could not claim its selected item, wrote
// triage-decision.json with top_n [] and one deferral, and the orchestrator ran
// twelve more phases against no committed task. The round-2 gate
// (decideTriageTermination + the PhaseTriage branch of selectNext) now stops
// EVERY explicit empty commitment before tdd — but it stops it as
// CycleTerminationTriageNoWork, the LEGITIMATE planned-no-work disposition, with
// no regard for whether the inbox still held claimable work. A claim race
// between fleet lanes, a loader that drops an item, or an agent that mis-writes
// the decision therefore all get credited as "nothing to do" and the loop
// reads a defect as a clean SKIPPED. The distinguishing input is "was there
// work to claim" (the inbox record's own words); this file pins it where the
// decision is CONSUMED — the composed RunCycle / RunCycleFromPhase paths — so
// a router-layer-only fix cannot stay GREEN (the cycle-1623 H1 lesson).
//
// Contract (test-report.md ## AC-Materialization):
//
//	AC1 claimable inbox + top_n [] ⇒ no tdd/build/audit/ship dispatch, a NAMED
//	    terminal reason that is NOT the legitimate no-work reason, never PASS,
//	    and never credited as IsTriageNoWorkResult.
//	AC2 the legitimate empty-inbox case keeps recordPlannedNoWorkOutcome's
//	    SKIPPED disposition (regression pin).
//	anti-no-op: committed work beside a still-populated inbox must advance.
//
// The exact spelling of the named reason is the Builder's (the inbox names the
// two candidates: claim-failed / no-commitment); what is frozen is that it is
// non-empty, distinct from CycleTerminationTriageNoWork, and that the cycle is
// not classified as planned no-work.

// claimableInboxItem is a dispatchable lane item: route "lane", no protected
// fix surface, no continuation — exactly what inboxmover.Claim would accept.
const claimableInboxItem = `{
  "id": "claimable-lane-item",
  "title": "a real queued task the lane could have claimed",
  "kind": "bug",
  "weight": 0.7,
  "priority": "P1",
  "route": "lane",
  "files": ["go/internal/core/triage_termination.go"],
  "acceptance": ["the item is claimed and built"]
}`

// writeClaimableInbox materializes <root>/.evolve/inbox/<n> claimable items
// and returns the root. n == 0 creates the inbox directory but leaves it empty
// (the legitimate no-work control: the directory exists, nothing is queued).
func writeClaimableInbox(t *testing.T, n int) string {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		name := "2026-09-13T00-00-0" + string(rune('0'+i)) + "Z-claimable-lane-item.json"
		body := claimableInboxItem
		if i > 0 {
			body = `{"id":"claimable-lane-item-` + string(rune('a'+i)) + `","title":"another queued task","kind":"bug","weight":0.5,"priority":"P2","route":"lane","files":["go/internal/core/cyclerun_select.go"]}`
		}
		if err := os.WriteFile(filepath.Join(inbox, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// implementationPhases are the phases an empty commitment must never reach.
var implementationPhases = []Phase{PhaseTDD, PhaseBuildPlanner, PhaseSwarmPlan, PhaseBuild, PhaseAudit, PhaseShip}

// assertNoImplementationDispatch is the anti-1623 core: zero requests reached
// any implementation runner AND the recorded history ends at triage.
func assertNoImplementationDispatch(t *testing.T, runners map[Phase]PhaseRunner, result CycleResult) {
	t.Helper()
	for _, phase := range implementationPhases {
		fr, ok := runners[phase].(*fakeRunner)
		if !ok {
			continue
		}
		if got := len(fr.requests); got != 0 {
			t.Errorf("RED: %s dispatched %d time(s) after an empty commitment with claimable work (cycle-1623 shape)", phase, got)
		}
	}
	if len(result.PhasesRun) == 0 || result.PhasesRun[len(result.PhasesRun)-1] != PhaseTriage {
		t.Errorf("RED: PhasesRun = %v, must end at triage", result.PhasesRun)
	}
	for _, phase := range result.PhasesRun {
		for _, impl := range implementationPhases {
			if phase == impl {
				t.Errorf("RED: PhasesRun = %v records implementation phase %s", result.PhasesRun, phase)
			}
		}
	}
}

// assertNamedNonNoWorkTermination pins the disposition side of AC1: the cycle
// records a reason, that reason is not the legitimate-no-work reason, the
// verdict is never PASS, and the result is not credited as planned no-work.
func assertNamedNonNoWorkTermination(t *testing.T, result CycleResult) {
	t.Helper()
	if result.TerminationReason == "" {
		t.Errorf("RED: TerminationReason is empty — the gate must NAME why an empty commitment with claimable work stopped (claim-failed / no-commitment)")
	}
	if result.TerminationReason == CycleTerminationTriageNoWork {
		t.Errorf("RED: TerminationReason = %q — the legitimate planned-no-work reason was granted although the inbox held a claimable item", result.TerminationReason)
	}
	if result.FinalVerdict == VerdictPASS || result.FinalVerdict == CycleOutcomeShippedViaBuild {
		t.Errorf("RED: FinalVerdict = %q — an empty commitment over claimable work is never a PASS", result.FinalVerdict)
	}
	if IsTriageNoWorkResult(result) {
		t.Errorf("RED: IsTriageNoWorkResult = true — the loop's throughput/no-work accounting would credit a claim failure as a clean no-work cycle")
	}
}

// TestRunCycle_EmptyTriageClaimableWorkStopsBeforeImplementation — AC1 on the
// fresh dispatch root. Two rows: the raw claim-race shape (inbox item present,
// decision silent about it) and the narrated shape (the decision defers the
// item it failed to claim — cycle 1623's literal artifact). Both must stop
// before tdd with a named, non-no-work reason. Triage may be re-dispatched at
// most ONCE (the inbox's "re-dispatch triage once with the claim error"
// option); a loop that keeps re-asking is bounded here.
func TestRunCycle_EmptyTriageClaimableWorkStopsBeforeImplementation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		verdict  string
		decision string
	}{
		{
			name:     "claim-race-silent-decision",
			verdict:  VerdictPASS,
			decision: `{"top_n":[],"deferred":[],"dropped":[]}`,
		},
		{
			// cycle-1623 verbatim: the unclaimable item narrated as a deferral.
			name:     "claim-failure-narrated-as-deferral",
			verdict:  VerdictPASS,
			decision: `{"cycle":1623,"top_n":[],"committed_floors":[],"deferred_floors":[],"deferred":[{"id":"claimable-lane-item"}],"dropped":[],"skip_shipped":[],"skip_rejected":[],"escalate_block":[],"phase_skip":[]}`,
		},
		{
			// A FAIL verdict over the same evidence must not regress to the
			// cycle-1639 pre-fix behavior (plain FAIL, no disposition) NOR be
			// granted planned no-work: the claimable item is still the fact.
			name:     "failed-triage-with-claimable-inbox",
			verdict:  VerdictFAIL,
			decision: `{"top_n":[]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := writeClaimableInbox(t, 1)
			storage := &fakeStorage{}
			ledger := &fakeLedger{}
			runners := buildRunners(nil)
			triage := &countingTriageRunner{inner: triageDecisionRunner{verdict: tc.verdict, decision: tc.decision}}
			runners[PhaseTriage] = triage
			worktree := &fakeWorktree{path: t.TempDir()}
			orchestrator := NewOrchestrator(storage, ledger, runners, WithWorktreeProvisioner(worktree))

			result, err := orchestrator.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "claimable-work"})
			if err != nil {
				t.Fatalf("RunCycle: %v", err)
			}
			assertNoImplementationDispatch(t, runners, result)
			assertNamedNonNoWorkTermination(t, result)
			if triage.calls > 2 {
				t.Errorf("RED: triage dispatched %d times — at most one re-dispatch is allowed for a claim race", triage.calls)
			}
			// The claimable item must still be where a later cycle can claim it:
			// the gate reads the inbox, it never consumes it.
			if _, statErr := os.Stat(filepath.Join(root, ".evolve", "inbox", "2026-09-13T00-00-00Z-claimable-lane-item.json")); statErr != nil {
				t.Errorf("RED: the claimable inbox item vanished from the inbox root during the cycle: %v", statErr)
			}
		})
	}
}

// countingTriageRunner wraps triageDecisionRunner to count dispatches (the
// "re-dispatch once" bound).
type countingTriageRunner struct {
	inner triageDecisionRunner
	calls int
}

func (r *countingTriageRunner) Name() string { return r.inner.Name() }

func (r *countingTriageRunner) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	r.calls++
	return r.inner.Run(ctx, req)
}

// TestRunCycleFromPhase_ResumedEmptyTriageWithClaimableInboxStopsBeforeImplementation
// is the resumed-root twin (the cycle-1639 lesson: a paused-and-resumed run
// must not diverge from a fresh one on identical on-disk evidence). Resume
// routes triage termination through the same o.triageTermination call, but
// closeout's recordPlannedNoWorkOutcome reclassifies independently — both
// seams must see the claimable item.
func TestRunCycleFromPhase_ResumedEmptyTriageWithClaimableInboxStopsBeforeImplementation(t *testing.T) {
	t.Parallel()
	root := writeClaimableInbox(t, 1)
	ws := RunWorkspacePath(root, 7)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	worktree := &fakeWorktree{path: t.TempDir()}
	storage := &fakeStorage{
		state: State{LastCycleNumber: 7},
		cycleState: CycleState{
			CycleID:         7,
			RunID:           "original-run",
			WorkspacePath:   ws,
			ActiveWorktree:  worktree.path,
			CompletedPhases: []string{"scout"},
		},
	}
	runners := buildRunners(nil)
	runners[PhaseTriage] = triageDecisionRunner{verdict: VerdictPASS, decision: `{"top_n":[],"deferred":[{"id":"claimable-lane-item"}]}`}
	orchestrator := NewOrchestrator(storage, &fakeLedger{}, runners, WithWorktreeProvisioner(worktree))

	result, err := orchestrator.RunCycleFromPhase(context.Background(),
		CycleRequest{ProjectRoot: root, GoalHash: "claimable-work"},
		&ResumePoint{Phase: string(PhaseTriage), CycleID: 7})
	if err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	assertNoImplementationDispatch(t, runners, result)
	assertNamedNonNoWorkTermination(t, result)
}

// TestRunCycle_EmptyInboxIsPlannedNoWork — AC2, the regression pin. With the
// inbox directory present but EMPTY (and, second row, absent entirely — the
// pre-inbox test fixture shape every existing planned-no-work test uses), an
// explicit empty commitment is still the legitimate no-work disposition:
// SKIPPED via recordPlannedNoWorkOutcome, CycleTerminationTriageNoWork, the
// worktree cleaned, and IsTriageNoWorkResult true. A fix that treats every
// empty commitment as a failure passes AC1 and breaks this.
func TestRunCycle_EmptyInboxIsPlannedNoWork(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		root func(t *testing.T) string
	}{
		{"inbox-dir-empty", func(t *testing.T) string { return writeClaimableInbox(t, 0) }},
		{"inbox-dir-absent", func(t *testing.T) string { return t.TempDir() }},
		{
			// A console-routed item is operator-owned: a lane can never claim
			// it, so it is NOT claimable work and must not turn a legitimate
			// empty commitment into a claim failure (ADR-0074 I1).
			"only-console-routed-item",
			func(t *testing.T) string {
				root := writeClaimableInbox(t, 0)
				body := `{"id":"operator-owned","title":"console item","kind":"bug","route":"console-only","files":["go/internal/core/orchestrator.go"]}`
				if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox", "2026-09-13T00-00-00Z-operator-owned.json"), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
				return root
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := tc.root(t)
			storage := &fakeStorage{}
			runners := buildRunners(nil)
			runners[PhaseTriage] = triageDecisionRunner{verdict: VerdictPASS, decision: `{"top_n":[],"deferred":[],"dropped":[]}`}
			worktree := &fakeWorktree{path: t.TempDir()}
			orchestrator := NewOrchestrator(storage, &fakeLedger{}, runners, WithWorktreeProvisioner(worktree))

			result, err := orchestrator.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "nothing-queued"})
			if err != nil {
				t.Fatalf("RunCycle: %v", err)
			}
			assertNoImplementationDispatch(t, runners, result)
			if result.TerminationReason != CycleTerminationTriageNoWork {
				t.Errorf("TerminationReason = %q, want %q (legitimate no-work regressed)", result.TerminationReason, CycleTerminationTriageNoWork)
			}
			if result.FinalVerdict != CycleOutcomeSkippedUnknown {
				t.Errorf("FinalVerdict = %q, want %q (recordPlannedNoWorkOutcome's SKIPPED disposition regressed)", result.FinalVerdict, CycleOutcomeSkippedUnknown)
			}
			if !IsTriageNoWorkResult(result) {
				t.Errorf("IsTriageNoWorkResult = false for a legitimate empty inbox; result=%+v", result)
			}
			if got := len(worktree.cleaned); got != 1 {
				t.Errorf("worktree cleanup calls = %d, want 1", got)
			}
		})
	}
}

// TestRunCycle_CommittedWorkWithClaimableInboxStillDispatchesImplementation is
// the ANTI-NO-OP row: a gate keyed on "the inbox is non-empty" instead of
// "empty commitment ∧ claimable work" would satisfy AC1 and brick every
// productive cycle. Two items queued, one committed ⇒ tdd/build must run.
func TestRunCycle_CommittedWorkWithClaimableInboxStillDispatchesImplementation(t *testing.T) {
	t.Parallel()
	root := writeClaimableInbox(t, 2)
	storage := &fakeStorage{}
	runners := buildRunners(nil)
	runners[PhaseTriage] = triageDecisionRunner{verdict: VerdictPASS, decision: `{"top_n":[{"id":"claimable-lane-item"}],"deferred":[{"id":"claimable-lane-item-b"}]}`}
	worktree := &fakeWorktree{path: t.TempDir()}
	orchestrator := NewOrchestrator(storage, &fakeLedger{}, runners, WithWorktreeProvisioner(worktree))

	result, err := orchestrator.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "committed-work"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if result.TerminationReason != "" {
		t.Errorf("TerminationReason = %q, want none — committed work must never be terminated at triage", result.TerminationReason)
	}
	for _, phase := range []Phase{PhaseTDD, PhaseBuild} {
		if got := len(runners[phase].(*fakeRunner).requests); got == 0 {
			t.Errorf("%s never dispatched although triage committed a task", phase)
		}
	}
	if result.FinalVerdict != VerdictPASS {
		t.Errorf("FinalVerdict = %q, want PASS (the happy path regressed)", result.FinalVerdict)
	}
}
