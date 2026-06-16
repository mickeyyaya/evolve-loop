package core

// apicover_misc_test.go — ADR-0050 Phase 5 public-API coverage: white-box
// (`package core`) tests that NAME + EXERCISE the last exported symbols in
// internal/core the apicover gate still flags uncovered. Each test asserts a
// real behavior of the symbol it covers (Rule 9 — no `_ = pkg.X` padding):
//
//   - FailureAdvisor / FailureAdvisorOption (failure_advisor.go) — option EFFECT
//     reaches the bridge request via a real Advise call.
//   - PhaseAdvisor / PhaseAdvisorOption (phase_advisor.go) — option EFFECT reaches
//     BridgeRequest.{CLI,Model} via a real Plan call.
//   - Observer (observer.go) — compile-time conformance + a real Start/cancel.
//   - StateUpdater (alloc.go) — compile-time conformance + a real allocate RMW.
//   - WorktreeProvisioner (worktree.go) — compile-time conformance + Create/Cleanup.
//   - ThroughputRecorder (throughput_hook.go) — the func-typed seam fires on a
//     shipped cycle and mutates the State it is handed.
//   - SealResult (reset.go) — every field, via a SealCycle dry-run.
//   - StateMachine (statemachine.go) — Next / CanTransition over the spine.
//   - VerdictReason (verdict.go) — ReasonFromDiagnostics folds a FAIL diagnostic.
//   - Orchestrator.FailureAdviserWired (failure_hook.go) — true with the adviser
//     option, false on a bare orchestrator (executed, not just named).
//   - PhaseBoundaryCheckpointer (orchestrator.go) — the package var the
//     checkpoint package sets via init(); core cannot import checkpoint
//     (circular), so we assign a recording closure, drive a full RunCycle (the
//     real consumer in cyclerun_record.go invokes it), and assert it fired.
//   - CycleStateFile (runworkspace.go) — the constant SealCycle reads from.
//   - PhaseSwarmPlan (phase.go) — Phase.IsValid()/String() over the const.

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/failureadapter"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// --- FailureAdvisor + FailureAdvisorOption --------------------------------

// TestFailureAdvisorOption_EffectReachesBridge names FailureAdvisor +
// FailureAdvisorOption and proves the option is FUNCTIONAL: a typed
// FailureAdvisorOption built by WithFailureAdvisorCLI changes the CLI that
// actually reaches the bridge request on a real Advise call (not just a struct
// field). WithFailureAdvisorModel is asserted the same way.
func TestFailureAdvisorOption_EffectReachesBridge(t *testing.T) {
	t.Parallel()
	fb := &fakeBridge{stdout: `{"cause":"dead_shell","pane_substr":"locked vault prompt here","justification":"the REPL exited to a locked vault"}`}

	var cliOpt FailureAdvisorOption = WithFailureAdvisorCLI("codex-tmux")
	var modelOpt FailureAdvisorOption = WithFailureAdvisorModel("gpt-5.5")
	var adv *FailureAdvisor = NewFailureAdvisor(fb, cliOpt, modelOpt)

	if _, err := adv.Advise(context.Background(), baseFailureInput()); err != nil {
		t.Fatalf("Advise: %v", err)
	}
	// The option's EFFECT — the configured CLI/model flow to the bridge.
	if fb.gotReq.CLI != "codex-tmux" {
		t.Errorf("WithFailureAdvisorCLI ineffective: BridgeRequest.CLI=%q, want codex-tmux", fb.gotReq.CLI)
	}
	if fb.gotReq.Model != "gpt-5.5" {
		t.Errorf("WithFailureAdvisorModel ineffective: BridgeRequest.Model=%q, want gpt-5.5", fb.gotReq.Model)
	}
}

// --- PhaseAdvisor + PhaseAdvisorOption ------------------------------------

// TestPhaseAdvisorOption_EffectReachesBridge names PhaseAdvisor +
// PhaseAdvisorOption and proves the option type is functional: typed
// PhaseAdvisorOptions (WithProposerCLI / WithProposerModel) change the
// {CLI,Model} that reach BridgeRequest on a real Plan launch.
func TestPhaseAdvisorOption_EffectReachesBridge(t *testing.T) {
	t.Parallel()
	fb := &fakeBridge{stdout: `[{"phase":"scout","run":true,"justification":"x"}]`}

	var cliOpt PhaseAdvisorOption = WithProposerCLI("agy")
	var modelOpt PhaseAdvisorOption = WithProposerModel("gemini-3.5-flash")
	var adv *PhaseAdvisor = NewPhaseAdvisor(fb, cliOpt, modelOpt)

	if _, err := adv.Plan(baseRouteInput()); err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if fb.gotReq.CLI != "agy" {
		t.Errorf("WithProposerCLI ineffective: BridgeRequest.CLI=%q, want agy", fb.gotReq.CLI)
	}
	if fb.gotReq.Model != "gemini-3.5-flash" {
		t.Errorf("WithProposerModel ineffective: BridgeRequest.Model=%q, want gemini-3.5-flash", fb.gotReq.Model)
	}
}

// --- Observer -------------------------------------------------------------

// recordingObserver (observer_test.go) already implements Observer; assert
// conformance at compile time and exercise the contract directly.
var _ Observer = (*recordingObserver)(nil)
var _ Observer = noopObserver{}

// TestObserver_StartReturnsCancel names the Observer interface and exercises a
// real implementation: Start records the phase and returns a callable cancel.
func TestObserver_StartReturnsCancel(t *testing.T) {
	t.Parallel()
	var obs Observer = &recordingObserver{}
	cancel := obs.Start(context.Background(), "tdd", PhaseRequest{Cycle: 7})
	if cancel == nil {
		t.Fatal("Observer.Start must return a non-nil cancel")
	}
	cancel() // must not panic
	ro := obs.(*recordingObserver)
	ro.mu.Lock()
	defer ro.mu.Unlock()
	if len(ro.starts) != 1 || ro.starts[0] != "tdd" {
		t.Errorf("Observer.Start did not record the phase: %v", ro.starts)
	}
	if got := ro.cancelCalls.Load(); got != 1 {
		t.Errorf("cancel calls=%d, want 1", got)
	}
}

// --- StateUpdater ---------------------------------------------------------

// memUpdater (alloc_test.go) implements StateUpdater; pin it.
var _ StateUpdater = (*memUpdater)(nil)

// TestStateUpdater_AllocatesThroughRMW names the StateUpdater interface and
// exercises it through AllocateCycleNumber — the serialized RMW mints the next
// number and persists the lease.
func TestStateUpdater_AllocatesThroughRMW(t *testing.T) {
	t.Parallel()
	var su StateUpdater = &memUpdater{st: State{LastCycleNumber: 41}}
	n, err := AllocateCycleNumber(context.Background(), su)
	if err != nil {
		t.Fatalf("AllocateCycleNumber: %v", err)
	}
	if n != 42 {
		t.Errorf("allocated %d, want 42 (LastCycleNumber+1 on a fresh lease)", n)
	}
}

// --- WorktreeProvisioner --------------------------------------------------

// fakeWorktree (worktree_test.go) and gitWorktree both implement
// WorktreeProvisioner; pin both.
var _ WorktreeProvisioner = (*fakeWorktree)(nil)
var _ WorktreeProvisioner = gitWorktree{}

// TestWorktreeProvisioner_CreateAndCleanup names the WorktreeProvisioner
// interface and exercises both methods on the fake: Create returns the scripted
// path and records the cycle; Cleanup records the removed path.
func TestWorktreeProvisioner_CreateAndCleanup(t *testing.T) {
	t.Parallel()
	var wp WorktreeProvisioner = &fakeWorktree{path: "/tmp/wt/cycle-7"}
	got, err := wp.Create("/proj", 7)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got != "/tmp/wt/cycle-7" {
		t.Errorf("Create path=%q, want /tmp/wt/cycle-7", got)
	}
	if err := wp.Cleanup("/proj", got); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	fw := wp.(*fakeWorktree)
	if len(fw.createdCycles) != 1 || fw.createdCycles[0] != 7 {
		t.Errorf("Create did not record cycle: %v", fw.createdCycles)
	}
	if len(fw.cleaned) != 1 || fw.cleaned[0] != "/tmp/wt/cycle-7" {
		t.Errorf("Cleanup did not record path: %v", fw.cleaned)
	}
}

// --- ThroughputRecorder ---------------------------------------------------

// TestThroughputRecorder_FiresAndMutatesState names the ThroughputRecorder
// func type and exercises it: a recorder mutates the State it is handed, and
// the value satisfies the WithThroughputRecorder seam.
func TestThroughputRecorder_FiresAndMutatesState(t *testing.T) {
	t.Parallel()
	var gotCycle int
	var gotWs string
	var rec ThroughputRecorder = func(state *State, cycle int, workspacePath string) {
		gotCycle = cycle
		gotWs = workspacePath
		state.LastCycleNumber = cycle // prove it can mutate in place
	}

	st := &State{LastCycleNumber: 0}
	rec(st, 99, "/tmp/ws")
	if gotCycle != 99 || gotWs != "/tmp/ws" {
		t.Errorf("recorder got (cycle=%d, ws=%q), want (99, /tmp/ws)", gotCycle, gotWs)
	}
	if st.LastCycleNumber != 99 {
		t.Errorf("recorder must mutate state in place: LastCycleNumber=%d, want 99", st.LastCycleNumber)
	}
	// The recorder is a valid seam value — the option accepts it without panic.
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithThroughputRecorder(rec))
	if !o.ThroughputRecorderWired() {
		t.Error("WithThroughputRecorder did not wire the recorder")
	}
}

// --- SealResult + CycleStateFile ------------------------------------------

// TestSealResult_FullFieldFromDryRun names SealResult and CycleStateFile.
// SealCycle reads <EvolveDir>/CycleStateFile; a dry-run populates every
// SealResult field without mutating anything.
func TestSealResult_FullFieldFromDryRun(t *testing.T) {
	t.Parallel()
	// CycleStateFile is the filename SealCycle reads; pin its value.
	if CycleStateFile != "cycle-state.json" {
		t.Fatalf("CycleStateFile=%q, want cycle-state.json", CycleStateFile)
	}

	ev := t.TempDir()
	workspace := sealFixture(t, ev, 108)

	opts := sealOpts(ev)
	opts.DryRun = true
	var res SealResult
	res, err := SealCycle(context.Background(), &recordingLedger{}, opts)
	if err != nil {
		t.Fatalf("SealCycle dry-run: %v", err)
	}
	// Every SealResult field.
	if res.SealedCycleID != 108 {
		t.Errorf("SealedCycleID=%d, want 108", res.SealedCycleID)
	}
	if res.SealedPhase != "scout" {
		t.Errorf("SealedPhase=%q, want scout", res.SealedPhase)
	}
	if res.Workspace != workspace {
		t.Errorf("Workspace=%q, want %q", res.Workspace, workspace)
	}
	if res.ArchiveDir == "" || res.ArchiveDir == workspace {
		t.Errorf("ArchiveDir=%q, want a distinct archive sibling of the workspace", res.ArchiveDir)
	}
	if res.NextCycle != 109 {
		t.Errorf("NextCycle=%d, want 109", res.NextCycle)
	}
	if !res.DryRun {
		t.Error("DryRun must be true on a dry-run seal")
	}
}

// --- StateMachine ---------------------------------------------------------

// TestStateMachine_NextAndCanTransition names StateMachine and exercises both
// the verdict-driven Next() and the structural CanTransition() over the spine.
func TestStateMachine_NextAndCanTransition(t *testing.T) {
	t.Parallel()
	var sm *StateMachine = NewStateMachine()

	// Verdict-driven successor: audit PASS → ship.
	nxt, err := sm.Next(PhaseAudit, VerdictPASS)
	if err != nil {
		t.Fatalf("Next(audit, PASS): %v", err)
	}
	if nxt != PhaseShip {
		t.Errorf("Next(audit, PASS)=%s, want ship", nxt)
	}
	// audit FAIL → retro.
	if nxt, _ := sm.Next(PhaseAudit, VerdictFAIL); nxt != PhaseRetro {
		t.Errorf("Next(audit, FAIL)=%s, want retro", nxt)
	}
	// Structural legality.
	if !sm.CanTransition(PhaseBuild, PhaseAudit) {
		t.Error("CanTransition(build, audit) must be legal")
	}
	if sm.CanTransition(PhaseBuild, PhaseShip) {
		t.Error("CanTransition(build, ship) must be illegal — build cannot skip audit")
	}
}

// --- PhaseSwarmPlan -------------------------------------------------------

// TestPhaseSwarmPlan_ValidAndStringer names the PhaseSwarmPlan const and
// exercises Phase.IsValid()/String() over it: swarm-plan is a recognized phase
// and stringifies to its wire value.
func TestPhaseSwarmPlan_ValidAndStringer(t *testing.T) {
	t.Parallel()
	if PhaseSwarmPlan.String() != "swarm-plan" {
		t.Errorf("PhaseSwarmPlan.String()=%q, want swarm-plan", PhaseSwarmPlan.String())
	}
	if !PhaseSwarmPlan.IsValid() {
		t.Error("PhaseSwarmPlan must be a valid Phase const")
	}
}

// --- VerdictReason --------------------------------------------------------

// TestVerdictReason_FromDiagnostics names VerdictReason and exercises
// ReasonFromDiagnostics: a FAIL with an error diagnostic folds into a
// VerdictReason carrying the error message as Summary and the supplied Taxonomy.
func TestVerdictReason_FromDiagnostics(t *testing.T) {
	t.Parallel()
	tax := Taxonomy{Source: "audit", FailureMode: "egps-red", Consequence: failureadapter.CodeAuditFail}
	var vr VerdictReason = ReasonFromDiagnostics(VerdictFAIL, []Diagnostic{
		{Severity: "warning", Message: "minor"},
		{Severity: "error", Message: "EGPS: red_count=3"},
	}, tax)

	if vr.Status != VerdictFAIL {
		t.Errorf("Status=%q, want FAIL", vr.Status)
	}
	if vr.Summary != "EGPS: red_count=3" {
		t.Errorf("Summary=%q, want the first error diagnostic", vr.Summary)
	}
	if vr.Taxonomy != tax {
		t.Errorf("Taxonomy=%+v, want %+v", vr.Taxonomy, tax)
	}
	if vr.IsPass() {
		t.Error("IsPass must be false for a FAIL verdict")
	}
}

// --- Orchestrator.FailureAdviserWired -------------------------------------

// TestOrchestrator_FailureAdviserWired covers the FailureAdviserWired method
// (named AND executed >0%): true when the WithFailureAdviser option injected an
// adviser, false on a bare orchestrator.
func TestOrchestrator_FailureAdviserWired(t *testing.T) {
	t.Parallel()
	bare := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	if bare.FailureAdviserWired() {
		t.Error("bare orchestrator must report FailureAdviserWired()=false")
	}
	wired := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil),
		WithFailureAdviser(&fakeAdviser{advice: &recovery.FailureAdvice{Cause: "dead_shell", Justification: "j"}}))
	if !wired.FailureAdviserWired() {
		t.Error("orchestrator with WithFailureAdviser must report FailureAdviserWired()=true")
	}
}

// --- PhaseBoundaryCheckpointer --------------------------------------------

// TestPhaseBoundaryCheckpointer_FiresDuringRunCycle names the package var
// PhaseBoundaryCheckpointer and proves the real consumer (cyclerun_record.go's
// recordAndBranch) invokes it at every phase boundary. core cannot import the
// checkpoint package that normally sets it (circular import), so the test
// assigns its own recording closure, runs a full RunCycle, and asserts the hook
// fired with the just-completed phases. The saved value is restored in
// t.Cleanup. NOT parallel: the var is package-global and shared with the other
// RunCycle tests in this package.
func TestPhaseBoundaryCheckpointer_FiresDuringRunCycle(t *testing.T) {
	saved := PhaseBoundaryCheckpointer
	t.Cleanup(func() { PhaseBoundaryCheckpointer = saved })

	var fired int
	var lastCS CycleState
	PhaseBoundaryCheckpointer = func(cs CycleState, projectRoot string, _ time.Time) error {
		fired++
		lastCS = cs
		return nil
	}

	o := NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 9}}, &fakeLedger{}, buildRunners(nil))
	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p", GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	// The hook fires once per completed phase boundary.
	if fired != len(res.PhasesRun) {
		t.Errorf("PhaseBoundaryCheckpointer fired %d time(s), want one per phase (%d)", fired, len(res.PhasesRun))
	}
	if fired == 0 {
		t.Fatal("PhaseBoundaryCheckpointer never fired during a full RunCycle")
	}
	// It is handed the live cycle-state, carrying the completed phases.
	if len(lastCS.CompletedPhases) == 0 {
		t.Error("checkpointer received an empty CycleState — expected the completed phases")
	}
	if lastCS.CycleID != 10 {
		t.Errorf("checkpointer CycleState.CycleID=%d, want 10", lastCS.CycleID)
	}
}
