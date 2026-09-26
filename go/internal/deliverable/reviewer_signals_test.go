package deliverable

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func observedReviewer(t *testing.T, stage config.Stage) (*Reviewer, *[]signalcenter.Event) {
	t.Helper()
	c := signalcenter.New()
	got := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	r := newTestReviewer(stage, filepath.Join(t.TempDir(), "b.json"), 3)
	WithSignals(c)(r)
	return r, got
}

func oneGateEvent(t *testing.T, got []signalcenter.Event, code signalcenter.Code) signalcenter.Event {
	t.Helper()
	if len(got) != 1 || got[0].Code != code || got[0].Module != signalcenter.ModuleGateContract || got[0].Origin != "Reviewer.Review" {
		t.Fatalf("one %s under module gate.contract from Reviewer.Review, got %+v", code, got)
	}
	return got[0]
}

func TestReviewerSignals_VerifiedNamesTheArtifact(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "build-report.md", "## Changes\n- x\nVerdict: PASS\n")
	r, got := observedReviewer(t, config.StageEnforce)
	in := reviewInput("build", ws, t.TempDir())
	in.Cycle, in.RunID = 42, "run-x"
	if res := r.Review(context.Background(), in); !res.Approve {
		t.Fatalf("precondition: a valid report is approved: %+v", res)
	}
	e := oneGateEvent(t, *got, "GATE_CONTRACT_VERIFIED")
	if e.Kind != signalcenter.KindGatePassed || e.Severity != signalcenter.SeverityInfo || e.Cycle != 42 || e.RunID != "run-x" || e.Phase != "build" {
		t.Fatalf("gate.passed INFO with the check's identity: %+v", e)
	}
	if e.Fields["artifact"] != "build-report.md" || e.Fields["bytes"] == "" || e.Fields["stage"] != "enforce" {
		t.Fatalf("the verification names the file it found and its size: %+v", e.Fields)
	}
}

func TestReviewerSignals_RejectedCarriesTheCodesAndTheBreakerCount(t *testing.T) {
	r, got := observedReviewer(t, config.StageEnforce)
	if res := r.Review(context.Background(), reviewInput("build", t.TempDir(), t.TempDir())); res.Approve {
		t.Fatal("precondition: a missing deliverable is blocked at enforce")
	}
	e := oneGateEvent(t, *got, "GATE_CONTRACT_REJECTED")
	if e.Kind != signalcenter.KindGateRejected || e.Severity != signalcenter.SeverityWarn || !strings.Contains(e.Fields["codes"], "missing_artifact") || e.Fields["blocks"] != "1" || e.Fields["threshold"] != "3" {
		t.Fatalf("gate.rejected WARN naming the violation codes and the breaker count: %+v", e)
	}
	if !strings.Contains(e.Reason, "build deliverable failed contract") {
		t.Fatalf("the reason is the gate's summary, the correction directive: %q", e.Reason)
	}
}

func TestReviewerSignals_ShadowIsAWouldBlock(t *testing.T) {
	r, got := observedReviewer(t, config.StageShadow)
	if res := r.Review(context.Background(), reviewInput("build", t.TempDir(), t.TempDir())); !res.Approve {
		t.Fatal("precondition: shadow approves")
	}
	e := oneGateEvent(t, *got, "GATE_CONTRACT_WOULD_BLOCK")
	if e.Kind != signalcenter.KindGatePassed || e.Severity != signalcenter.SeverityWarn || e.Fields["stage"] != "shadow" || !strings.Contains(e.Fields["codes"], "missing_artifact") {
		t.Fatalf("a shadow would-block is gate.passed WARN naming the stage and codes: %+v", e)
	}
}

func TestReviewerSignals_ReportSizeWouldBlockNamesItsOwnStage(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "build-report.md", "## Changes\n- x\n\n## Handoff Summary\n"+strings.Repeat("word ", 400)+"\nVerdict: PASS\n")
	r, got := observedReviewer(t, config.StageEnforce)
	r.reportSizeGate, r.reportSizeBudgetTokens = config.StageAdvisory, 10
	if res := r.Review(context.Background(), reviewInput("build", ws, t.TempDir())); !res.Approve {
		t.Fatalf("precondition: an advisory report-size gate approves: %+v", res)
	}
	e := oneGateEvent(t, *got, "GATE_CONTRACT_WOULD_BLOCK")
	if e.Fields["stage"] != "advisory" || e.Fields["codes"] != CodeHandoffBudgetExceeded {
		t.Fatalf("the report-size would-block names the report-size stage and its code: %+v", e.Fields)
	}
}

func TestReviewerSignals_DemotionIsAWarnPass(t *testing.T) {
	r, got := observedReviewer(t, config.StageEnforce)
	ws, pr := t.TempDir(), t.TempDir()
	for i := 0; i < 2; i++ {
		r.Review(context.Background(), reviewInput("build", ws, pr))
	}
	*got = nil
	if res := r.Review(context.Background(), reviewInput("build", ws, pr)); !res.Approve || !res.Demoted {
		t.Fatalf("precondition: the third block opens the breaker: %+v", res)
	}
	e := oneGateEvent(t, *got, "GATE_CONTRACT_DEMOTED")
	if e.Kind != signalcenter.KindGatePassed || e.Severity != signalcenter.SeverityWarn || e.Fields["blocks"] != "3" {
		t.Fatalf("a demotion is gate.passed WARN with the block count: %+v", e)
	}
}

func TestReviewerSignals_AmbiguityIsAFailOpenWarn(t *testing.T) {
	r, got := observedReviewer(t, config.StageEnforce)
	if res := r.Review(context.Background(), reviewInput("not-a-phase", t.TempDir(), t.TempDir())); !res.Approve {
		t.Fatal("precondition: ambiguity fails open")
	}
	e := oneGateEvent(t, *got, "GATE_CONTRACT_FAIL_OPEN")
	if e.Kind != signalcenter.KindGatePassed || e.Severity != signalcenter.SeverityWarn || e.Reason == "" {
		t.Fatalf("a fail-open is gate.passed WARN carrying the error: %+v", e)
	}
}

func TestReviewerSignals_SalvageIsAnInfoPass(t *testing.T) {
	const soleFencedPass = "## Verdict\n```json\n{\"phase\":\"audit\",\"verdict\":\"PASS\"}\n```\n"
	ws := t.TempDir()
	writeFile(t, ws, "audit-report.md", soleFencedPass)
	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	r, got := observedReviewer(t, config.StageEnforce)
	r.phaseIO = config.StageEnforce
	if res := r.Review(context.Background(), reviewInput("audit", ws, projectRoot)); !res.Approve {
		t.Fatalf("precondition: a sole recoverable bad_verdict salvages to approve: %+v", res)
	}
	e := oneGateEvent(t, *got, "GATE_CONTRACT_SALVAGED")
	if e.Kind != signalcenter.KindGatePassed || e.Severity != signalcenter.SeverityInfo || e.Fields["artifact"] != "audit-report.md" || e.Fields["pattern"] == "" {
		t.Fatalf("a salvage is gate.passed INFO naming the artifact and the pattern: %+v", e)
	}
}

func TestReviewerSignals_WouldSalvageUnderShadowIsAWouldBlock(t *testing.T) {
	const soleFencedPass = "## Verdict\n```json\n{\"phase\":\"audit\",\"verdict\":\"PASS\"}\n```\n"
	ws := t.TempDir()
	writeFile(t, ws, "audit-report.md", soleFencedPass)
	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	r, got := observedReviewer(t, config.StageShadow)
	r.phaseIO = config.StageEnforce
	if res := r.Review(context.Background(), reviewInput("audit", ws, projectRoot)); !res.Approve {
		t.Fatalf("precondition: shadow approves: %+v", res)
	}
	e := oneGateEvent(t, *got, "GATE_CONTRACT_WOULD_BLOCK")
	if e.Fields["salvage"] != "would" || e.Fields["stage"] != "shadow" {
		t.Fatalf("a shadow would-salvage is a would-block that says salvage would have acted: %+v", e.Fields)
	}
}

func TestReviewerSignals_OffAndUnwiredAreSilent(t *testing.T) {
	r, got := observedReviewer(t, config.StageOff)
	r.Review(context.Background(), reviewInput("build", t.TempDir(), t.TempDir()))
	if len(*got) != 0 {
		t.Fatalf("an off gate verifies nothing and reports nothing: %+v", *got)
	}
	plain := newTestReviewer(config.StageEnforce, filepath.Join(t.TempDir(), "b.json"), 3)
	if plain.SignalsWired() {
		t.Fatal("no Center: unwired (the Null Object)")
	}
	plain.Review(context.Background(), reviewInput("build", t.TempDir(), t.TempDir())) // must not panic
	if wired, _ := observedReviewer(t, config.StageEnforce); !wired.SignalsWired() {
		t.Fatal("a Center: wired")
	}
}

func TestReviewer_ExposesTheSignalsWiredCapabilityThroughTheCoreInterface(t *testing.T) {
	c := signalcenter.New()
	var rev core.DeliverableReviewer = NewReviewerWithCatalogStageReportSize(config.StageEnforce, phasespec.Catalog{}, config.StageOff, config.StageOff, 0, WithSignals(c))
	g, ok := rev.(interface {
		VerifiesDeclaredDeliverables() bool
		SignalsWired() bool
	})
	if !ok || !g.SignalsWired() || !g.VerifiesDeclaredDeliverables() {
		t.Fatalf("the production constructor accepts WithSignals and reports it: %v", ok)
	}
}

// ownedResolver overlays declared secondaries and effects onto a
// built-in contract, the way CatalogResolver overlays the registry's.
type ownedResolver struct{ owed, effects []string }

func (o ownedResolver) Resolve(name string) (phasecontract.Contract, bool) {
	c, ok := phasecontract.For(name)
	c.AgentOwedFiles, c.Effects = o.owed, o.effects
	return c, ok
}

func TestReviewerSignals_VerifiedNamesTheOwedFilesAndEffects(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "build-report.md", "## Changes\n- x\nVerdict: PASS\n")
	writeFile(t, ws, "build-extra.json", `{"ok":true}`)
	r, got := observedReviewer(t, config.StageEnforce)
	r.resolver = ownedResolver{owed: []string{"build-extra.json"}, effects: []string{"inbox-claim"}}
	in := reviewInput("build", ws, t.TempDir())
	in.Cycle = 42
	if res := r.Review(context.Background(), in); !res.Approve {
		t.Fatalf("precondition: the owed file is present and an unrecorded commitment owes no claim: %+v", res)
	}
	e := oneGateEvent(t, *got, "GATE_CONTRACT_VERIFIED")
	if e.Fields["owed"] != "build-extra.json" || e.Fields["effects"] != "inbox-claim" || !strings.Contains(e.Reason, "owed: build-extra.json") || !strings.Contains(e.Reason, "effects: inbox-claim") {
		t.Fatalf("the verification names the owed files and the effects it checked — where it searched: %+v", e)
	}
}

func TestVerify_ResultCarriesTheOwedFilesAndEffectsItChecked(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "build-report.md", "## Changes\n- x\nVerdict: PASS\n")
	writeFile(t, ws, "build-extra.json", `{"ok":true}`)
	in := reviewInput("build", ws, t.TempDir())
	in.Cycle = 42
	res, err := VerifyWithStage("build", rootsFor(in), ownedResolver{owed: []string{"build-extra.json"}, effects: []string{"inbox-claim"}}, config.StageOff)
	if err != nil || !res.OK {
		t.Fatalf("precondition: verified clean: %+v %v", res, err)
	}
	if len(res.Owed) != 1 || res.Owed[0] != "build-extra.json" || len(res.Effects) != 1 || res.Effects[0] != "inbox-claim" {
		t.Fatalf("the Result names the owed files and effects the verifier checked: owed=%v effects=%v", res.Owed, res.Effects)
	}
	plain, err := VerifyWithStage("build", rootsFor(in), ownedResolver{}, config.StageOff)
	if err != nil || len(plain.Owed) != 0 || len(plain.Effects) != 0 {
		t.Fatalf("nothing declared, nothing claimed: %+v %v", plain, err)
	}
}

func TestNewReviewerWithCatalogStageReportSize_OptionsApplyAfterTheReportSizeSettings(t *testing.T) {
	opts := []Option{func(r *Reviewer) { r.reportSizeBudgetTokens = 7 }}
	r := NewReviewerWithCatalogStageReportSize(config.StageEnforce, phasespec.Catalog{}, config.StageOff, config.StageAdvisory, 500, opts...)
	if r.reportSizeGate != config.StageAdvisory || r.reportSizeBudgetTokens != 7 {
		t.Fatalf("the constructor's report-size settings are options applied before the caller's: gate=%v budget=%d", r.reportSizeGate, r.reportSizeBudgetTokens)
	}
}

func TestVerify_OwedFileDeclaredWithADirectoryIsReadAtTheWorkspaceRoot(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "build-report.md", "## Changes\n- x\nVerdict: PASS\n")
	writeFile(t, ws, "build-extra.json", `{"ok":true}`)
	in := reviewInput("build", ws, t.TempDir())
	in.Cycle = 42
	res, err := VerifyWithStage("build", rootsFor(in), ownedResolver{owed: []string{"reports/build-extra.json"}}, config.StageOff)
	if err != nil || !res.OK {
		t.Fatalf("the file is read at OwedPath(workspace, name) — the workspace root: %+v %v", res, err)
	}
	if len(res.Owed) != 1 || res.Owed[0] != "build-extra.json" {
		t.Fatalf("recorded by the basename the gate read: %v", res.Owed)
	}
}
