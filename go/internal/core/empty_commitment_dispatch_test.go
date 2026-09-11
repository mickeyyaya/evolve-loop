package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

type triageDecisionRunner struct {
	verdict  string
	decision string
}

func (r triageDecisionRunner) Name() string { return string(PhaseTriage) }

func (r triageDecisionRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	if err := os.WriteFile(filepath.Join(req.Workspace, "handoff-triage.json"), []byte(`{"cycle_size":"small","deliverable_kind":"code","phase_skip":[]}`), 0o644); err != nil {
		return PhaseResponse{}, err
	}
	if r.decision != "" {
		if err := os.WriteFile(filepath.Join(req.Workspace, "triage-decision.json"), []byte(r.decision), 0o644); err != nil {
			return PhaseResponse{}, err
		}
	}
	return PhaseResponse{Phase: string(PhaseTriage), Verdict: r.verdict, ArtifactsDir: req.Workspace}, nil
}

// Cycle 1623, audit round 2, finding H1 (CRITICAL).
//
// Round 2 gated the empty triage commitment at router.Route. Route's decision
// is only a PROPOSAL: cyclerun_select.go:103 hands it to enforceNext, whose
// PhaseEnd branch (routing_dispatch.go:59-63) asks
// StateMachine.CanTerminateEarly(current, shipPlanned) — which returns false
// unconditionally when shipPlanned is true (statemachine.go:230-233). Cycle
// 1623's own clamped plan schedules ship, so the proposal was discarded and the
// orchestrator dispatched tdd, build and audit against a task no phase was
// authorized to own. The router-layer test stayed GREEN through all of it.
//
// This is the table pinning the decision where it is CONSUMED. It is the
// in-package twin of the cycle predicate
// TestC1623_005_EmptyCommitmentTerminatesOnTheComposedDispatchPath, which
// drives the same contract through a whole RunCycle: this one isolates the
// authority, that one proves it is reached.
//
// The signals are always derived by running the real router.Digest over a real
// on-disk workspace. That is deliberate — TriageSignals.commitmentKnown is
// unexported in package router, so a hand-built literal here could not express
// the distinction between "committed nothing" and "no decision artifact", which
// is the whole point of the gate. It also means this test fails if the
// committed count ever stops being plumbed from triage-decision.json.

// digestTriageWorkspace materializes a cycle workspace whose triage decision
// commits `committed` tasks and returns the production routing signals for it.
// committed < 0 writes NO triage-decision.json — the commitment-UNKNOWN case.
func writeTriageWorkspace(t *testing.T, committed int) string {
	t.Helper()
	ws := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("handoff-scout.json", `{"cycle_size_estimate":"small","deliverable_kind":"code","backlog_size":1}`)
	write("handoff-triage.json", `{"cycle_size":"small","deliverable_kind":"code","phase_skip":[]}`)
	if committed >= 0 {
		topN := "["
		for i := 0; i < committed; i++ {
			if i > 0 {
				topN += ","
			}
			topN += `{"id":"task-` + string(rune('a'+i)) + `"}`
		}
		topN += "]"
		write("triage-decision.json", `{"cycle":1623,"top_n":`+topN+`,"deferred":[],"dropped":[]}`)
	}
	return ws
}

func digestTriageWorkspace(t *testing.T, committed int) router.RoutingSignals {
	t.Helper()
	ws := writeTriageWorkspace(t, committed)
	sig, err := router.Digest(ws, []string{"scout", "triage"})
	if err != nil {
		t.Fatalf("router.Digest(%s): %v", ws, err)
	}
	return sig
}

func TestSelectNext_StaticRoutingHonorsTriageTermination(t *testing.T) {
	t.Parallel()
	stages := []config.Stage{config.StageOff, config.StageShadow, config.StageAdvisory, config.StageEnforce}
	cases := []struct {
		name      string
		verdict   string
		committed int
		wantPhase Phase
		wantAct   loopAction
	}{
		{"failed-contract", VerdictFAIL, 1, PhaseEnd, loopBreak},
		{"empty-commitment", VerdictPASS, 0, PhaseEnd, loopBreak},
		{"committed-work", VerdictPASS, 1, PhaseTDD, loopNext},
	}
	for _, stage := range stages {
		for _, tc := range cases {
			t.Run(stage.String()+"/"+tc.name, func(t *testing.T) {
				cr := cycleRun{
					ctx: context.Background(),
					o: &Orchestrator{
						sm:       NewStateMachine(),
						strategy: router.StaticPreset{},
						now:      time.Now,
						ledger:   &fakeLedger{},
						cfg: config.RoutingConfig{
							Stage:     stage,
							Mandatory: []string{"scout", "build", "audit", "ship"},
							Order:     []string{"scout", "triage", "tdd", "build", "audit", "ship"},
						},
					},
					cs: CycleState{
						WorkspacePath:   writeTriageWorkspace(t, tc.committed),
						CompletedPhases: []string{"scout", "triage"},
					},
					current:     PhaseTriage,
					lastVerdict: tc.verdict,
				}
				gotPhase, gotAct, err := cr.selectNext()
				if err != nil {
					t.Fatalf("selectNext: %v", err)
				}
				if tc.verdict == VerdictPASS && tc.committed > 0 {
					if gotPhase == PhaseEnd || gotAct != loopNext {
						t.Errorf("selectNext() = (%s, %v), committed work must advance", gotPhase, gotAct)
					}
					return
				}
				if gotPhase != tc.wantPhase || gotAct != tc.wantAct {
					t.Errorf("selectNext() = (%s, %v), want (%s, %v)", gotPhase, gotAct, tc.wantPhase, tc.wantAct)
				}
			})
		}
	}
}

func TestRunCycle_EmptyTriageCommitmentIsPlannedNoWork(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		verdict     string
		decision    string
		wantVerdict string
		wantReason  string
		wantCleanup int
	}{
		{
			name:        "explicit-empty-decision",
			verdict:     VerdictPASS,
			decision:    `{"top_n":[],"deferred":[{"id":"protected","reason":"source surface is protected"}]}`,
			wantVerdict: CycleOutcomeSkippedUnknown,
			wantReason:  CycleTerminationTriageNoWork,
			wantCleanup: 1,
		},
		{
			name:        "failed-triage-with-empty-decision",
			verdict:     VerdictFAIL,
			decision:    `{"top_n":[]}`,
			wantVerdict: VerdictFAIL,
			wantCleanup: 0,
		},
		{
			name:        "failed-triage-without-decision",
			verdict:     VerdictFAIL,
			wantVerdict: VerdictFAIL,
			wantCleanup: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storage := &fakeStorage{}
			ledger := &fakeLedger{}
			runners := buildRunners(nil)
			runners[PhaseTriage] = triageDecisionRunner{verdict: tc.verdict, decision: tc.decision}
			worktree := &fakeWorktree{path: t.TempDir()}
			orchestrator := NewOrchestrator(storage, ledger, runners, WithWorktreeProvisioner(worktree))

			result, err := orchestrator.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "protected-task"})
			if err != nil {
				t.Fatalf("RunCycle: %v", err)
			}
			if result.FinalVerdict != tc.wantVerdict {
				t.Errorf("FinalVerdict = %q, want %q", result.FinalVerdict, tc.wantVerdict)
			}
			if result.TerminationReason != tc.wantReason {
				t.Errorf("TerminationReason = %q, want %q", result.TerminationReason, tc.wantReason)
			}
			if got := len(worktree.cleaned); got != tc.wantCleanup {
				t.Errorf("worktree cleanup calls = %d, want %d", got, tc.wantCleanup)
			}
			if got, want := result.PhasesRun, []Phase{PhaseScout, PhaseTriage}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
				t.Errorf("PhasesRun = %v, want %v", got, want)
			}
		})
	}
}

func TestEnforceNext_EmptyTriageCommitmentIsALegalTermination(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{
		sm: NewStateMachine(),
		cfg: config.RoutingConfig{
			Mandatory: []string{"scout", "build", "audit", "ship"},
			Order:     []string{"scout", "triage", "tdd", "build", "audit", "ship"},
		},
	}
	cases := []struct {
		name        string
		current     Phase
		staticNext  Phase
		verdict     string
		committed   int // <0 ⇒ no triage-decision.json (commitment unknown)
		shipPlanned bool
		wantPhase   Phase
		wantOK      bool
	}{
		// The defect. Cycle 1623's live configuration: triage committed nothing
		// and the clamped plan scheduled ship.
		{"empty-commitment-terminates-even-when-ship-planned", PhaseTriage, PhaseTDD, VerdictPASS, 0, true, PhaseEnd, true},
		// The pre-existing no-ship early exit is unaffected.
		{"empty-commitment-terminates-when-no-ship", PhaseTriage, PhaseTDD, VerdictPASS, 0, false, PhaseEnd, true},
		{"failed-contract-terminates-with-committed-work", PhaseTriage, PhaseTDD, VerdictFAIL, 1, true, PhaseEnd, true},
		// ANTI-NO-OP: committed work must never be terminated, however the
		// proposal arrived. A gate that keys on the phase instead of the count
		// passes the first row and bricks every productive cycle on this one.
		{"committed-work-outranks-a-spurious-end-proposal", PhaseTriage, PhaseTDD, VerdictPASS, 1, true, PhaseTDD, false},
		{"two-committed-tasks-still-advance", PhaseTriage, PhaseTDD, VerdictPASS, 2, true, PhaseTDD, false},
		// FAIL-OPEN: a missing decision artifact is an UNKNOWN commitment, not
		// an empty one. This is the row that keeps the kernel invariant intact
		// for every cycle that never wrote triage-decision.json.
		{"unknown-commitment-stays-blocked-when-ship-planned", PhaseTriage, PhaseTDD, VerdictPASS, -1, true, PhaseTDD, false},
		// The scout edge is untouched: triage has not run, so no commitment
		// exists to be empty (extra_coverage_test.go's "early-exit-blocked-when-
		// ship" pins the same edge with zero-valued signals).
		{"scout-edge-unchanged", PhaseScout, PhaseTriage, VerdictPASS, -1, true, PhaseTriage, false},
		// The gate must not leak past build: once real work exists it must be
		// evaluated, never abandoned — even if the commitment was empty.
		{"post-build-never-terminates-early", PhaseBuild, PhaseAudit, VerdictPASS, 0, true, PhaseAudit, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sig := digestTriageWorkspace(t, tc.committed)
			gotPhase, gotOK := o.enforceNext(tc.current, tc.staticNext, tc.verdict, sig,
				router.RouterDecision{NextPhase: router.PhaseEnd}, tc.shipPlanned)
			if gotPhase != tc.wantPhase || gotOK != tc.wantOK {
				t.Errorf("enforceNext(current=%s, staticNext=%s, committed=%d, shipPlanned=%t) = (%s, %v), want (%s, %v)",
					tc.current, tc.staticNext, tc.committed, tc.shipPlanned,
					gotPhase, gotOK, tc.wantPhase, tc.wantOK)
			}
		})
	}
}

// TestCanTerminateEarly_ShipPlannedInvariantIntact pins the 2-arg authority's
// documented meaning. The empty-commitment termination must be ADDITIVE — a new
// seam beside this method, or a check in enforceNext — never a rewrite of what
// CanTerminateEarly(from, shipPlanned) means, because
// extra_coverage_test.go:45's "early-exit-blocked-when-ship" row and this one
// both depend on it and neither may be weakened to make the fix pass.
func TestCanTerminateEarly_ShipPlannedInvariantIntact(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine()
	for _, from := range []Phase{PhaseScout, PhaseTriage, PhaseBuild, PhaseAudit} {
		if sm.CanTerminateEarly(from, true) {
			t.Errorf("CanTerminateEarly(%s, shipPlanned=true) = true, want false — a ship-intended cycle must satisfy the integrity floor before it can end", from)
		}
	}
	if !sm.CanTerminateEarly(PhaseTriage, false) {
		t.Errorf("CanTerminateEarly(triage, shipPlanned=false) = false, want true — the pre-existing no-ship early exit regressed")
	}
	if sm.CanTerminateEarly(PhaseBuild, false) {
		t.Errorf("CanTerminateEarly(build, shipPlanned=false) = true, want false — only pre-build decision points may terminate early")
	}
}

func TestResumeCursorTriageTerminationOverridesScheduledSuccessor(t *testing.T) {
	t.Parallel()
	ws := writeTriageWorkspace(t, 0)
	o := &Orchestrator{sm: NewStateMachine()}
	cursor := newResumeCursor(PhaseTriage)
	cs := CycleState{WorkspacePath: ws, CompletedPhases: []string{"scout", "triage"}}
	if next, err := cursor.next(o, cs); err != nil || next != PhaseTriage {
		t.Fatalf("first next = (%s, %v), want triage", next, err)
	}
	cursor.advance(PhaseTriage, VerdictPASS)
	cursor.schedule(PhaseTDD)

	next, err := cursor.next(o, cs)
	if err != nil {
		t.Fatalf("terminal next: %v", err)
	}
	if next != PhaseEnd {
		t.Errorf("terminal next = %s, want end", next)
	}
}

func TestSkippedUnknownWarningExcludesPlannedNoWork(t *testing.T) {
	t.Parallel()
	if shouldWarnSkippedUnknown(CycleResult{
		FinalVerdict:      CycleOutcomeSkippedUnknown,
		TerminationReason: CycleTerminationTriageNoWork,
	}) {
		t.Error("planned no-work would emit the discarded-implementation warning")
	}
	if !shouldWarnSkippedUnknown(CycleResult{FinalVerdict: CycleOutcomeSkippedUnknown}) {
		t.Error("an unexplained skipped cycle must retain the operator warning")
	}
}
