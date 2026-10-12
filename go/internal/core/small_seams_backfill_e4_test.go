package core

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func TestBridgeRetryAdjudicatorComposePrompt_LeadsWithThePersonaAndListsEveryLegalAction(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "agents", "evolve-failure-adjudicator.md"), "# Failure adjudicator persona\nJudge architecturally.")
	a := &bridgeRetryAdjudicator{root: root}
	env := retryEnvelope{Legal: []retryAction{retryActionRetryBuild, retryActionDecline}, Reason: "two rejections, same defect"}

	prompt := a.composePrompt(CycleState{CycleID: 14}, env, "/ws/retry-decision.json")

	if !strings.HasPrefix(prompt, "# Failure adjudicator persona\nJudge architecturally.\n\n---\n\n## Failure adjudication — cycle 14") {
		t.Fatalf("the persona must lead, separated from the brief:\n%s", prompt)
	}
	for _, want := range []string{"Policy envelope: two rejections, same defect", "  - retry@build\n", "  - decline\n", "Write STRICT JSON to /ws/retry-decision.json"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt lacks %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "retry@tdd") {
		t.Errorf("an action outside the envelope was offered:\n%s", prompt)
	}
}

func TestBridgeRetryAdjudicatorComposePrompt_NoPersonaAndNoLegalActionsStillBriefs(t *testing.T) {
	t.Parallel()
	for _, root := range []string{"", t.TempDir()} {
		prompt := (&bridgeRetryAdjudicator{root: root}).composePrompt(CycleState{CycleID: 2}, retryEnvelope{Reason: "halt"}, "x.json")
		if !strings.HasPrefix(prompt, "## Failure adjudication — cycle 2") || strings.Contains(prompt, "  - ") {
			t.Fatalf("root %q: prompt must start at the brief and list no action:\n%s", root, prompt)
		}
	}
}

type fixedReviewer struct{ result ReviewResult }

func (r fixedReviewer) Review(context.Context, ReviewInput) ReviewResult { return r.result }

func TestWithMandatoryExplanationReviewer_ANilNextApprovesWhatTheFloorDoesNotOwn(t *testing.T) {
	t.Parallel()
	in := ReviewInput{Phase: string(PhaseScout), Workspace: t.TempDir()}
	if got := withMandatoryExplanationReviewer(nil).Review(context.Background(), in); !got.Approve {
		t.Fatalf("a nil next must fall back to approving: %+v", got)
	}
	reject := fixedReviewer{ReviewResult{Approve: false, Reason: "scout report missing"}}
	if got := withMandatoryExplanationReviewer(reject).Review(context.Background(), in); got.Approve || got.Reason != "scout report missing" {
		t.Fatalf("a supplied next must be consulted: %+v", got)
	}
}

func TestMandatoryExplanationReviewer_AnUnsealedExplanationRejectsAndCarriesTheDemotion(t *testing.T) {
	t.Parallel()
	in := ReviewInput{
		Phase: string(PhaseBuild), ProjectRoot: t.TempDir(), Workspace: t.TempDir(), Worktree: t.TempDir(),
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
	}
	approve := fixedReviewer{ReviewResult{Approve: true}}
	demoted := fixedReviewer{ReviewResult{Approve: true, Demoted: true, Reason: "contract gate demoted", Blocks: 3}}
	plain := mandatoryExplanationReviewer{next: approve, floor: BuildHandoffFloor{}}
	carried := mandatoryExplanationReviewer{next: demoted, floor: BuildHandoffFloor{}}

	sealed := plain.Review(context.Background(), in)
	got := carried.Review(context.Background(), in)

	if sealed.Approve || sealed.Demoted {
		t.Fatalf("control: an unsealed explanation must reject undemoted: %+v", sealed)
	}
	if got.Approve || !got.Demoted || got.Reason != sealed.Reason || got.Blocks != 3 {
		t.Fatalf("the lifecycle rejection must carry the optional gate's demotion: %+v (sealed %+v)", got, sealed)
	}
}

type nilStopObserver struct{ started []string }

func (o *nilStopObserver) Start(_ context.Context, phase string, _ PhaseRequest) func() {
	o.started = append(o.started, phase)
	return nil
}

func TestObserveRouter_AnObserverWithoutACancelStillYieldsACallableStop(t *testing.T) {
	t.Parallel()
	obs := &nilStopObserver{}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithObserver(obs))

	stop := o.observeRouter(context.Background(), CycleState{WorkspacePath: t.TempDir(), RunID: "r"}, 3)

	if stop == nil {
		t.Fatal("observeRouter must never return a nil stop")
	}
	stop()
	if len(obs.started) != 1 || obs.started[0] != routerAgentLabel {
		t.Fatalf("observer starts = %v, want one router watch", obs.started)
	}
}

func TestResetFloorFailReason_ClearsOnlyThePhasesOwnReasons(t *testing.T) {
	t.Parallel()
	resetFloorFailReason(nil, PhaseShip)

	noWorkspace := &CycleState{AuditFailReasons: []string{"a"}, ShipFailReasons: []string{"s"}}
	resetFloorFailReason(noWorkspace, PhaseShip)
	if noWorkspace.ShipFailReasons != nil || len(noWorkspace.AuditFailReasons) != 1 {
		t.Fatalf("a ship reset clears only ship reasons: %+v", noWorkspace)
	}

	ws := t.TempDir()
	shipPath := floorFailReasonPath(ws, PhaseShip)
	auditPath := floorFailReasonPath(ws, PhaseAudit)
	writeFile(t, shipPath, `{"phase":"ship","reasons":["push rejected"]}`)
	if auditPath != shipPath {
		writeFile(t, auditPath, `{"phase":"ship","reasons":["push rejected"]}`)
	}
	withWorkspace := &CycleState{WorkspacePath: ws, ShipFailReasons: []string{"s"}}
	resetFloorFailReason(withWorkspace, PhaseShip)
	if _, err := os.Stat(shipPath); !os.IsNotExist(err) {
		t.Fatalf("a ship reset must remove the ship reason artifact: %v", err)
	}
	if withWorkspace.ShipFailReasons != nil {
		t.Fatalf("ship reasons survived: %+v", withWorkspace)
	}
}

func TestSeedTaskContract_ADocumentCycleWithoutARegistryContractGradesAcceptanceAlone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	item := writeItem(t, dir, "netflix-margin", `{"id":"netflix-margin","title":"Netflix margin","deliverable_kind":"document","acceptance":["two options compared"]}`)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	o.acsPredicates = func(context.Context, string, int) acsPredicates {
		return acsPredicates{names: []string{"TestShouldNotAppear"}}
	}
	cs := CycleState{CycleID: 7, WorkspacePath: documentWorkspace(t, "document")}

	block := o.seedTaskContract(context.Background(), map[string]string{"fleet_scope_paths": "netflix-margin=" + item}, PhaseBuild, cs, dir)[CtxKeyTaskContract]

	if !strings.Contains(block, "the registry declares no document contract — the audit grades the deliverable against the acceptance above alone") {
		t.Fatalf("block must say the acceptance alone is graded:\n%s", block)
	}
	for _, absent := range []string{"evolve solution check", "TestShouldNotAppear"} {
		if strings.Contains(block, absent) {
			t.Errorf("block must not carry %q without a document contract:\n%s", absent, block)
		}
	}
}

func TestTriageTermination_AnUnreadableWorkspaceFallsBackToTheVerdict(t *testing.T) {
	t.Parallel()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	missing := filepath.Join(t.TempDir(), "gone")

	if got := o.triageTermination(t.TempDir(), missing, 3, []string{"triage"}, VerdictFAIL); !got.stop || got.reason != "" {
		t.Fatalf("a FAIL with no readable signals = %+v, want a reasonless stop", got)
	}
	if got := o.triageTermination(t.TempDir(), missing, 3, []string{"triage"}, VerdictPASS); got.stop || got.reason != "" {
		t.Fatalf("a PASS with no readable signals = %+v, want no stop", got)
	}
}

func TestTriageTermination_AnEmptyCommitmentAfterTheFloorStopsWithoutANoWorkReason(t *testing.T) {
	t.Parallel()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	ws := t.TempDir()
	writeFile(t, filepath.Join(ws, "handoff-triage.json"), `{"cycle_size":"small"}`)
	writeFile(t, filepath.Join(ws, "triage-decision.json"), `{"top_n":[]}`)

	before := o.triageTermination(t.TempDir(), ws, 3, []string{"triage"}, VerdictPASS)
	after := o.triageTermination(t.TempDir(), ws, 3, []string{"triage", "audit"}, VerdictPASS)

	if !before.stop || before.reason != CycleTerminationTriageNoWork {
		t.Fatalf("control: an empty commitment before the floor = %+v, want a no-work stop", before)
	}
	if !after.stop || after.reason != "" {
		t.Fatalf("an empty commitment after the floor = %+v, want a stop with the reason cleared", after)
	}
}

func TestUnansweredClaimableWork_AnUnscannableInboxOrClaimIsAnswerlessWork(t *testing.T) {
	t.Parallel()
	cases := map[string]func(root string){
		"the inbox is a file": func(root string) {
			writeFile(t, filepath.Join(root, ".evolve", "inbox"), "x")
		},
		"the cycle claim dir is a file": func(root string) {
			writeFile(t, inboxbatch.ProcessingCycleDir(filepath.Join(root, ".evolve", "inbox"), 7), "x")
		},
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			root, ws := t.TempDir(), t.TempDir()
			writeLaneScope(t, ws, "item-a")
			if unansweredClaimableWork(root, ws, 7, withholdsEverything) {
				t.Fatal("control: an empty inbox holds no unanswered work")
			}
			arrange(root)
			if !unansweredClaimableWork(root, ws, 7, withholdsEverything) {
				t.Fatal("an inbox the check cannot scan is never proof of no work")
			}
		})
	}
}

func TestAdmittedIDs_ScanErrorsAreReturnedAndAdmittedItemsAreKeyed(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "inbox")
	writeFile(t, file, "x")
	ids, unreadable, err := admittedIDs(file, file, routingOnlyLaneMenu)
	if err == nil || ids != nil || unreadable {
		t.Fatalf("admittedIDs(file) = (%v, %v, %v), want the scan error", ids, unreadable, err)
	}

	inbox := t.TempDir()
	writeFile(t, filepath.Join(inbox, "a.json"), `{"id":"a"}`)
	writeFile(t, filepath.Join(inbox, "b.json"), `{"id":"b"}`)
	writeFile(t, filepath.Join(inbox, "bad.json"), `{not json`)
	ids, unreadable, err = admittedIDs(inbox, inbox, func(_ string, items []inboxbatch.Item) []inboxbatch.Item { return items[:1] })
	if err != nil || len(ids) != 1 || !ids["a"] || !unreadable {
		t.Fatalf("admittedIDs = (%v, %v, %v), want only the admitted id and the unreadable flag", ids, unreadable, err)
	}
}

func TestGitWorktreeCleanup_NoWorktreeIsANoOp(t *testing.T) {
	rec := &failFirstGit{}
	orig := gitRunner
	gitRunner = sysexec.RunFunc(rec.run)
	t.Cleanup(func() { gitRunner = orig })
	root := t.TempDir()
	var err error
	log := captureStderr(t, func() { err = (gitWorktree{}).Cleanup(root, "") })
	if err != nil || log != "" || len(rec.calls) != 0 {
		t.Fatalf("Cleanup of no worktree = %v, stderr %q, git calls %v; want a silent no-op", err, log, rec.calls)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("the project root must be untouched: %v", err)
	}
	if err := (gitWorktree{}).Cleanup(root, root); err == nil {
		t.Fatal("control: the project root itself is refused")
	}
}

type failFirstGit struct{ calls [][]string }

func (g *failFirstGit) run(_ context.Context, _, _ string, args, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
	g.calls = append(g.calls, append([]string(nil), args...))
	if len(g.calls) == 1 {
		return -1, errors.New("git binary vanished")
	}
	return 0, nil
}

func TestRecordAuditBinding_AFailedHeadReadStopsBeforeAnyOtherGitOrBinding(t *testing.T) {
	g := &failFirstGit{}
	orig := gitRunner
	gitRunner = sysexec.RunFunc(g.run)
	t.Cleanup(func() { gitRunner = orig })
	ws := t.TempDir()
	writeFile(t, filepath.Join(ws, "audit-report.md"), "# Audit\nPASS\n")
	ledger := &fakeLedger{}
	o := NewOrchestrator(&fakeStorage{}, ledger, buildRunners(nil))

	log := captureStderr(t, func() {
		o.recordAuditBinding(context.Background(), 5, t.TempDir(), ws, "", "", VerdictPASS)
	})

	if !strings.Contains(log, "WARN audit-binding: git rev-parse HEAD failed: ") || !strings.Contains(log, "git binary vanished") {
		t.Fatalf("stderr must name the HEAD failure:\n%s", log)
	}
	if len(g.calls) != 1 || len(ledger.entries) != 0 {
		t.Fatalf("git calls %v, ledger %+v: nothing may run or bind after the HEAD read fails", g.calls, ledger.entries)
	}
}
