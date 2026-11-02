package core

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	coverResidualFailure = "2 of 3 changed function(s) are below 100% statement coverage: REFUSED ..."
	omissionDoc          = "docs/explain/builds/cycle-2093-run.md"
	testFailure          = "./cmd/evolve: unit tests FAIL"
)

var omissionFailure = explanationdocs.FailurePrefix + "Changed Areas does not explain material path go/internal/repocontract/repocontract.go"

func coverResidualsFixture() []FloorResidual {
	return coverResiduals([]string{coverResidualFailure}, []covchanged.Func{
		{File: "cmd/evolve/cmd_prompt.go", Line: 46, Key: "cmd/evolve.promptStats"},
		{File: "cmd/evolve/cmd_prompt.go", Line: 78, Key: "cmd/evolve.collectPromptRecords"},
	}, "go")
}

func omissionResidualFixture() []FloorResidual {
	var log floorResidualLog
	log.noteOmission(explanationdocs.Omission{Path: "go/internal/repocontract/repocontract.go", Document: omissionDoc, Failure: omissionFailure})
	return log.items
}

type floorResidualCase struct {
	cr      *cycleRun
	dr      *dispatchResult
	handler *recordingRungHandler
	events  *[]signalcenter.Event
}

func floorResidualHarness(t *testing.T, stage string, res RungResult) floorResidualCase {
	t.Helper()
	center, events := captureSignals()
	wf := ladderWorkflow(policy.RetryLadderShadow)
	wf.RetryLadder.AuditFix.Stage = stage
	h := &recordingRungHandler{res: res}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithWorkflowConfig(wf), WithSignalCenter(center), WithRungHandler(retryladder.RungAuditFix, h))
	cr := &cycleRun{
		o: o, ctx: context.Background(), req: CycleRequest{ProjectRoot: t.TempDir()}, cycle: 2093,
		cs:          CycleState{CycleID: 2093, WorkspacePath: t.TempDir(), ActiveWorktree: t.TempDir()},
		retryConfig: policy.Policy{}.RetryConfig(), envSnap: map[string]string{},
	}
	dr := &dispatchResult{resp: PhaseResponse{Phase: string(PhaseBuild), Verdict: VerdictPASS}, attemptCount: 1, runner: &fakeRunner{name: "build"}}
	return floorResidualCase{cr: cr, dr: dr, handler: h, events: events}
}

func floorRejected(failures []string, residuals []FloorResidual) ReviewResult {
	return ReviewResult{Reason: floorReasonHead + strings.Join(failures, "; "), Retry: true, Failures: failures, Residuals: residuals}
}

func auditFixSources(events []signalcenter.Event) []string {
	var sources []string
	for _, e := range events {
		if e.Code == CodeLadderAuditFix {
			sources = append(sources, e.Fields["source"]+":"+e.Fields["audit_fix"])
		}
	}
	return sources
}

var passedFix = RungResult{Handled: true, Reason: "passed"}

func TestBuildFloorResidual_OnlyCoverageRunsTheRoundWithOnePrescriptionPerFunction(t *testing.T) {
	t.Parallel()
	c := floorResidualHarness(t, policy.RetryLadderEnforce, passedFix)

	act, err := c.cr.rejectAfterCorrections(PhaseBuild, c.dr, floorRejected([]string{coverResidualFailure}, coverResidualsFixture()), 1)

	if act != loopNext || err != nil {
		t.Fatalf("rejectAfterCorrections = %v, %v; a passed audit_fix delivers the build", act, err)
	}
	if len(c.handler.calls) != 1 {
		t.Fatalf("handler calls = %d, want 1", len(c.handler.calls))
	}
	fb := c.handler.calls[0].Failure
	want := []string{
		"add tests in go/cmd/evolve/cmd_prompt_test.go that cover cmd/evolve.promptStats (go/cmd/evolve/cmd_prompt.go:46) to 100% statement coverage",
		"add tests in go/cmd/evolve/cmd_prompt_test.go that cover cmd/evolve.collectPromptRecords (go/cmd/evolve/cmd_prompt.go:78) to 100% statement coverage",
	}
	if !slices.Equal(fb.Prescription, want) || len(fb.Defects) != 2 || !strings.HasPrefix(fb.Defects[0], "go/cmd/evolve/cmd_prompt.go:46 ") {
		t.Fatalf("failure block = %+v, want one cited prescription per uncovered function", fb)
	}
	if !slices.Equal(fb.Targets, []string{"go/cmd/evolve/cmd_prompt_test.go"}) || !slices.Equal(fb.EvidencePaths, []string{"go/cmd/evolve/cmd_prompt.go"}) {
		t.Fatalf("targets = %v, want only the test file", fb.Targets)
	}
	if !c.handler.calls[0].Choice.Choose || fb.Class != floorResidualClass {
		t.Fatalf("input = %+v, want the chosen rung on the floor residual class", c.handler.calls[0])
	}
	if got := auditFixSources(*c.events); !slices.Equal(got, []string{"build_floor:" + auditFixChosen}) {
		t.Fatalf("LADDER_AUDIT_FIX = %v, want one build_floor chosen record", got)
	}
	if len(c.cr.phaseTimings) != 0 {
		t.Fatalf("a delivered build recorded a rejected outcome: %+v", c.cr.phaseTimings)
	}
}

func TestBuildFloorResidual_OnlyChangedAreasRunsTheRoundWithTheDocumentPrescription(t *testing.T) {
	t.Parallel()
	c := floorResidualHarness(t, policy.RetryLadderEnforce, passedFix)

	act, err := c.cr.rejectAfterCorrections(PhaseBuild, c.dr, floorRejected([]string{omissionFailure, omissionFailure}, append(omissionResidualFixture(), omissionResidualFixture()...)), 1)

	if act != loopNext || err != nil || len(c.handler.calls) != 1 {
		t.Fatalf("rejectAfterCorrections = %v, %v after %d calls; want a delivered build", act, err, len(c.handler.calls))
	}
	fb := c.handler.calls[0].Failure
	want := "add a Changed Areas line for go/internal/repocontract/repocontract.go in " + omissionDoc
	if !slices.Equal(fb.Prescription, []string{want}) || !slices.Equal(fb.Defects, []string{omissionDoc + ": Changed Areas does not explain material path go/internal/repocontract/repocontract.go"}) {
		t.Fatalf("failure block = %+v, want one deduplicated prescription that cites the document", fb)
	}
	if !slices.Equal(fb.EvidencePaths, []string{omissionDoc}) || !slices.Equal(fb.Targets, []string{omissionDoc}) {
		t.Fatalf("evidence = %v, want the explanation document", fb.EvidencePaths)
	}
}

func TestBuildFloorResidual_AnUnprescribableResidualAbortsAsToday(t *testing.T) {
	t.Parallel()
	for name, rr := range map[string]ReviewResult{
		"a red test beside a coverage residual": floorRejected([]string{coverResidualFailure, testFailure}, coverResidualsFixture()),
		"a residual of another failure text":    floorRejected([]string{testFailure}, coverResidualsFixture()),
		"no floor failure":                      {Reason: "contract violation"},
	} {
		t.Run(name, func(t *testing.T) {
			c := floorResidualHarness(t, policy.RetryLadderEnforce, passedFix)

			act, err := c.cr.rejectAfterCorrections(PhaseBuild, c.dr, rr, 1)

			if act != loopAbort || err == nil || len(c.handler.calls) != 0 || len(auditFixSources(*c.events)) != 0 {
				t.Fatalf("rejectAfterCorrections = %v, %v after %d calls; want the abort of today", act, err, len(c.handler.calls))
			}
		})
	}
}

func TestBuildFloorResidual_ShadowRecordsTheDecisionAndAbortsAsToday(t *testing.T) {
	t.Parallel()
	c := floorResidualHarness(t, policy.RetryLadderShadow, RungResult{Reason: "shadow"})

	act, err := c.cr.rejectAfterCorrections(PhaseBuild, c.dr, floorRejected([]string{coverResidualFailure}, coverResidualsFixture()), 1)

	if act != loopAbort || err == nil {
		t.Fatalf("rejectAfterCorrections = %v, %v; shadow keeps the abort", act, err)
	}
	if got := auditFixSources(*c.events); !slices.Equal(got, []string{"build_floor:" + auditFixWould}) {
		t.Fatalf("LADDER_AUDIT_FIX = %v, want one build_floor would record", got)
	}
}

func TestBuildFloorResidual_AnEscalationAbortsAsToday(t *testing.T) {
	t.Parallel()
	c := floorResidualHarness(t, policy.RetryLadderEnforce, RungResult{Handled: true, Escalate: true, Reason: "escalate"})

	act, err := c.cr.rejectAfterCorrections(PhaseBuild, c.dr, floorRejected([]string{coverResidualFailure}, coverResidualsFixture()), 1)

	if act != loopAbort || err == nil || len(c.handler.calls) != 1 {
		t.Fatalf("rejectAfterCorrections = %v, %v; an escalation aborts, the build had its retry", act, err)
	}
	if e := c.cr.phaseTimings[0]; e.Verdict != VerdictFAIL {
		t.Fatalf("build outcome = %+v, want the coded FAIL of today", e)
	}
}

func TestBuildFloorResidual_AnExhaustedRungSealsTheCycleHandedOff(t *testing.T) {
	t.Parallel()
	c := floorResidualHarness(t, policy.RetryLadderEnforce, RungResult{Handled: true, Stop: true, Reason: "audit_fix_exhausted"})

	act, err := c.cr.rejectAfterCorrections(PhaseBuild, c.dr, floorRejected([]string{omissionFailure}, omissionResidualFixture()), 1)

	if act != loopBreak || err != nil {
		t.Fatalf("rejectAfterCorrections = %v, %v; want the cycle to end", act, err)
	}
	r := c.cr.result
	if r.FinalVerdict != VerdictFAIL || r.TerminationReason != CycleTerminationHandedOff {
		t.Fatalf("result = %s %s, want a handed-off FAIL", r.FinalVerdict, r.TerminationReason)
	}
	if e := c.cr.phaseTimings[0]; !strings.Contains(e.AbortReason, "audit_fix_exhausted") || !strings.Contains(e.AbortReason, c.cr.cs.ActiveWorktree) {
		t.Fatalf("abort reason = %q, want audit_fix_exhausted and the salvage worktree", e.AbortReason)
	}
}

func TestBuildFloorResidual_NoRoundRunsWithoutAHandlerOrOutsideTheBuild(t *testing.T) {
	t.Parallel()
	c := floorResidualHarness(t, policy.RetryLadderEnforce, passedFix)
	c.cr.o.rungHandlers = nil

	if act, err := c.cr.rejectAfterCorrections(PhaseBuild, c.dr, floorRejected([]string{coverResidualFailure}, coverResidualsFixture()), 1); act != loopAbort || err == nil {
		t.Fatalf("no handler: %v, %v; want the abort of today", act, err)
	}

	d := floorResidualHarness(t, policy.RetryLadderEnforce, passedFix)
	if act, err := d.cr.rejectAfterCorrections(PhaseTDD, d.dr, floorRejected([]string{coverResidualFailure}, coverResidualsFixture()), 1); act != loopAbort || err == nil || len(d.handler.calls) != 0 {
		t.Fatalf("tdd phase: %v, %v; want the abort of today", act, err)
	}
}

func TestBuildFloorResidual_AHandlerErrorAbortsAsToday(t *testing.T) {
	t.Parallel()
	c := floorResidualHarness(t, policy.RetryLadderEnforce, RungResult{})
	c.handler.err = errors.New("bridge down")

	act, err := c.cr.rejectAfterCorrections(PhaseBuild, c.dr, floorRejected([]string{coverResidualFailure}, coverResidualsFixture()), 1)

	if act != loopAbort || err == nil || len(c.handler.calls) != 1 {
		t.Fatalf("rejectAfterCorrections = %v, %v; a handler error keeps the abort", act, err)
	}
}

func TestReviewDeliverable_CarriesTheTypedResidualsOfItsOwnReview(t *testing.T) {
	t.Parallel()
	residual := omissionResidualFixture()[0]
	c := floorResidualHarness(t, policy.RetryLadderEnforce, passedFix)
	c.cr.o.reviewer = residualFloor(residual)
	in := ReviewInputFor(c.cr.cs, PhaseBuild, "")

	first := c.cr.reviewDeliverable(PhaseBuild, in, contractDispatch{})
	second := c.cr.reviewDeliverable(PhaseBuild, in, contractDispatch{})

	if !slices.Equal(first.Residuals, []FloorResidual{residual}) || !slices.Equal(second.Residuals, []FloorResidual{residual}) {
		t.Fatalf("residuals = %+v then %+v, want only the residuals of each review", first.Residuals, second.Residuals)
	}
}

func TestCoverResiduals_NamesEachRefusedFunctionOnlyWhenTheFloorRefuses(t *testing.T) {
	t.Parallel()
	refused := []covchanged.Func{{File: "internal/swarm/reap_orphans.go", Line: 181, Key: "internal/swarm.reap"}}

	if got := coverResiduals(nil, refused, "go"); len(got) != 0 {
		t.Fatalf("shadow residuals = %+v, want none when the floor does not refuse", got)
	}
	got := coverResiduals([]string{"refusal"}, refused, ".")
	want := FloorResidual{Class: FloorResidualCoverChanged, Failure: "refusal", Source: "internal/swarm/reap_orphans.go", Line: 181, Func: "internal/swarm.reap"}
	if !slices.Equal(got, []FloorResidual{want}) {
		t.Fatalf("residuals = %+v, want %+v", got, want)
	}
}

func TestFloorResidualLog_ANilLogIgnoresResiduals(t *testing.T) {
	t.Parallel()
	var log *floorResidualLog
	log.add(FloorResidual{Failure: "x"})
	log.noteOmission(explanationdocs.Omission{Path: "a.go"})
	if got := log.list(); got != nil {
		t.Fatalf("nil log list = %+v", got)
	}
}

func TestModuleRepoPrefix_IsTheModuleDirectoryRelativeToTheWorktree(t *testing.T) {
	t.Parallel()
	if got := moduleRepoPrefix("/wt", "/wt/go"); got != "go" {
		t.Fatalf("prefix = %q, want go", got)
	}
	if got := moduleRepoPrefix("/wt", "relative"); got != "." {
		t.Fatalf("prefix of an unrelatable module = %q, want .", got)
	}
}

func TestAuditFixRound_RecordsTheAuditSource(t *testing.T) {
	t.Parallel()
	cr, events := auditFixRun(t, policy.RetryLadderShadow, prescribedSmallFix)

	cr.auditFixRound(PhaseBuild, repairGrant, nil)

	if got := auditFixSources(*events); !slices.Equal(got, []string{"audit:" + auditFixWould}) {
		t.Fatalf("LADDER_AUDIT_FIX = %v, want one audit source record", got)
	}
}

func TestBuildFloorResidual_ManyFilesAndOmissionsStayInsideTheDefaultBounds(t *testing.T) {
	t.Parallel()
	c := floorResidualHarness(t, policy.RetryLadderEnforce, passedFix)
	var log floorResidualLog
	for _, p := range []string{"go/internal/a/a.go", "go/internal/b/b.go", "go/internal/c/c.go", "go/internal/d/d.go", "go/internal/e/e.go"} {
		log.noteOmission(explanationdocs.Omission{Path: p, Document: omissionDoc, Failure: "omits " + p})
	}
	cover := coverResiduals([]string{coverResidualFailure}, []covchanged.Func{
		{File: "internal/a/a.go", Line: 3, Key: "internal/a.A"},
		{File: "internal/b/b.go", Line: 9, Key: "internal/b.B"},
	}, "go")
	var failures []string
	for _, r := range log.items {
		failures = append(failures, r.Failure)
	}

	act, err := c.cr.rejectAfterCorrections(PhaseBuild, c.dr, floorRejected(append(failures, coverResidualFailure), append(log.items, cover...)), 1)

	if act != loopNext || err != nil || len(c.handler.calls) != 1 || !c.handler.calls[0].Choice.Choose {
		t.Fatalf("rejectAfterCorrections = %v, %v; calls %+v; want a chosen round inside the default bounds", act, err, c.handler.calls)
	}
	want := []string{omissionDoc, "go/internal/a/a_test.go", "go/internal/b/b_test.go"}
	if got := c.handler.calls[0].Failure.Targets; !slices.Equal(got, want) {
		t.Fatalf("targets = %v, want %v", got, want)
	}
}

func TestFloorResidualBlock_DropsAResidualWhoseFailureTheReviewDidNotReturn(t *testing.T) {
	t.Parallel()
	rs := append(omissionResidualFixture(), coverResidualsFixture()...)

	fb, ok := floorResidualBlock(floorRejected([]string{omissionFailure}, rs))

	if !ok || len(fb.Prescription) != 1 || !slices.Equal(fb.EvidencePaths, []string{omissionDoc}) {
		t.Fatalf("block = %+v, want only the omission prescription", fb)
	}
}

func TestFloorResidual_TheFloorReviewKeepsTheFailureTextOfEachResidual(t *testing.T) {
	t.Parallel()
	residual := omissionResidualFixture()[0]
	c := floorResidualHarness(t, policy.RetryLadderEnforce, passedFix)
	c.cr.o.reviewer = residualFloor(residual)
	in := ReviewInputFor(c.cr.cs, PhaseBuild, "")
	in.Protected = func(string) bool { return true }

	rr := c.cr.reviewDeliverable(PhaseBuild, in, contractDispatch{})

	if fb, ok := floorResidualBlock(rr); rr.Approve || !ok || len(fb.Prescription) != 1 {
		t.Fatalf("review = %+v, block ok %v; the floor must keep the failure text that the residual names", rr, ok)
	}
}

func residualFloor(residual FloorResidual) BuildHandoffFloor {
	return BuildHandoffFloor{{Name: "residual", Run: func(_ context.Context, in ReviewInput) []string {
		in.residuals.add(residual)
		return []string{residual.Failure}
	}}}
}

func TestFloorResidualBlock_AnUnknownClassIsNotPrescribable(t *testing.T) {
	t.Parallel()
	unknown := FloorResidual{Class: "apicover", Failure: "apicover refused", Source: "go/x.go"}

	if fb, ok := floorResidualBlock(floorRejected([]string{unknown.Failure}, []FloorResidual{unknown})); ok {
		t.Fatalf("block = %+v, want no block for an unknown class", fb)
	}
}

func TestFloorResidualClass_OnlyTheTwoTypedClassesArePrescribable(t *testing.T) {
	t.Parallel()
	omission := FloorResidual{Class: FloorResidualChangedAreas, Document: "docs/explain/builds/c.md", Path: "go/internal/x/x.go"}
	got, ok := omission.prescribe()
	if !ok || got.target != omission.Document || got.text != "add a Changed Areas line for go/internal/x/x.go in docs/explain/builds/c.md" {
		t.Fatalf("Changed Areas omission = %+v, %v; want a docs-only prescription that targets the explanation document", got, ok)
	}
	if _, ok := (FloorResidual{Class: FloorResidualClass("size_ratchet")}).prescribe(); ok {
		t.Fatal("an unknown residual class must not be prescribable, so the build aborts as today")
	}
}
