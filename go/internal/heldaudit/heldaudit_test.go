package heldaudit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/heldproposal"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

var modelFamilies = map[string]string{"claude": "claude", "agy": "gemini", "claude-tmux": "claude", "agy-claude-tmux": "claude", "agy-tmux": "gemini", "codex-tmux": "gpt"}

func modelFamily(cli string) string { return modelFamilies[cli] }

type harness struct {
	dir        string
	rec        heldproposal.Record
	candidates []string
	floor      string
	resolveErr error
	seatCalls  int
	seatReq    SeatRequest
	outcome    SeatOutcome
	seatErr    error
}

func newHarness(t *testing.T, builders ...string) *harness {
	t.Helper()
	h := &harness{dir: t.TempDir(), candidates: []string{"agy-tmux", "claude-tmux", "agy-claude-tmux"},
		outcome: SeatOutcome{Verdict: "PASS", Report: "# Verdict\nPASS\n", RunLabel: "run-1", LaunchRecord: "rec-1", CLI: "agy-tmux", Tier: "deep"}}
	diff := []byte("diff --git a b\n")
	h.rec = heldproposal.Record{Cycle: 7, Status: heldproposal.StatusHeld, Tree: "tree-a", Reasons: []string{"protected:x"}, DiffSHA256: heldproposal.DigestOf(diff), BuilderFamilies: builders}
	if err := heldproposal.WriteRecord(h.dir, h.rec); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(heldproposal.AuditDiffPath(h.dir, 7), diff, 0o644); err != nil {
		t.Fatal(err)
	}
	return h
}

func (h *harness) deps() Deps {
	return Deps{
		Seat: func(_ context.Context, req SeatRequest) (SeatOutcome, error) {
			h.seatCalls++
			h.seatReq = req
			return h.outcome, h.seatErr
		},
		Resolve: func(key string) (cliroute.Decision, error) {
			return cliroute.Decision{Plan: llmroute.Plan{Candidates: h.candidates}}, h.resolveErr
		},
		Floor:       func(string) string { return h.floor },
		ModelFamily: modelFamily,
		Now:         func() time.Time { return time.Unix(10, 0) },
	}
}

func (h *harness) dispatcher() Dispatcher { return Dispatcher{Deps: h.deps(), EvolveDir: h.dir} }

func (h *harness) dispatch(t *testing.T) error {
	t.Helper()
	return h.dispatcher().Dispatch(context.Background(), t.TempDir(), h.rec)
}

func (h *harness) report(t *testing.T) heldproposal.AuditReport {
	t.Helper()
	r, _, found, err := heldproposal.ReadAuditReport(h.dir, 7)
	if err != nil || !found {
		t.Fatalf("no audit report: %v %v", found, err)
	}
	return r
}

func TestDispatch_AGeminiAuditPassesAClaudeBuildOnTheHeldRouteKey(t *testing.T) {
	t.Parallel()
	h := newHarness(t, "claude")

	if err := h.dispatch(t); err != nil {
		t.Fatal(err)
	}

	r := h.report(t)
	if r.Verdict != heldproposal.AuditPass || r.Family != "gemini" || r.Tier != "deep" || r.RunLabel != "run-1" {
		t.Errorf("report = %+v, want a PASS from gemini at deep", r)
	}
	if h.seatReq.Route != cliroute.HeldAuditKey || len(h.seatReq.Files) != 1 || !strings.Contains(h.seatReq.Input, "protected:x") {
		t.Errorf("seat request = %+v, want the held route key, the diff and the hold reasons", h.seatReq)
	}
	rec, body, _, _ := heldproposal.ReadAuditReport(h.dir, 7)
	if verr := heldproposal.VerifyAudit(h.rec, rec, true, body); verr != nil {
		t.Errorf("the written report fails the shared verifier: %v", verr)
	}
}

func TestDispatch_TheAdmissionRefusesEveryBuilderFamilyCandidateAndEveryTierBelowDeep(t *testing.T) {
	t.Parallel()
	h := newHarness(t, "claude")
	if err := h.dispatch(t); err != nil {
		t.Fatal(err)
	}

	admit := h.seatReq.Admit
	for cli, want := range map[string]bool{"claude-tmux": false, "agy-claude-tmux": false, "agy-tmux": true, "codex-tmux": true, "unknown-tmux": false} {
		if got := admit(cli, "deep"); got != want {
			t.Errorf("admit(%s, deep) = %v, want %v", cli, got, want)
		}
	}
	if admit("agy-tmux", "balanced") || admit("agy-tmux", "fast") || !admit("agy-tmux", "top") {
		t.Error("the admission must refuse tiers below deep and keep deep and top")
	}
}

func TestDispatch_AGeminiBuildTakesAClaudeFamilyAudit(t *testing.T) {
	t.Parallel()
	h := newHarness(t, "gemini")
	h.candidates = []string{"agy-tmux", "claude-tmux"}
	h.outcome.CLI = "claude-tmux"

	if err := h.dispatch(t); err != nil {
		t.Fatal(err)
	}

	if r := h.report(t); r.Verdict != heldproposal.AuditPass || r.Family != "claude" {
		t.Errorf("report = %+v, want a PASS from claude", r)
	}
}

func TestDispatch_AnAuditThatRanOnTheBuilderFamilyIsRefusedAsTheExtraAudit(t *testing.T) {
	t.Parallel()
	h := newHarness(t, "claude")
	h.outcome.CLI = "agy-claude-tmux"

	err := h.dispatch(t)

	if !errors.Is(err, heldproposal.ErrAuditBuilderFamily) {
		t.Fatalf("err = %v, want ErrAuditBuilderFamily", err)
	}
	r, body, _, _ := heldproposal.ReadAuditReport(h.dir, 7)
	if r.Verdict == heldproposal.AuditPass || !strings.Contains(r.Reason, "built the change") || heldproposal.VerifyAudit(h.rec, r, true, body) == nil {
		t.Errorf("report = %+v, want a non-PASS report that the verifier refuses", r)
	}
	r.Verdict = heldproposal.AuditPass
	if !errors.Is(heldproposal.VerifyAudit(h.rec, r, true, body), heldproposal.ErrAuditBuilderFamily) {
		t.Error("a forged PASS of the builder family must still fail on the family")
	}
}

func TestDispatch_WaitsAndNeverLaunchesWhenNoRouteIsIndependent(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		setup  func(h *harness)
		reason string
	}{
		"only builder families":      {func(h *harness) { h.candidates = []string{"claude-tmux", "agy-claude-tmux"} }, heldproposal.WaitNoIndependentRoute},
		"a floor holds the key":      {func(h *harness) { h.candidates, h.floor = []string{"claude-tmux"}, "claude" }, heldproposal.WaitFloorExcludesOtherFamilies},
		"the route does not resolve": {func(h *harness) { h.resolveErr = errors.New("no table") }, heldproposal.WaitNoIndependentRoute},
		"an unattributed hold":       {func(h *harness) { h.rec.BuilderFamilies = nil }, heldproposal.WaitBuilderUnattributed},
		"a missing diff":             {func(h *harness) { _ = os.Remove(heldproposal.AuditDiffPath(h.dir, 7)) }, heldproposal.WaitDiffMissing},
		"a changed diff":             {func(h *harness) { _ = os.WriteFile(heldproposal.AuditDiffPath(h.dir, 7), []byte("other"), 0o644) }, heldproposal.WaitDiffMismatch},
		"a failed seat":              {func(h *harness) { h.seatErr = errors.New("bridge down") }, heldproposal.WaitSeatFailed},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, "claude")
			tc.setup(h)

			_ = h.dispatch(t)

			r := h.report(t)
			if r.Verdict != heldproposal.AuditWait || !strings.HasPrefix(r.Reason, tc.reason) {
				t.Errorf("report = %+v, want a WAIT with reason %s", r, tc.reason)
			}
			if tc.reason != heldproposal.WaitSeatFailed && h.seatCalls != 0 {
				t.Errorf("seat launched %d times, want 0 on a wait", h.seatCalls)
			}
		})
	}
}

func TestDispatch_AFailVerdictIsAReportWithItsBodyNotAnError(t *testing.T) {
	t.Parallel()
	h := newHarness(t, "claude")
	h.outcome.Verdict, h.outcome.Report = "FAIL", "# Findings\nCRITICAL: gate weakened\n"

	if err := h.dispatch(t); err != nil {
		t.Fatalf("a FAIL verdict is a report: %v", err)
	}

	_, body, _, _ := heldproposal.ReadAuditReport(h.dir, 7)
	if r := h.report(t); r.Verdict != heldproposal.AuditFail || !strings.Contains(string(body), "gate weakened") {
		t.Errorf("report = %+v body %q, want a FAIL report with its findings", r, body)
	}
}

func TestDispatch_AWarnVerdictIsNotAPass(t *testing.T) {
	t.Parallel()
	h := newHarness(t, "claude")
	h.outcome.Verdict = "WARN"

	if err := h.dispatch(t); err != nil {
		t.Fatal(err)
	}

	if r := h.report(t); r.Verdict != heldproposal.AuditFail {
		t.Errorf("verdict = %s, want FAIL: only PASS passes", r.Verdict)
	}
}

func TestDispatch_ADecidedReportOfTheSameTreeIsKeptAndAWaitIsRetried(t *testing.T) {
	t.Parallel()
	h := newHarness(t, "claude")
	if err := h.dispatch(t); err != nil {
		t.Fatal(err)
	}
	if err := h.dispatch(t); err != nil || h.seatCalls != 1 {
		t.Fatalf("second dispatch err %v seat calls %d, want the PASS kept with one launch", err, h.seatCalls)
	}

	h.rec.Tree = "tree-b"
	if err := h.dispatch(t); err != nil || h.seatCalls != 2 {
		t.Errorf("another tree: err %v seat calls %d, want a new launch", err, h.seatCalls)
	}

	w := newHarness(t, "claude")
	w.candidates = []string{"claude-tmux"}
	_ = w.dispatch(t)
	w.candidates = []string{"agy-tmux"}
	if err := w.dispatch(t); err != nil || w.seatCalls != 1 || w.report(t).Verdict != heldproposal.AuditPass {
		t.Errorf("after a wait: err %v calls %d, want the retry to launch and pass", err, w.seatCalls)
	}
}

func TestDispatch_AReportReadFailureIsReturned(t *testing.T) {
	t.Parallel()
	h := newHarness(t, "claude")
	if err := os.WriteFile(heldproposal.AuditReportPath(h.dir, 7), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := h.dispatch(t); err == nil || h.seatCalls != 0 {
		t.Errorf("err %v calls %d, want a loud read error and no launch", err, h.seatCalls)
	}
}

func TestRecordWait_WritesATypedWaitReport(t *testing.T) {
	t.Parallel()
	h := newHarness(t, "claude")

	if err := h.dispatcher().RecordWait(h.rec, heldproposal.WaitPortNotWired, "no port"); err != nil {
		t.Fatal(err)
	}

	if r := h.report(t); r.Verdict != heldproposal.AuditWait || !strings.HasPrefix(r.Reason, heldproposal.WaitPortNotWired) {
		t.Errorf("report = %+v", r)
	}
}

func TestDispatch_AnAttributionThatCannotBeRecordedIsAWaitNotALaunch(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ws := t.TempDir()
	writeCalls(t, ws, llmcalls.Record{Phase: "build", Cycle: 7, CLI: "agy-tmux", TS: "t"})
	if err := os.Remove(filepath.Join(heldproposal.HeldDir(h.dir), "cycle-7.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(heldproposal.HeldDir(h.dir), "cycle-7.json"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := h.dispatcher().Dispatch(context.Background(), ws, h.rec)

	if err != nil || h.seatCalls != 0 || h.report(t).Verdict != heldproposal.AuditWait {
		t.Errorf("err %v calls %d, want a WAIT and no launch when the families cannot be recorded", err, h.seatCalls)
	}
}

func TestDispatch_ARecordedAttributionIsKeptOnTheHold(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ws := t.TempDir()
	writeCalls(t, ws, llmcalls.Record{Phase: "build", Cycle: 7, CLI: "claude-tmux", TS: "t"})

	if err := h.dispatcher().Dispatch(context.Background(), ws, h.rec); err != nil {
		t.Fatal(err)
	}

	got, _, _ := heldproposal.ReadRecord(h.dir, 7)
	if len(got.BuilderFamilies) != 1 || got.BuilderFamilies[0] != "claude" || h.report(t).Family != "gemini" {
		t.Errorf("record families %v report %+v, want claude recorded and a gemini audit", got.BuilderFamilies, h.report(t))
	}
}

func TestPlanFor_NamesTheRouteItWouldLaunchOrTheWaitReasonAndLaunchesNothing(t *testing.T) {
	t.Parallel()
	h := newHarness(t, "claude")
	plan := PlanFor(h.deps(), t.TempDir(), h.rec)
	if plan.Wait != "" || plan.CLI != "agy-tmux" || plan.Family != "gemini" || plan.Tier != "deep" {
		t.Errorf("plan = %+v, want the agy route at deep", plan)
	}

	h.candidates = []string{"claude-tmux"}
	if plan := PlanFor(h.deps(), t.TempDir(), h.rec); plan.Wait != heldproposal.WaitNoIndependentRoute || plan.CLI != "" {
		t.Errorf("plan = %+v, want a no_independent_route wait", plan)
	}
	h.floor = "claude"
	if plan := PlanFor(h.deps(), t.TempDir(), h.rec); plan.Wait != heldproposal.WaitFloorExcludesOtherFamilies {
		t.Errorf("plan = %+v, want a floor wait", plan)
	}
	h.resolveErr = errors.New("no table")
	if plan := PlanFor(h.deps(), t.TempDir(), h.rec); plan.Wait != heldproposal.WaitNoIndependentRoute || !strings.Contains(plan.Detail, "no table") {
		t.Errorf("plan = %+v, want a wait that carries the resolve error, never an error return", plan)
	}
	h.rec.BuilderFamilies = nil
	if plan := PlanFor(h.deps(), t.TempDir(), h.rec); plan.Wait != heldproposal.WaitBuilderUnattributed {
		t.Errorf("plan = %+v, want a builder_unattributed wait", plan)
	}
	ws := t.TempDir()
	writeCalls(t, ws, llmcalls.Record{Phase: "build", Cycle: 7, CLI: "claude-tmux", TS: "t"})
	h.resolveErr, h.candidates = nil, []string{"agy-tmux"}
	if plan := PlanFor(h.deps(), ws, h.rec); plan.CLI != "agy-tmux" {
		t.Errorf("plan = %+v, want the attribution read from the workspace", plan)
	}
	if h.seatCalls != 0 {
		t.Errorf("seat launched %d times, want 0: a plan never launches", h.seatCalls)
	}
	if _, _, found, _ := heldproposal.ReadAuditReport(h.dir, 7); found {
		t.Error("a plan wrote a report")
	}
}

func TestDispatch_ADecidedReportOfAnotherDiffOfTheSameTreeIsRedispatched(t *testing.T) {
	t.Parallel()
	h := newHarness(t, "claude")
	if err := h.dispatch(t); err != nil || h.seatCalls != 1 {
		t.Fatalf("first dispatch err %v calls %d", err, h.seatCalls)
	}
	newDiff := []byte("diff --git a b\n+moved base\n")
	if err := os.WriteFile(heldproposal.AuditDiffPath(h.dir, 7), newDiff, 0o644); err != nil {
		t.Fatal(err)
	}
	h.rec.DiffSHA256 = heldproposal.DigestOf(newDiff)

	if err := h.dispatch(t); err != nil || h.seatCalls != 2 {
		t.Errorf("err %v calls %d, want a new launch: the kept report names another diff, and VerifyAudit would refuse it for ever", err, h.seatCalls)
	}
	if r := h.report(t); r.DiffSHA256 != h.rec.DiffSHA256 {
		t.Errorf("report names diff %s, want the new diff", r.DiffSHA256)
	}
}

func TestDispatch_ASeatOnACLIWithNoModelFamilyNeverPasses(t *testing.T) {
	t.Parallel()
	h := newHarness(t, "claude")
	h.outcome.CLI = "unknown-tmux"

	err := h.dispatch(t)

	r := h.report(t)
	if !errors.Is(err, heldproposal.ErrAuditBuilderFamily) || r.Verdict == heldproposal.AuditPass || r.Family != "" {
		t.Errorf("err %v report %+v, want a refusal and a non-PASS report for a seat with no model family", err, r)
	}
}

func TestDispatch_ASeatBelowTheDeepTierNeverPasses(t *testing.T) {
	t.Parallel()
	h := newHarness(t, "claude")
	h.outcome.Tier = "balanced"

	err := h.dispatch(t)

	if !errors.Is(err, heldproposal.ErrAuditNotDeep) || h.report(t).Verdict == heldproposal.AuditPass {
		t.Errorf("err %v report %+v, want a refusal and a non-PASS report for a seat below deep", err, h.report(t))
	}
}
