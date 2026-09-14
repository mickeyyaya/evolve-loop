package verdict

// verdict_test.go — construction, the closed set, the shapes and the Judge
// contract (ADR-0103 unit 11 §6 tests 11, 14, 15, 18-20, 32-34). Every export
// is named and exercised in this package's own tests (apicover, cover-strict
// count package-local tests only). Every test injects WithSleep: no real sleep.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// The fixed bridge response the host goldens were captured with.
var bridgeResp = core.BridgeResponse{ExitCode: 81, CostUSD: 1.5, Tokens: core.TokenUsage{Input: 42}, BootMS: 7}

const (
	reportPASS     = "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n"
	challengeToken = "chal-tok-verdict-0001"
	// reportWithToken is a PASS report echoing THIS cycle's challenge token —
	// the anti-gaming anchor the ACS floor requires.
	reportWithToken = "<!-- challenge-token: " + challengeToken + " -->\n# Audit Report\n**Verdict:** PASS\n" +
		"<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\",\"schema_version\":1} -->\n"
)

func timeoutErr() error {
	return fmt.Errorf("bridge: launch exit=%d: %w", 81, core.ErrArtifactTimeout)
}

func transientErr() error {
	return fmt.Errorf("bridge: launch exit=%d: %w", 85, core.ErrTransientBridgeFailure)
}

// counts records what the injected seams saw.
type counts struct {
	verify, sleep int
}

// probe scripts the deliverable probe: not-OK with the given codes until the
// okFrom-th call (0 = never OK); err, when set, is returned on every call.
type probe struct {
	okFrom int
	codes  []string
	err    error
	empty  bool // return no bytes (an infra read fault) instead of the file's
	n      *counts
}

// verifiedFrom stamps the path and the bytes exactly as deliverable.Verify
// does (the single-read seam): the runner classifies these bytes.
func verifiedFrom(res deliverable.Result, phase string, roots phasecontract.Roots) deliverable.Result {
	path := filepath.Join(roots.Workspace, phase+"-report.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return res
	}
	res.ArtifactPath, res.Content = path, string(data)
	return res
}

func (p probe) fn() Verify {
	return func(phase string, roots phasecontract.Roots) (deliverable.Result, error) {
		p.n.verify++
		if p.err != nil {
			return deliverable.Result{}, p.err
		}
		res := deliverable.Result{OK: p.okFrom > 0 && p.n.verify >= p.okFrom}
		if !res.OK {
			for _, c := range p.codes {
				res.Violations = append(res.Violations, deliverable.Violation{Code: c, Message: "violation " + c})
			}
		}
		if p.empty {
			return res, nil
		}
		return verifiedFrom(res, phase, roots), nil
	}
}

// harness is one engine over a recording Center with counted seams.
type harness struct {
	e      *Engine
	n      *counts
	events *[]signalcenter.Event
	ws     string
	root   string
}

func newHarness(t *testing.T, p probe, opts ...Option) *harness {
	t.Helper()
	h := &harness{n: &counts{}, events: &[]signalcenter.Event{}, ws: t.TempDir(), root: t.TempDir()}
	p.n = h.n
	c := signalcenter.New()
	c.Subscribe(func(ev signalcenter.Event) { *h.events = append(*h.events, ev) })
	all := append([]Option{WithSleep(func(time.Duration) { h.n.sleep++ }), WithSignals(func() *signalcenter.Center { return c })}, opts...)
	h.e = New(p.fn(), all...)
	return h
}

func (h *harness) dispatch(phase string, bridgeErr error) Dispatch {
	return Dispatch{Cycle: 7, RunID: "run-11", Phase: phase, Workspace: h.ws, Worktree: h.ws + "-wt", ProjectRoot: h.root,
		ArtifactPath: filepath.Join(h.ws, phase+"-report.md"), Bridge: bridgeResp, BridgeErr: bridgeErr, DurationMS: 200, ResolvedModel: "opus", ModelSource: "profile"}
}

func (h *harness) writeReport(t *testing.T, phase, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(h.ws, phase+"-report.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeACSFloor stages the deterministic-truth artifacts the floor reads.
func (h *harness) writeACSFloor(t *testing.T, acsVerdict string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(h.ws, "challenge-token.txt"), []byte(challengeToken+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.ws, "acs-verdict.json"), []byte(`{"schema_version":"1.0","verdict":"`+acsVerdict+`","red_count":0,"ship_eligible":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func classifyAs(verdict, next string, diags ...core.Diagnostic) Classify {
	return func(string) (string, []core.Diagnostic, string) { return verdict, diags, next }
}

func codesOf(events []signalcenter.Event) []signalcenter.Code {
	out := make([]signalcenter.Code, 0, len(events))
	for _, e := range events {
		out = append(out, e.Code)
	}
	return out
}

// Test 11 — every code is registered under module runner with a doc.
func TestRunnerCodes_AreRegisteredWithDocsUnderModuleRunner(t *testing.T) {
	for _, c := range []signalcenter.Code{CodeTeardownFail, CodeOptionalPhaseDegraded, CodeReconciled, CodeDeliverableUnverified, CodeStdoutFilterFailed} {
		if m, ok := signalcenter.IsRegistered(c); !ok || m != signalcenter.ModuleRunner {
			t.Fatalf("%s must be registered under module runner (got %q, %v)", c, m, ok)
		}
		if !c.BelongsTo(signalcenter.ModuleRunner) {
			t.Errorf("%s must carry the RUNNER_ prefix", c)
		}
	}
	docs := signalcenter.RegisteredCodes()[signalcenter.ModuleRunner]
	if len(docs) != 5 {
		t.Fatalf("module runner registers exactly the five verdict codes, got %d", len(docs))
	}
	for _, d := range docs {
		if d.Doc == "" {
			t.Errorf("%s has no doc", d.Code)
		}
	}
}

// Test 14 — the input shape is constructed positionally so a new field breaks
// this test at compile time and the host's one projection (dispatchOf) is
// revisited instead of silently zeroing.
func TestDispatch_HasExactlyTheDeclaredFields(t *testing.T) {
	d := Dispatch{7, "run", "audit", "ws", "wt", "root", 2, "ws/audit-report.md", Snapshot{}, false, bridgeResp, nil, 200, "opus", "profile", nil}
	if d.Cycle != 7 || d.ModelSource != "profile" || d.ExplanationDocumentationVersion != 2 {
		t.Fatal("positional construction")
	}
}

// Test 15 — construction contract: a nil probe panics at first use (no guard),
// the Null Object, the live accessor, SignalsWired.
func TestNew_PanicsAtFirstUseWithoutVerify(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a nil probe is a programming error — no guard, no silent skip")
		}
	}()
	e := New(nil, WithSleep(func(time.Duration) {}))
	e.Judge(context.Background(), Dispatch{Phase: "audit"}, classifyAs(core.VerdictPASS, ""))
}

func TestWithSignals_NilIsTheNullObject(t *testing.T) {
	for _, opts := range [][]Option{nil, {WithSignals(nil)}, {WithSignals(func() *signalcenter.Center { return nil })}} {
		n := &counts{}
		e := New(probe{codes: []string{deliverable.CodeMissingArtifact}, n: n}.fn(), append(opts, WithSleep(func(time.Duration) {}))...)
		if e.SignalsWired() {
			t.Fatal("no Center ⇒ not wired")
		}
		// a provoked fault (teardown, malformed) emits into nothing and still FAILs
		resp, err := e.Judge(context.Background(), Dispatch{Phase: "audit", Workspace: t.TempDir(), BridgeErr: timeoutErr()}, classifyAs(core.VerdictPASS, ""))
		if err == nil || resp.Verdict != core.VerdictFAIL {
			t.Fatalf("the Null Object never changes the verdict: %v %+v", err, resp)
		}
	}
}

func TestWithSignals_ReadsTheCenterLive(t *testing.T) {
	var c *signalcenter.Center
	n := &counts{}
	e := New(probe{codes: []string{deliverable.CodeMissingArtifact}, n: n}.fn(), WithSleep(func(time.Duration) {}), WithSignals(func() *signalcenter.Center { return c }))
	if e.SignalsWired() {
		t.Fatal("not wired before the Center exists")
	}
	c = signalcenter.New()
	var got []signalcenter.Event
	c.Subscribe(func(ev signalcenter.Event) { got = append(got, ev) })
	if !e.SignalsWired() {
		t.Fatal("the accessor is read live, never snapshotted at construction")
	}
	e.Judge(context.Background(), Dispatch{Cycle: 3, Phase: "audit", RunID: "r", Workspace: t.TempDir(), BridgeErr: timeoutErr()}, classifyAs(core.VerdictPASS, ""))
	if len(got) != 1 || got[0].Code != CodeTeardownFail || got[0].Cycle != 3 || got[0].RunID != "r" || got[0].Phase != "audit" {
		t.Fatalf("the Center applied after construction receives the WARN with the dispatch's identity: %+v", got)
	}
}

func TestSignalsWired(t *testing.T) {
	h := newHarness(t, probe{})
	if !h.e.SignalsWired() {
		t.Fatal("a harness engine reaches its recording Center")
	}
}

// Test 18 — two cancellation policies for ONE ladder: the teardown reconcile
// runs the full window under a dead ctx (the cancel IS the teardown), the
// clean-exit path bails after the first probe.
func TestJudge_TeardownLadderIsCancellationImmune_CleanExitLadderIsNot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	teardown := newHarness(t, probe{codes: []string{deliverable.CodeMissingArtifact}})
	if _, err := teardown.e.Judge(ctx, teardown.dispatch("audit", timeoutErr()), classifyAs(core.VerdictPASS, "")); err == nil {
		t.Fatal("a never-settling mandatory teardown FAILs")
	}
	if *teardown.n != (counts{verify: 1 + SettleRetries, sleep: SettleRetries}) {
		t.Errorf("teardown ladder under a cancelled ctx: %+v, want the full window (%d probes, %d sleeps)", *teardown.n, 1+SettleRetries, SettleRetries)
	}
	clean := newHarness(t, probe{codes: []string{deliverable.CodeBadVerdict}})
	clean.writeReport(t, "audit", "partial")
	if _, err := clean.e.Judge(ctx, clean.dispatch("audit", nil), classifyAs(core.VerdictFAIL, "")); err != nil {
		t.Fatal(err)
	}
	if *clean.n != (counts{verify: 1, sleep: 0}) {
		t.Errorf("clean-exit ladder under a cancelled ctx: %+v, want one probe and no sleep", *clean.n)
	}
}

// Test 19 — no bridge error ⇒ reconcile is a pass-through (no probe, no
// event); through Judge the clean path then probes exactly once.
func TestJudge_PassThroughWhenNoBridgeError(t *testing.T) {
	h := newHarness(t, probe{okFrom: 1})
	r, early, err := h.e.reconcile(context.Background(), h.dispatch("audit", nil))
	if early != nil || err != nil || r.reconciled || r.verified.Content != "" || h.n.verify != 0 || len(*h.events) != 0 {
		t.Fatalf("reconcile without a bridge error is inert: %+v %v %v probes=%d events=%d", r, early, err, h.n.verify, len(*h.events))
	}
	h.writeReport(t, "audit", reportPASS)
	resp, err := h.e.Judge(context.Background(), h.dispatch("audit", nil), classifyAs(core.VerdictPASS, "ship"))
	if err != nil || resp.Verdict != core.VerdictPASS || resp.Reconciled || h.n.verify != 1 || len(*h.events) != 0 {
		t.Fatalf("the clean path probes once and emits nothing on the happy path: %+v %v probes=%d events=%d", resp, err, h.n.verify, len(*h.events))
	}
}

func TestJudge_SubstantiveBridgeError_FailsWithoutTheDeliverableOrTheStream(t *testing.T) {
	for _, optional := range []bool{false, true} {
		h := newHarness(t, probe{okFrom: 1}, WithOptional(optional))
		h.writeReport(t, "audit", reportPASS)
		classified := false
		resp, err := h.e.Judge(context.Background(), h.dispatch("audit", errors.New("bridge: launch exit=2")), func(string) (string, []core.Diagnostic, string) {
			classified = true
			return core.VerdictPASS, nil, ""
		})
		if err == nil || err.Error() != "audit: bridge: bridge: launch exit=2" || core.IsInfraTeardownError(err) {
			t.Fatalf("optional=%v: the substantive arm wraps the bare error: %v", optional, err)
		}
		if resp.Verdict != core.VerdictFAIL || len(resp.Diagnostics) != 1 || resp.Diagnostics[0] != (core.Diagnostic{Severity: "error", Message: "bridge: launch exit=2"}) || resp.BootMS != 7 || resp.ArtifactsDir != h.ws {
			t.Errorf("optional=%v: FAIL with the bare bridge error and the bridge's cost fields: %+v", optional, resp)
		}
		if *h.n != (counts{}) || classified || len(*h.events) != 0 {
			t.Errorf("optional=%v: never the deliverable, never Classify, never the stream (the C1 chokepoint owns this error): %+v %v %d", optional, *h.n, classified, len(*h.events))
		}
	}
}

// Test 20 — the sentinel survives the wrap on both FAIL arms.
func TestJudge_TeardownErrorChainSurvivesTheWrap(t *testing.T) {
	for _, tc := range []struct {
		err      error
		sentinel error
	}{{timeoutErr(), core.ErrArtifactTimeout}, {transientErr(), core.ErrTransientBridgeFailure}} {
		h := newHarness(t, probe{codes: []string{deliverable.CodeMissingArtifact}})
		_, err := h.e.Judge(context.Background(), h.dispatch("audit", tc.err), classifyAs(core.VerdictPASS, ""))
		if !errors.Is(err, tc.sentinel) || err.Error() != "audit: bridge: "+tc.err.Error() {
			t.Errorf("want %q wrapped under the phase prefix, got %v", tc.sentinel, err)
		}
	}
}

// Test 32 — the response base carries exactly the six shared fields.
func TestResponseBase_CarriesExactlyTheSixFields(t *testing.T) {
	h := newHarness(t, probe{})
	d := h.dispatch("audit", nil)
	want := core.PhaseResponse{Phase: "audit", ArtifactsDir: h.ws, CostUSD: 1.5, Tokens: core.TokenUsage{Input: 42}, DurationMS: 200, BootMS: 7}
	if got := responseBase(d); got.Phase != want.Phase || got.ArtifactsDir != want.ArtifactsDir || got.CostUSD != want.CostUSD || got.Tokens != want.Tokens || got.DurationMS != want.DurationMS || got.BootMS != want.BootMS ||
		got.Verdict != "" || got.NextPhase != "" || got.Diagnostics != nil || got.ModelSource != "" || got.ResolvedModel != "" || got.Reconciled {
		t.Errorf("responseBase = %+v, want the six base fields only", got)
	}
}

// Test 34 — the constants and settle bounds are named and load-bearing.
func TestAPICover_EveryExportIsNamedAndExercised(t *testing.T) {
	if SettleRetries != 15 || SettleInterval != 200*time.Millisecond {
		t.Fatalf("the settle bounds are runner.go's verbatim: %d × %s", SettleRetries, SettleInterval)
	}
	var v Verify = probe{n: &counts{}}.fn()
	var c Classify = classifyAs(core.VerdictPASS, "")
	var opt Option = WithOptional(true)
	e := New(v, opt, WithStdoutFilter(nil))
	if !e.optional || e.stdoutFilter != nil || e.sleep == nil {
		t.Fatal("options apply; the default sleep is time.Sleep; a nil filter is the Null Object")
	}
	if verdict, _, _ := c("x"); verdict != core.VerdictPASS {
		t.Fatal("Classify is the phase hook's shape")
	}
	if _, ok := StatSnapshot(filepath.Join(t.TempDir(), "absent")); ok {
		t.Fatal("StatSnapshot of an absent file")
	}
}
