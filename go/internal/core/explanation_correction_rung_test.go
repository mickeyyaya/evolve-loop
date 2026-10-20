package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func explanationReviewReport(status, document, evidence string) string {
	return "# Audit Report\n\n## " + phasecontract.ExplanationDocumentation.Title() + "\n" +
		"- Status: " + status + "\n- Build status: required\n- Document: " + document +
		"\n- Document SHA256: sha\n- Evidence: " + evidence + "\n"
}

type reviewingAuditRunner struct {
	t        *testing.T
	runs     int
	statuses []string
	verdict  string
	document string
}

func (r *reviewingAuditRunner) Name() string { return string(PhaseAudit) }

func (r *reviewingAuditRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	r.runs++
	document, err := explanationdocs.DocumentPath(req.Cycle, req.RunID)
	if err != nil {
		return PhaseResponse{}, err
	}
	r.document = document
	status := r.statuses[min(r.runs, len(r.statuses))-1]
	evidence := document + ":8 states 9 lines; the ratchet scanner measures 8 at go/internal/report.go:143"
	writeBriefFixture(r.t, req.Workspace, phasecontract.ArtifactFilename(string(PhaseAudit)), explanationReviewReport(status, document, evidence))
	verdict := r.verdict
	if verdict == "" {
		verdict = VerdictPASS
	}
	return PhaseResponse{Phase: string(PhaseAudit), Verdict: verdict, ArtifactsDir: req.Workspace}, nil
}

func runReviewedCycle(t *testing.T, audit *reviewingAuditRunner, opts ...Option) (CycleResult, map[Phase]PhaseRunner, []signalcenter.Event) {
	t.Helper()
	runners := buildRunners(map[Phase]string{PhaseRetro: VerdictFAIL})
	runners[PhaseAudit] = audit
	c := signalcenter.New()
	var got []signalcenter.Event
	c.Subscribe(func(e signalcenter.Event) { got = append(got, e) })
	o := NewOrchestrator(&fakeStorage{state: State{}}, &fakeLedger{}, runners, append([]Option{WithSignalCenter(c)}, opts...)...)
	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	return res, runners, got
}

func TestRunCycle_ANeedsCorrectionOnAVerifiedBuildCorrectsTheDocumentOnceAndShipsWithNoAuditFail(t *testing.T) {
	t.Parallel()

	audit := &reviewingAuditRunner{t: t, statuses: []string{"NEEDS_CORRECTION", "VERIFIED"}}
	res, runners, events := runReviewedCycle(t, audit)

	tdd, build, ship := runners[PhaseTDD].(*fakeRunner), runners[PhaseBuild].(*fakeRunner), runners[PhaseShip].(*fakeRunner)
	if len(tdd.requests) != 1 {
		t.Fatalf("tdd dispatched %d time(s), want 1: a document correction never re-runs tdd", len(tdd.requests))
	}
	if len(build.requests) != 2 || audit.runs != 2 {
		t.Fatalf("build %d, audit %d dispatches, want 2 and 2: one document-only Build round, re-reviewed by the audit", len(build.requests), audit.runs)
	}
	round := build.requests[1].Context
	if round[CtxKeyExplanationReauthor] != audit.document {
		t.Errorf("the correction round was not scoped to %q: %q", audit.document, round[CtxKeyExplanationReauthor])
	}
	if !strings.Contains(round[CtxKeyExplanationCorrection], audit.document+":8") {
		t.Errorf("the correction round does not name the document lines the review cited:\n%s", round[CtxKeyExplanationCorrection])
	}
	if round[CtxKeyAuditRepairFindings] != "" {
		t.Errorf("the correction round was briefed as an audit repair: %q", round[CtxKeyAuditRepairFindings])
	}
	if _, rescoped := build.requests[0].Context[CtxKeyExplanationCorrection]; rescoped {
		t.Error("the first Build was scoped to a correction no review asked for")
	}
	if len(ship.requests) != 1 || res.FinalVerdict != VerdictPASS {
		t.Fatalf("ship %d, verdict %s: the corrected cycle ships as a PASS", len(ship.requests), res.FinalVerdict)
	}
	if rungs := eventsWithCode(events, CodeExplanationCorrectionRung); len(rungs) != 1 || rungs[0].Fields["round"] != "1" || rungs[0].Fields["budget"] != "1" {
		t.Errorf("want ONE rung signal naming round 1 of budget 1, got %+v", rungs)
	}
	if repairs := eventsWithCode(events, CodeAuditRepairGranted); len(repairs) != 0 {
		t.Errorf("the correction spent an audit-repair round: %+v", repairs)
	}
}

func TestRunCycle_ANeedsCorrectionPastItsBudgetShipsTheVerifiedCodeAndSignalsTheExhaustion(t *testing.T) {
	t.Parallel()

	audit := &reviewingAuditRunner{t: t, statuses: []string{"NEEDS_CORRECTION"}}
	res, runners, events := runReviewedCycle(t, audit)

	build, ship := runners[PhaseBuild].(*fakeRunner), runners[PhaseShip].(*fakeRunner)
	if len(build.requests) != 2 || audit.runs != 2 {
		t.Fatalf("build %d, audit %d dispatches, want 2 and 2: the budget of one correction is spent once", len(build.requests), audit.runs)
	}
	if len(ship.requests) != 1 || res.FinalVerdict != VerdictPASS {
		t.Fatalf("ship %d, verdict %s: the verified code ships when the form recovery is exhausted", len(ship.requests), res.FinalVerdict)
	}
	exhausted := eventsWithCode(events, CodeExplanationCorrectionExhausted)
	if len(exhausted) != 1 || exhausted[0].Severity != signalcenter.SeverityWarn {
		t.Fatalf("want ONE WARN that the correction budget is spent, got %+v", exhausted)
	}
}

func TestRunCycle_TheCorrectionBudgetComesFromTheFailurePolicy(t *testing.T) {
	t.Parallel()

	fp := policy.DefaultSystemFailurePolicy()
	fp.Categories[policy.CategoryExplanationCorrection] = policy.FailureCategory{Level: policy.LevelTask, Action: policy.ActionRetryWithFix}
	audit := &reviewingAuditRunner{t: t, statuses: []string{"NEEDS_CORRECTION"}}
	_, runners, events := runReviewedCycle(t, audit, WithFailurePolicy(fp))

	if build := runners[PhaseBuild].(*fakeRunner); len(build.requests) != 1 || audit.runs != 1 {
		t.Fatalf("build %d, audit %d dispatches, want 1 and 1: a budget of zero runs no correction", len(build.requests), audit.runs)
	}
	if len(eventsWithCode(events, CodeExplanationCorrectionExhausted)) != 1 {
		t.Errorf("a zero budget is still reported as exhausted: %+v", events)
	}

	fp.Categories[policy.CategoryExplanationCorrection] = policy.FailureCategory{Level: policy.LevelTask, Action: policy.ActionRetryWithFix, MaxRetries: 2}
	audit = &reviewingAuditRunner{t: t, statuses: []string{"NEEDS_CORRECTION"}}
	_, runners, _ = runReviewedCycle(t, audit, WithFailurePolicy(fp))
	if build := runners[PhaseBuild].(*fakeRunner); len(build.requests) != 3 || audit.runs != 3 {
		t.Fatalf("build %d, audit %d dispatches, want 3 and 3: a budget of two runs two corrections", len(build.requests), audit.runs)
	}
}

func TestRunCycle_AFailingAuditWithANeedsCorrectionReviewTakesTheFailRouteNotTheRung(t *testing.T) {
	t.Parallel()

	audit := &reviewingAuditRunner{t: t, statuses: []string{"NEEDS_CORRECTION", "VERIFIED"}, verdict: VerdictFAIL}
	_, runners, events := runReviewedCycle(t, audit)

	if len(eventsWithCode(events, CodeExplanationCorrectionRung)) != 0 {
		t.Fatalf("a FAIL verdict names the code; it must not take the document rung: %+v", events)
	}
	if ship := runners[PhaseShip].(*fakeRunner); len(ship.requests) != 0 {
		t.Fatalf("a FAILing audit shipped %d time(s)", len(ship.requests))
	}
}

func TestRunCycle_AVerifiedReviewRunsNoCorrection(t *testing.T) {
	t.Parallel()

	audit := &reviewingAuditRunner{t: t, statuses: []string{"VERIFIED"}}
	_, runners, events := runReviewedCycle(t, audit)

	if build := runners[PhaseBuild].(*fakeRunner); len(build.requests) != 1 || audit.runs != 1 {
		t.Fatalf("build %d, audit %d dispatches, want 1 and 1", len(build.requests), audit.runs)
	}
	if len(eventsWithCode(events, CodeExplanationCorrectionRung))+len(eventsWithCode(events, CodeExplanationCorrectionExhausted)) != 0 {
		t.Errorf("a VERIFIED review emitted a correction signal: %+v", events)
	}
}

func TestRunCycle_TheDossierRecordsEachCorrection(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runners := buildRunners(map[Phase]string{PhaseRetro: VerdictFAIL})
	audit := &reviewingAuditRunner{t: t, statuses: []string{"NEEDS_CORRECTION", "VERIFIED"}}
	runners[PhaseAudit] = audit
	o := NewOrchestrator(&fakeStorage{state: State{}}, &fakeLedger{}, runners, WithDossierDestination(DossierPending))
	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "goal-x2"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dossier.PendingDir(root), fmt.Sprintf("cycle-%d.json", res.Cycle)))
	if err != nil {
		t.Fatalf("no dossier for cycle %d: %v", res.Cycle, err)
	}
	if !strings.Contains(string(raw), `"explanation_corrections"`) || !strings.Contains(string(raw), audit.document+":8 states 9 lines") {
		t.Fatalf("the dossier does not record the correction and the review that asked for it:\n%s", raw)
	}
}

func TestDecideExplanationCorrection_OnlyAPassingAuditWithANeedsCorrectionReviewAsks(t *testing.T) {
	t.Parallel()

	ws := t.TempDir()
	cs := CycleState{CycleID: correctionCycle, RunID: correctionRunID, WorkspacePath: ws}
	writeBriefFixture(t, ws, "audit-report.md", explanationReviewReport("NEEDS_CORRECTION", correctionDocument, correctionDocument+":8 is stale"))
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))

	for _, tc := range []struct {
		phase   Phase
		verdict string
	}{{PhaseAudit, VerdictFAIL}, {PhaseShip, VerdictPASS}, {PhaseTDD, VerdictPASS}} {
		probe := cs
		if next, ok := o.decideExplanationCorrection(&probe, tc.phase, tc.verdict); ok || len(probe.ExplanationCorrections) != 0 {
			t.Errorf("%s %s routed to %s; only a passing audit asks for the rung", tc.phase, tc.verdict, next)
		}
	}
	probe := cs
	if next, ok := o.decideExplanationCorrection(&probe, PhaseAudit, VerdictWARN); !ok || next != PhaseBuild || !probe.ExplanationCorrectionActive {
		t.Fatalf("a WARN audit with NEEDS_CORRECTION did not take the rung: next=%s ok=%v state=%+v", next, ok, probe)
	}
	if _, ok := o.decideExplanationCorrection(&probe, PhaseBuild, VerdictPASS); ok || probe.ExplanationCorrectionActive {
		t.Fatalf("the Build that ran the correction did not retire it: %+v", probe)
	}
	if len(probe.ExplanationCorrections) != 1 || probe.ExplanationCorrections[0] != correctionDocument+":8 is stale" {
		t.Errorf("the rung did not record the review evidence: %q", probe.ExplanationCorrections)
	}
}

func TestPendingExplanationCorrection_NeedsAReadableReviewOfTheCyclesOwnDocument(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		report string
		cs     func(ws string) CycleState
	}{
		{"no audit report", "", func(ws string) CycleState {
			return CycleState{CycleID: correctionCycle, RunID: correctionRunID, WorkspacePath: ws}
		}},
		{"no explanation section", "# Audit Report\n\nPASS\n", func(ws string) CycleState {
			return CycleState{CycleID: correctionCycle, RunID: correctionRunID, WorkspacePath: ws}
		}},
		{"a verified review", explanationReviewReport("VERIFIED", correctionDocument, correctionDocument+":8 is right"), func(ws string) CycleState {
			return CycleState{CycleID: correctionCycle, RunID: correctionRunID, WorkspacePath: ws}
		}},
		{"a cycle that names no document", explanationReviewReport("NEEDS_CORRECTION", correctionDocument, correctionDocument+":8 is stale"), func(ws string) CycleState {
			return CycleState{CycleID: correctionCycle, WorkspacePath: ws}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			if tc.report != "" {
				writeBriefFixture(t, ws, "audit-report.md", tc.report)
			}
			if review, ok := pendingExplanationCorrection(tc.cs(ws)); ok {
				t.Fatalf("asked for a correction (%+v)", review)
			}
		})
	}
}

func TestExplanationCorrectionBrief_NamesOnlyTheDocumentLines(t *testing.T) {
	t.Parallel()

	evidence := correctionDocument + ":8 states 9 lines; the scanner measures 8 at go/internal/report.go:143, and " + correctionDocument + "#L35 misnames the probe"
	brief := explanationCorrectionBrief(correctionDocument, evidence)

	for _, want := range []string{"- " + correctionDocument + ":8", "- " + correctionDocument + "#L35", evidence} {
		if !strings.Contains(brief, want) {
			t.Errorf("the brief lacks %q:\n%s", want, brief)
		}
	}
	if strings.Contains(brief, "- go/internal/report.go:143") {
		t.Errorf("the brief lists a code line as a document line to correct:\n%s", brief)
	}
	if got := explanationCorrectionBrief(correctionDocument, "the document overstates the scope"); !strings.Contains(got, "the document overstates the scope") {
		t.Errorf("a review with no line reference still briefs its evidence:\n%s", got)
	}
}

func TestDossierParams_CarryTheCyclesExplanationCorrections(t *testing.T) {
	t.Parallel()

	cr := &cycleRun{o: NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil)), cs: CycleState{ExplanationCorrections: []string{"doc.md:1 is stale"}}}
	if got := cr.dossierParams(VerdictPASS).ExplanationCorrections; len(got) != 1 || got[0] != "doc.md:1 is stale" {
		t.Fatalf("dossier params carry %q", got)
	}
}

func TestResumePath_ANeedsCorrectionOnAPassingAuditTakesTheSameRung(t *testing.T) {
	t.Parallel()

	st := &fakeStorage{state: State{}, cycleState: CycleState{CycleID: correctionCycle, RunID: correctionRunID, WorkspacePath: t.TempDir()}}
	runners := buildRunners(map[Phase]string{PhaseRetro: VerdictFAIL})
	audit := &reviewingAuditRunner{t: t, statuses: []string{"NEEDS_CORRECTION", "VERIFIED"}}
	runners[PhaseAudit] = audit
	o := NewOrchestrator(st, &fakeLedger{}, runners)

	if _, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: t.TempDir()},
		&ResumePoint{Phase: string(PhaseAudit), CycleID: correctionCycle}); err != nil {
		t.Fatalf("resume cycle: %v", err)
	}

	build := runners[PhaseBuild].(*fakeRunner)
	if len(build.requests) != 1 || audit.runs != 2 {
		t.Fatalf("build %d, audit %d dispatches, want 1 and 2: the resumed audit's NEEDS_CORRECTION runs one document-only Build", len(build.requests), audit.runs)
	}
	if !strings.Contains(build.requests[0].Context[CtxKeyExplanationCorrection], correctionDocument+":8") {
		t.Errorf("the resumed correction round does not name the document lines:\n%s", build.requests[0].Context[CtxKeyExplanationCorrection])
	}
	if tdd := runners[PhaseTDD].(*fakeRunner); len(tdd.requests) != 0 {
		t.Errorf("the resumed correction re-ran tdd %d time(s)", len(tdd.requests))
	}
}

func TestSeedExplanationCorrection_NeedsARecordedReviewAndADocument(t *testing.T) {
	t.Parallel()

	base := map[string]string{"k": "v"}
	for name, cs := range map[string]CycleState{
		"no recorded review": {CycleID: correctionCycle, RunID: correctionRunID, ExplanationCorrectionActive: true},
		"no document":        {CycleID: correctionCycle, ExplanationCorrectionActive: true, ExplanationCorrections: []string{"x.md:1 is stale"}},
	} {
		if got := seedAuditRepairContext(base, PhaseBuild, cs); len(got) != 1 {
			t.Errorf("%s: seeded %v", name, got)
		}
	}
	cs := CycleState{CycleID: correctionCycle, RunID: correctionRunID, ExplanationCorrectionActive: true, ExplanationCorrections: []string{"old", correctionDocument + ":9 is stale"}}
	if got := seedAuditRepairContext(base, PhaseBuild, CycleState{CycleID: correctionCycle, RunID: correctionRunID, ExplanationCorrectionActive: true, ExplanationCorrections: []string{correctionDocument + ":9 is stale"}, AuditRepairActive: true}); got[CtxKeyExplanationCorrection] != "" {
		t.Errorf("a granted repair round left over from a crash was briefed as a correction: %v", got)
	}
	if got := seedAuditRepairContext(base, PhaseTDD, cs); len(got) != 1 {
		t.Errorf("a TDD dispatch was seeded with the correction: %v", got)
	}
	got := seedAuditRepairContext(base, PhaseBuild, cs)
	if got[CtxKeyExplanationReauthor] != correctionDocument || !strings.Contains(got[CtxKeyExplanationCorrection], correctionDocument+":9") || strings.Contains(got[CtxKeyExplanationCorrection], "old") {
		t.Errorf("the Build is seeded with the latest review only: %v", got)
	}
}
