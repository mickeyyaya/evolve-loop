package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

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
			name:     "claim-failure-narrated-as-deferral",
			verdict:  VerdictPASS,
			decision: `{"cycle":1623,"top_n":[],"committed_floors":[],"deferred_floors":[],"deferred":[{"id":"claimable-lane-item"}],"dropped":[],"skip_shipped":[],"skip_rejected":[],"escalate_block":[],"phase_skip":[]}`,
		},
		{
			// A FAIL verdict over the same evidence must not be granted planned
			// no-work: the claimable item is still the fact.
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

func TestRunCycle_EmptyInboxIsPlannedNoWork(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		root func(t *testing.T) string
	}{
		{"inbox-dir-empty", func(t *testing.T) string { return writeClaimableInbox(t, 0) }},
		{"inbox-dir-absent", func(t *testing.T) string { return t.TempDir() }},
		{
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
