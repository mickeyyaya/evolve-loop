package core

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindow"
)

func TestReadShipError_MalformedOrCodelessIsAbsent(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeFileAt(t, filepath.Join(ws, "ship-error.json"), "{bad")
	if _, _, _, ok := readShipError(ws); ok {
		t.Fatal("malformed ship-error must read as absent")
	}
	writeFileAt(t, filepath.Join(ws, "ship-error.json"), `{"class":"c"}`)
	if _, _, _, ok := readShipError(ws); ok {
		t.Fatal("code-less ship-error must read as absent")
	}
}

func TestLoadResumeStateFrom_UnreadableStatePath(t *testing.T) {
	t.Parallel()
	_, err := loadResumeStateFrom(t.TempDir(), t.TempDir(), ResumeOptions{})
	if err == nil || !strings.Contains(err.Error(), "resume: read state") {
		t.Fatalf("err = %v", err)
	}
}

func TestActivateResumeStatePath_RestoresPriorEnvironment(t *testing.T) {
	t.Setenv(ipcenv.CycleStateFileKey, "/prior/state.json")
	restore := ActivateResumeStatePath(&ResumePoint{StatePath: "/other/state.json"}, t.TempDir())
	if got := os.Getenv(ipcenv.CycleStateFileKey); got != "/other/state.json" {
		t.Fatalf("activated = %q", got)
	}
	restore()
	if got := os.Getenv(ipcenv.CycleStateFileKey); got != "/prior/state.json" {
		t.Fatalf("restored = %q", got)
	}
}

func TestDefaultCurrentHead_GitTransportError(t *testing.T) {
	useScriptedGit(t, func(int, []string) scriptedReply { return scriptedReply{err: io.ErrUnexpectedEOF} })
	if _, err := defaultCurrentHead("/p"); err == nil {
		t.Fatal("want transport error")
	}
}

func TestDiscardMainLeak_GitFailures(t *testing.T) {
	useScriptedGit(t, func(int, []string) scriptedReply { return scriptedReply{err: io.ErrUnexpectedEOF} })
	if err := discardMainLeak(context.Background(), "/p", "f"); err == nil {
		t.Fatal("want transport error")
	}
	useScriptedGit(t, func(int, []string) scriptedReply { return scriptedReply{exit: 1} })
	if err := discardMainLeak(context.Background(), "/p", "f"); err == nil {
		t.Fatal("want exit error")
	}
}

func TestRestoreResumeGoal_TextMismatchAndDossierGoal(t *testing.T) {
	t.Parallel()
	req := CycleRequest{Context: map[string]string{"goal": "other"}}
	if _, err := restoreResumeGoal(req, CycleState{GoalText: "mine"}, State{}, true); err == nil {
		t.Fatal("differing goal text must be refused")
	}
	root := t.TempDir()
	body, err := json.Marshal(dossier.Dossier{Cycle: 4, RunID: "r", Goal: "from dossier"})
	if err != nil {
		t.Fatal(err)
	}
	writeFileAt(t, filepath.Join(dossier.CyclesDir(root), "cycle-4.json"), string(body))
	got, err := restoreResumeGoal(CycleRequest{ProjectRoot: root}, CycleState{CycleID: 4, RunID: "r"}, State{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.Context["goal"] != "from dossier" {
		t.Fatalf("goal = %q", got.Context["goal"])
	}
}

func TestOnDemandCatalogNames_SkipsControlPhases(t *testing.T) {
	t.Parallel()
	cat := catalogOf(t,
		specWith("gate", "control", phasespec.CatalogOnDemand),
		specWith("market", "plan", phasespec.CatalogOnDemand),
		specWith("always", "plan", ""),
	)
	if got := onDemandCatalogNames(cat); len(got) != 1 || got[0] != "market" {
		t.Fatalf("names = %v", got)
	}
}

func TestDegradeInsertedSuccessor_ByArchetype(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{catalog: catalogOf(t,
		specWith("p1", "plan", ""), specWith("b1", "build", ""),
		specWith("e1", "evaluate", ""), specWith("c1", "control", ""))}
	cases := map[string]Phase{"p1": PhaseBuild, "b1": PhaseAudit, "e1": PhaseAudit, "c1": PhaseShip}
	for name, want := range cases {
		if got := o.degradeInsertedSuccessor(Phase(name)); got != want {
			t.Errorf("%s -> %s, want %s", name, got, want)
		}
	}
}

func TestNextFromRoutingPlan_Edges(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if _, ok := nextFromRoutingPlan("", "scout"); ok {
		t.Fatal("blank workspace")
	}
	if _, ok := nextFromRoutingPlan(ws, "scout"); ok {
		t.Fatal("missing plan")
	}
	writeFileAt(t, filepath.Join(ws, "routing-plan.json"), "garbage")
	if _, ok := nextFromRoutingPlan(ws, "scout"); ok {
		t.Fatal("unparseable plan")
	}
	writeFileAt(t, filepath.Join(ws, "routing-plan.json"),
		`[{"phase":"scout","run":true},{"phase":"skipped","run":false},{"phase":"build","run":true}]`)
	if next, ok := nextFromRoutingPlan(ws, "scout"); !ok || next != "build" {
		t.Fatalf("skip-over next = %q %v", next, ok)
	}
	if next, ok := nextFromRoutingPlan(ws, "build"); !ok || next != PhaseEnd {
		t.Fatalf("tail next = %q %v", next, ok)
	}
	if _, ok := nextFromRoutingPlan(ws, "absent"); ok {
		t.Fatal("phase outside the plan")
	}
}

func TestParseAdjudication_Edges(t *testing.T) {
	t.Parallel()
	if parseAdjudication("", filepath.Join(t.TempDir(), "absent")) != nil {
		t.Fatal("no stdout and no artifact")
	}
	if parseAdjudication("prose without braces", filepath.Join(t.TempDir(), "absent")) != nil {
		t.Fatal("no json anywhere")
	}
}

func TestBridgeRetryAdjudicator_ProfilePath(t *testing.T) {
	t.Parallel()
	a := &bridgeRetryAdjudicator{}
	if got := a.profilePath(); got != "" {
		t.Fatalf("no root = %q", got)
	}
	a.root = "/r"
	if got := a.profilePath(); !strings.HasSuffix(got, "failure-adjudicator.json") {
		t.Fatalf("default = %q", got)
	}
	a.identity.Profile = "/custom.json"
	if got := a.profilePath(); got != "/custom.json" {
		t.Fatalf("identity profile = %q", got)
	}
}

func TestBridgeExitCode_NilIsZero(t *testing.T) {
	t.Parallel()
	if bridgeExitCode(nil) != 0 {
		t.Fatal("nil error exits 0")
	}
}

func TestComputeRetryEnvelope_CategoryWithoutPolicyRowDeclines(t *testing.T) {
	t.Parallel()
	env := computeRetryEnvelope(retryEnvelopeInput{DeclaredClass: "code-build-fail", Policy: policy.SystemFailurePolicy{}})
	if len(env.Legal) != 1 || !strings.Contains(env.Reason, "no row for category") {
		t.Fatalf("envelope = %+v", env)
	}
}

func TestCarryDemotion_InheritsSeenDemotion(t *testing.T) {
	t.Parallel()
	got := carryDemotion(ReviewResult{Approve: false}, ReviewResult{Demoted: true, Reason: "why", Blocks: 3})
	if !got.Demoted || got.Reason != "why" || got.Blocks != 3 {
		t.Fatalf("carried = %+v", got)
	}
	if got := carryDemotion(ReviewResult{Reason: "own", Blocks: 1}, ReviewResult{Demoted: true, Reason: "why", Blocks: 3}); got.Reason != "own" || got.Blocks != 1 {
		t.Fatalf("own fields must win: %+v", got)
	}
}

func TestMintRunID_IsStableLength(t *testing.T) {
	t.Parallel()
	if got := MintRunID(time.Unix(1, 0)); len(got) != 26 {
		t.Fatalf("run id %q", got)
	}
}

func TestStartRunLease_WorkspaceFailuresAreWarningsOnly(t *testing.T) {
	t.Parallel()
	stop := startRunLease(filepath.Join(fileAsDir(t), "ws"), "r", time.Now, time.Hour)
	stop()
	stop()
}

func TestRunLeaseHeartbeat_RefreshFailureWarnsAndStops(t *testing.T) {
	t.Parallel()
	tick := make(chan time.Time)
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		runLeaseHeartbeat(filepath.Join(fileAsDir(t), "ws"), runlease.Lease{RunID: "r"}, tick, time.Now, done)
		close(finished)
	}()
	tick <- time.Now()
	close(done)
	<-finished
}

func TestRunIDFromWorkspace_UnreadableRunFile(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.Mkdir(filepath.Join(ws, RunStateFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := RunIDFromWorkspace(ws); got != "" {
		t.Fatalf("run id = %q", got)
	}
}

func TestReleaseShipWindow_ReleaseFailureStillClearsLease(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	l, err := shipwindow.Acquire(context.Background(), dir, shipwindow.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	writeFileAt(t, dir, "now a file")
	cr := &cycleRun{shipLease: l}
	cr.releaseShipWindow()
	if cr.shipLease != nil {
		t.Fatal("lease must be cleared")
	}
}

func TestSolutionViolations_FallsBackToProjectRootTree(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeFileAt(t, filepath.Join(ws, "triage-decision.json"), `{"deliverable_kind":"document","top_n":[]}`)
	out := SolutionViolations(ws, "", t.TempDir(), config.DeliverableKindSpec{})
	if DocumentCycle(ws) && len(out) == 0 {
		t.Fatal("a document cycle without bound tasks must report a violation")
	}
}

func TestBoundTaskIDs_Malformed(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeFileAt(t, filepath.Join(ws, "triage-decision.json"), "{bad")
	if BoundTaskIDs(ws) != nil {
		t.Fatal("malformed triage decision binds nothing")
	}
}

func TestFloorFailReasons_PersistAndReadEdges(t *testing.T) {
	t.Parallel()
	persistFloorFailReasons(nil, PhaseAudit, nil)
	persistFloorFailReasons(&CycleState{}, PhaseAudit, nil)
	ws := t.TempDir()
	writeFileAt(t, floorFailReasonPath(ws, PhaseShip), "{bad")
	if readFloorFailReasons(ws, PhaseShip) != nil {
		t.Fatal("malformed reasons read as none")
	}
}

func TestDigestSignalFor_UnanchoredPhaseIsAbsent(t *testing.T) {
	t.Parallel()
	var sig router.RoutingSignals
	sig.Build.Present = true
	if present, _ := digestSignalFor(PhaseBuild, sig); !present {
		t.Fatal("build anchor reads its own signal")
	}
	if present, v := digestSignalFor(PhaseShip, sig); present || v != "" {
		t.Fatal("ship has no digest signal")
	}
}

func TestLegalGraphFrom_SkipsUnknownSourcePhases(t *testing.T) {
	t.Parallel()
	g := legalGraphFrom(map[string][]string{"definitely-not-a-phase": {"build"}, "scout": {"build"}})
	if len(g) != 1 {
		t.Fatalf("graph = %v", g)
	}
}

func TestUnavailablePhases_ProbeFailureAndExclusion(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{runners: map[Phase]PhaseRunner{"p": probeFailRunner{}}}
	if err := o.unavailablePhaseReason("p"); err != nil {
		t.Fatalf("non-doc probe failure keeps the phase selectable, got %v", err)
	}
	if got := withoutNames([]string{"a", "b", "c"}, []string{"b"}); len(got) != 2 || got[1] != "c" {
		t.Fatalf("withoutNames = %v", got)
	}
}

type probeFailRunner struct{}

func (probeFailRunner) Name() string { return "p" }
func (probeFailRunner) Run(context.Context, PhaseRequest) (PhaseResponse, error) {
	return PhaseResponse{}, nil
}
func (probeFailRunner) PersonaAvailable() error { return errors.New("probe exploded") }

func TestArchiveRepairPrompts_RenameFailureWarns(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	name := "build-prompt.txt"
	writeFileAt(t, filepath.Join(ws, name), "p")
	if err := os.Chmod(ws, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ws, 0o755) })
	archiveRepairPrompts(CycleState{WorkspacePath: ws, AuditRepairActive: true, AuditRepairAttempts: 1}, PhaseBuild)
}

func TestEmitPhaseOutputsSignal_UnreadableWorkspace(t *testing.T) {
	t.Parallel()
	emitPhaseOutputsSignal(filepath.Join(t.TempDir(), "absent"), 1, nil, nil)
}

func TestBackfillExhaustedArtifact_WriteFailureAndLedgerFailure(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeFileAt(t, filepath.Join(ws, "build-stdout.clean.txt"), "# Build Report\n"+strings.Repeat("x", 300))
	cr := &cycleRun{
		o:  NewOrchestrator(&fakeStorage{}, &fakeLedger{failOnAppend: true}, nil),
		cs: CycleState{WorkspacePath: ws},
	}
	cr.workflowConfig.BackfillEnabled = true
	artifact := backfillArtifactPath(ws, "build")
	if err := os.Mkdir(artifact, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, ok := cr.backfillExhaustedArtifact(PhaseBuild, ErrArtifactTimeout, 3, 3); ok {
		t.Fatal("write failure must not backfill")
	}
	if err := os.Remove(artifact); err != nil {
		t.Fatal(err)
	}
	if _, ok := cr.backfillExhaustedArtifact(PhaseBuild, ErrArtifactTimeout, 3, 3); !ok {
		t.Fatal("ledger failure is non-fatal")
	}
}

func TestMintRunID_EntropyFailureZeroesTheRandomPart(t *testing.T) {
	orig := randRead
	randRead = func(b []byte) (int, error) {
		for i := range b {
			b[i] = 0xff
		}
		return 0, errors.New("entropy exhausted")
	}
	t.Cleanup(func() { randRead = orig })
	got := MintRunID(time.Unix(1, 0))
	if len(got) != 26 {
		t.Fatalf("length %d", len(got))
	}
	if want := strings.Repeat("0", 16); got[10:] != want {
		t.Fatalf("random part %q, want zeroed %q", got[10:], want)
	}
}
