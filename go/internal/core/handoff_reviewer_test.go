package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const handoffTestLedger = `{"schema_version":1,"pair":"tdd-build","request_digest":"sha256:req","entries":[{"ask":"a1","status":"met","evidence":[],"note":""}]}`

type scriptedHandoffCheck struct {
	checks []handoff.Check
	pairs  []string
}

func (s *scriptedHandoffCheck) check(_ ReviewInput, pair string) handoff.Check {
	s.pairs = append(s.pairs, pair)
	next := s.checks[0]
	if len(s.checks) > 1 {
		s.checks = s.checks[1:]
	}
	return next
}

func handoffStages(stage string) policy.HandoffConfig {
	cfg := policy.Policy{}.HandoffConfig()
	for name := range cfg {
		cfg[name] = policy.HandoffPairConfig{Stage: stage}
	}
	return cfg
}

func readHandoffCheckFile(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return doc
}

func decodeHandoffCheck(t *testing.T, doc map[string]json.RawMessage) handoff.Check {
	t.Helper()
	var c handoff.Check
	if err := json.Unmarshal(doc["check"], &c); err != nil {
		t.Fatalf("check record: %v (%s)", err, doc["check"])
	}
	return c
}

type ledgerWritingBuild struct {
	fakeRunner
}

func (b *ledgerWritingBuild) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	if err := os.WriteFile(filepath.Join(req.Workspace, "handoff-tdd-build.json"), []byte(handoffTestLedger), 0o644); err != nil {
		return PhaseResponse{}, err
	}
	return b.fakeRunner.Run(ctx, req)
}

func TestHandoffReviewer_AtShadowACycleWritesTheCheckAndKeepsItsVerdict(t *testing.T) {
	t.Parallel()
	script := &scriptedHandoffCheck{checks: []handoff.Check{{Outcome: "retry", Unmet: []string{"a2"}, FailedEvidence: []string{`predicate "TestX"`}}}}
	runners := buildRunners(nil)
	runners[PhaseBuild] = &ledgerWritingBuild{fakeRunner{name: string(PhaseBuild)}}
	root := t.TempDir()
	reviewer := NewHandoffReviewer(policy.Policy{}.HandoffConfig(), script.check)

	res, err := NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 9}}, &fakeLedger{}, runners, WithReviewer(reviewer)).
		RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "goal"})

	if err != nil || res.FinalVerdict != VerdictPASS {
		t.Fatalf("result = %+v err=%v; a check at stage shadow records and never changes the verdict", res, err)
	}
	if strings.Join(script.pairs, ",") != "triage-tdd,tdd-build" {
		t.Errorf("checked pairs = %v; the reviewer checks the pair that ends at each boundary", script.pairs)
	}
	doc := readHandoffCheckFile(t, filepath.Join(root, ".evolve", "runs", "cycle-10", "handoff-tdd-build.json"))
	if !strings.Contains(string(doc["entries"]), `"a1"`) {
		t.Errorf("the ledger of the phase must stay as written: %s", doc["entries"])
	}
	c := decodeHandoffCheck(t, doc)
	if c.Stage != "shadow" || c.Outcome != "retry" || strings.Join(c.Unmet, ",") != "a2" {
		t.Errorf("check = %+v; want the shadow stage and the check outcome", c)
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "runs", "cycle-10", "handoff-triage-tdd.json")); err != nil {
		t.Errorf("a boundary with no ledger still records its check: %v", err)
	}
}

func TestHandoffReviewer_AtEnforceABlockedCheckEndsTheCycleBlockedAndKeepsTheSalvageWorktree(t *testing.T) {
	t.Parallel()
	root := writeClaimableInbox(t, 1)
	repo := gittest.Fixture(t)
	wt := &fakeWorktree{path: repo.Dir}
	runners := buildRunners(nil)
	runners[PhaseTriage] = triageDecisionRunner{verdict: VerdictPASS, decision: oneCardDecision}
	build := &fakeRunner{name: string(PhaseBuild)}
	runners[PhaseBuild] = build
	script := &scriptedHandoffCheck{checks: []handoff.Check{{Outcome: "pass"}, {Outcome: "blocked", Blocked: []string{"a1", "a2"}, Paths: []string{breakerPath}}}}

	res, err := NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 9}}, &fakeLedger{}, runners,
		WithWorktreeProvisioner(wt), WithReviewer(NewHandoffReviewer(handoffStages("enforce"), script.check))).
		RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "lane-goal", Env: laneEnv})

	if err != nil {
		t.Fatalf("RunCycle: %v; a proven handoff blocker is a planned ending", err)
	}
	if res.TerminationReason != CycleTerminationBlockedNeedsConsole || !IsProvenBlockerResult(res) {
		t.Fatalf("result = %+v; want the blocked-needs-console ending", res)
	}
	if build.calls != 1 {
		t.Errorf("build dispatched %d times; a blocked handoff spends no correction round", build.calls)
	}
	if len(wt.cleaned) != 0 {
		t.Errorf("the salvage worktree must be kept: cleaned=%v", wt.cleaned)
	}
	entries, err := phasetiming.Read(filepath.Join(root, ".evolve", "runs", "cycle-10"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("phase timing: %v", err)
	}
	last := entries[len(entries)-1]
	if last.Phase != string(PhaseBuild) || len(last.Diagnostics) == 0 || last.Diagnostics[0].Code != cyclestate.DiagCodeBlockerRoute || last.Diagnostics[0].Subject != laneScopedItem {
		t.Fatalf("last timing entry = %+v", last)
	}
	for _, want := range []string{"blocked: needs console", "tdd-build", "a1, a2", breakerPath} {
		if !strings.Contains(last.Diagnostics[0].Message, want) {
			t.Errorf("the revision must name %q: %q", want, last.Diagnostics[0].Message)
		}
	}
}

func TestHandoffReviewer_AtEnforceTheRetryCorrectionListsTheUnmetAsksAndTheFailedEvidence(t *testing.T) {
	t.Parallel()
	runners := buildRunners(nil)
	build := &fakeRunner{name: string(PhaseBuild)}
	runners[PhaseBuild] = build
	failed := `evidence does not resolve: predicate "go/acs/cycle10/predicates_test.go:TestC10_002"`
	script := &scriptedHandoffCheck{checks: []handoff.Check{
		{Outcome: "pass"},
		{Outcome: "retry", Unmet: []string{"a2"}, FailedEvidence: []string{failed}},
		{Outcome: "pass"},
	}}

	res, err := NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 9}}, &fakeLedger{}, runners,
		WithReviewer(NewHandoffReviewer(handoffStages("enforce"), script.check))).
		RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "goal"})

	if err != nil || res.FinalVerdict != VerdictPASS {
		t.Fatalf("result = %+v err=%v; a met retry round passes", res, err)
	}
	if len(build.requests) != 2 {
		t.Fatalf("build dispatched %d times; want one correction round", len(build.requests))
	}
	directive := build.requests[1].CorrectionDirective
	for _, want := range []string{"tdd-build", "a2", failed} {
		if !strings.Contains(directive, want) {
			t.Errorf("the correction prompt must name %q:\n%s", want, directive)
		}
	}
}

func TestHandoffReviewer_AProvenHandoffBlockerWithoutOneCommittedItemKeepsTheRounds(t *testing.T) {
	t.Parallel()
	root := writeClaimableInbox(t, 1)
	runners := buildRunners(nil)
	runners[PhaseTriage] = triageDecisionRunner{verdict: VerdictPASS, decision: `{"top_n":[{"id":"claimable-lane-item"},{"id":"other-item"}]}`}
	build := &fakeRunner{name: string(PhaseBuild)}
	runners[PhaseBuild] = build
	script := &scriptedHandoffCheck{checks: []handoff.Check{{Outcome: "pass"}, {Outcome: "blocked", Blocked: []string{"a1"}}}}

	res, _ := NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 9}}, &fakeLedger{}, runners,
		WithReviewer(NewHandoffReviewer(handoffStages("enforce"), script.check))).
		RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "goal"})

	if res.TerminationReason == CycleTerminationBlockedNeedsConsole {
		t.Errorf("result = %+v; the console route names exactly one item", res)
	}
	if build.calls < 2 {
		t.Fatalf("build dispatched %d times; a refused route keeps the correction rounds", build.calls)
	}
	if directive := build.requests[1].CorrectionDirective; !strings.Contains(directive, "a1") || !strings.Contains(directive, "tdd-build") {
		t.Errorf("the correction after a refused route must name the blocked ask and the pair:\n%s", directive)
	}
}

func TestHandoffReviewer_APhaseThatEndsNoPairIsNotChecked(t *testing.T) {
	t.Parallel()
	script := &scriptedHandoffCheck{checks: []handoff.Check{{Outcome: "blocked"}}}
	got := NewHandoffReviewer(handoffStages("enforce"), script.check).Review(context.Background(), ReviewInput{Phase: string(PhaseAudit), Workspace: t.TempDir()})
	if !got.Approve || len(script.pairs) != 0 {
		t.Errorf("result = %+v pairs=%v; audit ends no declared pair", got, script.pairs)
	}
}

func TestHandoffReviewer_AtEnforceEachNonPassOutcomeRejects(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{"retry", "cap", "invalid", "error"} {
		ws := t.TempDir()
		script := &scriptedHandoffCheck{checks: []handoff.Check{{Outcome: outcome, Unmet: []string{"a1"}, Errors: []string{"missing entry: a1"}}}}
		got := NewHandoffReviewer(handoffStages("enforce"), script.check).Review(context.Background(), ReviewInput{Phase: string(PhaseTDD), Workspace: ws})
		if got.Approve || !got.Retry || got.ProvenBlocker != "" {
			t.Errorf("%s: result = %+v; want a retry rejection", outcome, got)
		}
		for _, want := range []string{"triage-tdd", outcome, "a1", "missing entry: a1"} {
			if !strings.Contains(got.Reason+got.Remediation, want) {
				t.Errorf("%s: the rejection must name %q: %q / %q", outcome, want, got.Reason, got.Remediation)
			}
		}
	}
}

func TestHandoffReviewer_AnUnwritableRecordIsReportedAndTheShadowVerdictStands(t *testing.T) {
	t.Parallel()
	cases := map[string]func(t *testing.T, ws string) string{
		"a ledger that is not JSON": func(t *testing.T, ws string) string {
			return writeHandoffLedgerBody(t, ws, "not json")
		},
		"a ledger that is JSON null": func(t *testing.T, ws string) string {
			return writeHandoffLedgerBody(t, ws, "null")
		},
		"a ledger path that is a directory": func(t *testing.T, ws string) string {
			if err := os.Mkdir(filepath.Join(ws, "handoff-tdd-build.json"), 0o755); err != nil {
				t.Fatal(err)
			}
			return ""
		},
		"a read-only workspace": func(t *testing.T, ws string) string {
			if err := os.Chmod(ws, 0o555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(ws, 0o755) })
			return ""
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ws := t.TempDir()
			kept := setup(t, ws)
			var stderr strings.Builder
			script := &scriptedHandoffCheck{checks: []handoff.Check{{Outcome: "invalid"}}}
			got := NewHandoffReviewer(policy.Policy{}.HandoffConfig(), script.check).Review(context.Background(), ReviewInput{Phase: string(PhaseBuild), Workspace: ws, Stderr: &stderr})
			if !got.Approve {
				t.Errorf("result = %+v; stage shadow approves", got)
			}
			if !strings.Contains(stderr.String(), "WARN handoff tdd-build") {
				t.Errorf("stderr = %q; a record that cannot be written is reported", stderr.String())
			}
			if kept == "" {
				return
			}
			if raw, _ := os.ReadFile(filepath.Join(ws, "handoff-tdd-build.json")); string(raw) != kept {
				t.Errorf("a ledger that is not a JSON object is kept as written: %q", raw)
			}
		})
	}
}

func writeHandoffLedgerBody(t *testing.T, ws, body string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(ws, "handoff-tdd-build.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestOrchestrator_HandoffReviewerWired(t *testing.T) {
	t.Parallel()
	bare := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	if bare.HandoffReviewerWired() {
		t.Error("an orchestrator with no reviewer has no handoff reviewer")
	}
	wired := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil),
		WithReviewer(ChainReviewers(noopReviewer{}, NewHandoffReviewer(policy.Policy{}.HandoffConfig(), (&scriptedHandoffCheck{}).check))))
	if !wired.HandoffReviewerWired() {
		t.Error("a chain that holds the handoff reviewer reports it")
	}
}

func TestHandoffReviewer_TheCheckRecordKeepsEachLedgerValueVerbatim(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeHandoffLedgerBody(t, ws, `{"schema_version":1,"pair":"tdd-build","round":12345678901234567891}`)
	script := &scriptedHandoffCheck{checks: []handoff.Check{{Outcome: "pass"}}}
	NewHandoffReviewer(policy.Policy{}.HandoffConfig(), script.check).Review(context.Background(), ReviewInput{Phase: string(PhaseBuild), Workspace: ws})
	doc := readHandoffCheckFile(t, filepath.Join(ws, "handoff-tdd-build.json"))
	if string(doc["round"]) != "12345678901234567891" {
		t.Errorf("round = %s; the merge must not round a ledger value", doc["round"])
	}
	if c := decodeHandoffCheck(t, doc); c.Outcome != "pass" || c.Stage != "shadow" {
		t.Errorf("check = %+v", c)
	}
}

func resumeAtTDDWithHandoff(t *testing.T, decision string, checks ...handoff.Check) (CycleResult, error, *fakeRunner, *fakeWorktree) {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-9")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(decision), 0o644); err != nil {
		t.Fatal(err)
	}
	wt := &fakeWorktree{path: t.TempDir()}
	st := &fakeStorage{state: State{LastCycleNumber: 9}, cycleState: CycleState{CycleID: 9, WorkspacePath: ws, ActiveWorktree: wt.path}}
	runners := buildRunners(nil)
	tdd := &fakeRunner{name: string(PhaseTDD)}
	runners[PhaseTDD] = tdd
	script := &scriptedHandoffCheck{checks: checks}
	o := NewOrchestrator(st, &fakeLedger{}, runners, WithWorktreeProvisioner(wt),
		WithReviewer(NewHandoffReviewer(handoffStages("enforce"), script.check)))
	res, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"}, &ResumePoint{Phase: string(PhaseTDD), CycleID: 9})
	return res, err, tdd, wt
}

func TestHandoffReviewer_AResumedPhaseEndsBlockedOnAProvenHandoffBlocker(t *testing.T) {
	t.Parallel()
	res, err, tdd, wt := resumeAtTDDWithHandoff(t, oneCardDecision, handoff.Check{Outcome: "blocked", Blocked: []string{"a1"}, Paths: []string{breakerPath}})

	if err != nil {
		t.Fatalf("RunCycleFromPhase: %v; a proven handoff blocker is a planned ending on resume too", err)
	}
	if res.TerminationReason != CycleTerminationBlockedNeedsConsole || !IsProvenBlockerResult(res) {
		t.Fatalf("result = %+v; want the blocked-needs-console ending", res)
	}
	if tdd.calls != 1 {
		t.Errorf("tdd dispatched %d times; a blocked handoff spends no correction round", tdd.calls)
	}
	if len(wt.cleaned) != 0 {
		t.Errorf("the salvage worktree must be kept: cleaned=%v", wt.cleaned)
	}
	if got := (resumedBlocker{diag: Diagnostic{Message: "blocked: needs console"}}).Error(); got != "blocked: needs console" {
		t.Errorf("Error() = %q; the resumed blocker reads as its revision", got)
	}
}

func TestHandoffReviewer_AResumedRefusedRouteKeepsTheCorrectionRounds(t *testing.T) {
	t.Parallel()
	twoCards := `{"top_n":[{"id":"claimable-lane-item"},{"id":"other-item"}]}`
	res, _, tdd, _ := resumeAtTDDWithHandoff(t, twoCards, handoff.Check{Outcome: "blocked", Blocked: []string{"a1"}}, handoff.Check{Outcome: "pass"})

	if res.TerminationReason == CycleTerminationBlockedNeedsConsole {
		t.Errorf("result = %+v; the console route names exactly one item", res)
	}
	if tdd.calls != 2 {
		t.Errorf("tdd dispatched %d times; a refused route spends a correction round", tdd.calls)
	}
}

func TestHandoffReviewer_GivesItsHandoffCheckerTheReviewInputAndThePairThePhaseEnds(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	var gotWorkspace, gotPair string
	var check HandoffChecker = func(in ReviewInput, pair string) handoff.Check {
		gotWorkspace, gotPair = in.Workspace, pair
		return handoff.Check{Outcome: string(handoff.ExitPass)}
	}
	var reviewer HandoffReviewer = NewHandoffReviewer(handoffStages("enforce"), check)

	got := reviewer.Review(context.Background(), ReviewInput{Phase: string(PhaseBuild), Workspace: ws})

	if !got.Approve {
		t.Errorf("a passing check must approve, got %+v", got)
	}
	if gotPair != request.PairTDDBuild || gotWorkspace != ws {
		t.Errorf("checker got pair %q workspace %q, want %q %q", gotPair, gotWorkspace, request.PairTDDBuild, ws)
	}
}
