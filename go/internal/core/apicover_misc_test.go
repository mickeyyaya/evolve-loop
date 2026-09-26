package core

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/failureadapter"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

func TestFailureAdvisorOption_EffectReachesBridge(t *testing.T) {
	t.Parallel()
	fb := &fakeBridge{stdout: `{"cause":"dead_shell","pane_substr":"locked vault prompt here","justification":"the REPL exited to a locked vault"}`}

	var cliOpt FailureAdvisorOption = WithFailureAdvisorCLI("codex-tmux")
	var modelOpt FailureAdvisorOption = WithFailureAdvisorModel("gpt-5.5")
	var adv *FailureAdvisor = NewFailureAdvisor(fb, cliOpt, modelOpt)

	if _, err := adv.Advise(context.Background(), baseFailureInput()); err != nil {
		t.Fatalf("Advise: %v", err)
	}
	if fb.gotReq.CLI != "codex-tmux" {
		t.Errorf("WithFailureAdvisorCLI ineffective: BridgeRequest.CLI=%q, want codex-tmux", fb.gotReq.CLI)
	}
	if fb.gotReq.Model != "gpt-5.5" {
		t.Errorf("WithFailureAdvisorModel ineffective: BridgeRequest.Model=%q, want gpt-5.5", fb.gotReq.Model)
	}
}

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

var _ Observer = (*recordingObserver)(nil)
var _ Observer = noopObserver{}

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

var _ StateUpdater = (*memUpdater)(nil)

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

var _ WorktreeProvisioner = (*fakeWorktree)(nil)
var _ WorktreeProvisioner = gitWorktree{}

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

func TestThroughputRecorder_FiresAndMutatesState(t *testing.T) {
	t.Parallel()
	var gotCycle int
	var gotWs string
	var rec ThroughputRecorder = func(state *State, cycle int, workspacePath string) {
		gotCycle = cycle
		gotWs = workspacePath
		state.LastCycleNumber = cycle
	}

	st := &State{LastCycleNumber: 0}
	rec(st, 99, "/tmp/ws")
	if gotCycle != 99 || gotWs != "/tmp/ws" {
		t.Errorf("recorder got (cycle=%d, ws=%q), want (99, /tmp/ws)", gotCycle, gotWs)
	}
	if st.LastCycleNumber != 99 {
		t.Errorf("recorder must mutate state in place: LastCycleNumber=%d, want 99", st.LastCycleNumber)
	}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithThroughputRecorder(rec))
	if !o.ThroughputRecorderWired() {
		t.Error("WithThroughputRecorder did not wire the recorder")
	}
}

func TestSealResult_FullFieldFromDryRun(t *testing.T) {
	t.Parallel()
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

func TestStateMachine_NextAndCanTransition(t *testing.T) {
	t.Parallel()
	var sm *StateMachine = NewStateMachine()

	nxt, err := sm.Next(PhaseAudit, VerdictPASS)
	if err != nil {
		t.Fatalf("Next(audit, PASS): %v", err)
	}
	if nxt != PhaseShip {
		t.Errorf("Next(audit, PASS)=%s, want ship", nxt)
	}
	if nxt, _ := sm.Next(PhaseAudit, VerdictFAIL); nxt != PhaseRetro {
		t.Errorf("Next(audit, FAIL)=%s, want retro", nxt)
	}
	if !sm.CanTransition(PhaseBuild, PhaseAudit) {
		t.Error("CanTransition(build, audit) must be legal")
	}
	if sm.CanTransition(PhaseBuild, PhaseShip) {
		t.Error("CanTransition(build, ship) must be illegal — build cannot skip audit")
	}
}

func TestPhaseSwarmPlan_ValidAndStringer(t *testing.T) {
	t.Parallel()
	if PhaseSwarmPlan.String() != "swarm-plan" {
		t.Errorf("PhaseSwarmPlan.String()=%q, want swarm-plan", PhaseSwarmPlan.String())
	}
	if !PhaseSwarmPlan.IsValid() {
		t.Error("PhaseSwarmPlan must be a valid Phase const")
	}
}

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

// Not parallel: PhaseBoundaryCheckpointer is package-global, and core cannot import the checkpoint package that sets it.
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
	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if fired != len(res.PhasesRun) {
		t.Errorf("PhaseBoundaryCheckpointer fired %d time(s), want one per phase (%d)", fired, len(res.PhasesRun))
	}
	if fired == 0 {
		t.Fatal("PhaseBoundaryCheckpointer never fired during a full RunCycle")
	}
	if len(lastCS.CompletedPhases) == 0 {
		t.Error("checkpointer received an empty CycleState — expected the completed phases")
	}
	if lastCS.CycleID != 10 {
		t.Errorf("checkpointer CycleState.CycleID=%d, want 10", lastCS.CycleID)
	}
}

func TestOrchestrator_ModelCatalogLookupWired(t *testing.T) {
	t.Parallel()
	bare := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	if bare.ModelCatalogLookupWired() {
		t.Error("bare orchestrator must report ModelCatalogLookupWired()=false — a nil lookup makes the clamp's catalog gate a no-op")
	}
	wired := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil),
		WithModelCatalogLookup(func(cli, tier string) (string, bool) { return "opus", true }))
	if !wired.ModelCatalogLookupWired() {
		t.Error("orchestrator with WithModelCatalogLookup must report ModelCatalogLookupWired()=true")
	}
}
