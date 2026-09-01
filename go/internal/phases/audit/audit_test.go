// Tests for the audit phase. Audit is the EGPS gate: PASS requires
// BOTH a parseable PASS verdict in audit-report.md AND red_count == 0
// in acs-verdict.json.
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const auditorExplanationExample = `## Explanation Documentation
- Status: VERIFIED
- Build status: required
- Document: docs/explain/builds/cycle-42-run-42.md
- Document SHA256: bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
- Evidence: docs/explain/builds/cycle-42-run-42.md:1 accurately explains the behavior implemented at config/app.yaml:1`

func TestAuditorExplanationLiteralExampleMatchesProductionReader(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "agents", "evolve-auditor-reference.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := auditContractExample(t, string(body), "audit-explanation-review")
	if got != auditorExplanationExample {
		t.Fatalf("Auditor explanation example drifted\n--- got ---\n%s\n--- want ---\n%s", got, auditorExplanationExample)
	}
	req := core.PhaseRequest{
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
		BuildExplanationState:           core.BuildExplanationAvailable,
		BuildExplanation: &phaseio.ExplanationView{
			Status: "required", DocumentPath: "docs/explain/builds/cycle-42-run-42.md",
			DocumentSHA256: strings.Repeat("b", 64), MaterialPaths: []string{"config/app.yaml"},
		},
	}
	if err := validateExplanationReview(got, req); err != nil {
		t.Fatalf("documented Auditor example rejected by production reader: %v", err)
	}
}

func auditContractExample(t *testing.T, body, name string) string {
	t.Helper()
	marker := "<!-- CONTRACT-EXAMPLE:" + name + " -->"
	start := strings.Index(body, marker)
	if start < 0 {
		t.Fatalf("missing contract example marker %s", marker)
	}
	rest := body[start+len(marker):]
	const fence = "```markdown\n"
	open := strings.Index(rest, fence)
	if open < 0 {
		t.Fatalf("%s lacks a markdown fence", marker)
	}
	rest = rest[open+len(fence):]
	close := strings.Index(rest, "\n```")
	if close < 0 {
		t.Fatalf("%s has no closing fence", marker)
	}
	return rest[:close]
}

// TestExtractHonorsPhaseContract pins audit's verdict extractor to the canonical
// heading declared in phasecontract.Audit — the single source the producer-side
// contract test (phasecontract/contract_test.go) also reads. Audit is the only
// phase whose classifier keeps its own regex (it extracts a verdict TOKEN, not
// section presence); this test ties it to the shared contract so the two cannot
// drift apart.
func TestExtractHonorsPhaseContract(t *testing.T) {
	canonical := phasecontract.Audit.Sections[0].Canonical
	got, found := extractAuditVerdict(canonical+": PASS\n", config.StageOff)
	if !found || got != core.VerdictPASS {
		t.Fatalf("extract under contract canonical %q = (%q,%v), want (PASS,true)", canonical, got, found)
	}
}

// TestExtractPrefersSentinel pins the Layer-5 strangler: when an evolve-verdict
// sentinel is present, it wins over the prose; when absent, the legacy regex
// fallback still works (backward compatible).
func TestExtractPrefersSentinel(t *testing.T) {
	// Sentinel says FAIL even though prose says PASS — sentinel must win.
	body := "## Verdict\n**PASS**\n" + phasecontract.RenderVerdictSentinel("audit", "FAIL") + "\n"
	got, found := extractAuditVerdict(body, config.StageOff)
	if !found || got != core.VerdictFAIL {
		t.Fatalf("sentinel-first: got (%q,%v), want (FAIL,true)", got, found)
	}
	// No sentinel → legacy regex still parses prose.
	got, found = extractAuditVerdict("## Verdict\n**WARN**\n", config.StageOff)
	if !found || got != core.VerdictWARN {
		t.Fatalf("regex fallback: got (%q,%v), want (WARN,true)", got, found)
	}
}

type fakeBridge struct {
	resp          core.BridgeResponse
	err           error
	writeArtifact string
	gotReq        core.BridgeRequest
}

func (f *fakeBridge) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	f.gotReq = req
	if f.writeArtifact != "" && req.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		_ = os.WriteFile(req.ArtifactPath, []byte(f.writeArtifact), 0o644)
		f.resp.Stdout = f.writeArtifact
	}
	return f.resp, f.err
}

func (f *fakeBridge) Probe(ctx context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func fakePromptsFS(body string) *prompts.Loader {
	return prompts.NewFromFS(fstest.MapFS{
		"agents/evolve-auditor.md": &fstest.MapFile{
			Data: []byte("---\nname: evolve-auditor\n---\n" + body),
		},
	})
}

// writeACSVerdict writes a verdict.json to ws/acs-verdict.json with the
// given red_count.
func writeACSVerdict(t *testing.T, ws string, redCount int) {
	t.Helper()
	v := map[string]any{
		"cycle":      42,
		"red_count":  redCount,
		"total":      10,
		"predicates": []any{},
	}
	b, _ := json.Marshal(v)
	if err := os.WriteFile(filepath.Join(ws, "acs-verdict.json"), b, 0o644); err != nil {
		t.Fatalf("write verdict: %v", err)
	}
}

func activeMissingExplanationRequest(t *testing.T) core.PhaseRequest {
	t.Helper()
	root := t.TempDir()
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-42")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	binding := explanationdocs.CycleBinding{
		ProjectRoot: root, Worktree: t.TempDir(), Workspace: workspace,
		BaseSHA: strings.Repeat("a", 40), Cycle: 42, RunID: "run-42",
		ContractVersion: explanationdocs.CurrentContractVersion,
	}
	if err := explanationdocs.Activate(binding); err != nil {
		t.Fatal(err)
	}
	if err := explanationdocs.SealBuild(binding); err != nil {
		t.Fatal(err)
	}
	return core.PhaseRequest{
		Cycle: 42, RunID: "run-42", ProjectRoot: root, Worktree: binding.Worktree,
		Workspace: workspace, WorktreeBaseSHA: binding.BaseSHA,
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
	}
}

func TestBindExplanationResult_MissingRequiredHandoffBecomesEGPSRed(t *testing.T) {
	req := activeMissingExplanationRequest(t)
	workspace := req.Workspace
	writeACSVerdict(t, workspace, 0)
	if err := verifyExplanationDocumentation(req); err == nil || !strings.Contains(err.Error(), "build-explanation.json") {
		t.Fatalf("missing explanation verification error=%v", err)
	}

	h := hooks{explanationCheck: verifyExplanationDocumentation}
	got, _, _ := h.Classify("## Verdict\n**PASS**\n", req, core.BridgeResponse{})
	if got != core.VerdictFAIL {
		t.Fatalf("deterministic explanation gate must override narrative PASS, got %s", got)
	}
	body, err := os.ReadFile(filepath.Join(workspace, "acs-verdict.json"))
	if err != nil {
		t.Fatal(err)
	}
	var verdict acssuite.Verdict
	if err := json.Unmarshal(body, &verdict); err != nil {
		t.Fatal(err)
	}
	if verdict.RedCount != 0 || len(verdict.Results) != 0 {
		t.Fatalf("native host check was mislabeled as an ACS predicate: %+v", verdict)
	}
}

func TestClassify_RequiresAuditorExplanationDocumentationReview(t *testing.T) {
	workspace := t.TempDir()
	writeACSVerdict(t, workspace, 0)
	view := &core.PhaseRequest{
		Workspace:                       workspace,
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
		BuildExplanationState:           core.BuildExplanationAvailable,
		BuildExplanation: &phaseio.ExplanationView{
			Status: "required", Reason: "material behavior changed",
			DocumentPath: "docs/explain/builds/cycle-42.md", DocumentSHA256: "document-sha",
			MaterialPaths: []string{"go/internal/example/example.go"},
		},
	}
	h := hooks{}
	if got, _, _ := h.Classify("## Verdict\n**PASS**\n", *view, core.BridgeResponse{}); got != core.VerdictFAIL {
		t.Fatalf("missing qualitative explanation review verdict=%s, want FAIL", got)
	}
	report := `## Explanation Documentation
- Status: VERIFIED
- Build status: required
- Build reason: material behavior changed
- Document: docs/explain/builds/cycle-42.md
- Document SHA256: document-sha
- Evidence: compared docs/explain/builds/cycle-42.md:1 with go/internal/example/example.go:12 in the base-bound diff

## Verdict
**PASS**
`
	if got, diags, _ := h.Classify(report, *view, core.BridgeResponse{}); got != core.VerdictPASS {
		t.Fatalf("complete qualitative explanation review verdict=%s diags=%v", got, diags)
	}
}

func TestValidateExplanationReview_RejectsTokenEvidenceAndAcceptsNegativeJudgment(t *testing.T) {
	req := core.PhaseRequest{
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
		BuildExplanationState:           core.BuildExplanationAvailable,
		BuildExplanation: &phaseio.ExplanationView{
			Status: "required", DocumentPath: "docs/explain/builds/cycle-42.md",
			DocumentSHA256: "sha", MaterialPaths: []string{"go/app.go"},
		},
	}
	weak := `## Explanation Documentation
- Status: VERIFIED
- Build status: required
- Document: docs/explain/builds/cycle-42.md
- Document SHA256: sha
- Evidence: x
`
	if err := validateExplanationReview(weak, req); err == nil || !strings.Contains(err.Error(), "concrete") {
		t.Fatalf("token evidence was accepted: %v", err)
	}
	negative := `## Explanation Documentation
- Status: NEEDS_CORRECTION
- Build status: required
- Document: docs/explain/builds/cycle-42.md
- Document SHA256: sha
- Evidence: docs/explain/builds/cycle-42.md:1 misstates the branch behavior implemented at go/app.go:19
`
	if err := validateExplanationReview(negative, req); err == nil || !strings.Contains(err.Error(), "NEEDS_CORRECTION") || strings.Contains(err.Error(), "Status must") {
		t.Fatalf("well-formed negative judgment was not preserved: %v", err)
	}
}

func TestValidateExplanationReview_RequiresPathLineEvidenceForEveryReference(t *testing.T) {
	req := core.PhaseRequest{
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
		BuildExplanationState:           core.BuildExplanationAvailable,
		BuildExplanation: &phaseio.ExplanationView{
			Status: "required", DocumentPath: "docs/explain/builds/cycle-42.md",
			DocumentSHA256: "sha", MaterialPaths: []string{"go/app.go"},
		},
	}
	report := `## Explanation Documentation
- Status: VERIFIED
- Build status: required
- Document: docs/explain/builds/cycle-42.md
- Document SHA256: sha
- Evidence: reviewed docs/explain/builds/cycle-42.md and go/app.go against the implementation
`
	if err := validateExplanationReview(report, req); err == nil || !strings.Contains(err.Error(), "path:line") {
		t.Fatalf("path-only evidence was accepted: %v", err)
	}
}

func TestValidateExplanationReview_NotApplicableRejectsDocumentClaims(t *testing.T) {
	req := core.PhaseRequest{
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
		BuildExplanationState:           core.BuildExplanationAvailable,
		BuildExplanation:                &phaseio.ExplanationView{Status: "not_applicable", Reason: "tests only"},
	}
	report := `## Explanation Documentation
- Status: VERIFIED
- Build status: not_applicable
- Document: docs/explain/builds/forged.md
- Document SHA256: forged
- Evidence: verified the base-bound diff contains no material changes
`
	if err := validateExplanationReview(report, req); err == nil || !strings.Contains(err.Error(), "must omit") {
		t.Fatalf("N/A review accepted forged document fields: %v", err)
	}
}

func TestValidateExplanationReview_InvalidPostBuildStateRequiresFail(t *testing.T) {
	req := core.PhaseRequest{
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
		BuildExplanationState:           core.BuildExplanationInvalid,
	}
	report := `## Explanation Documentation
- Status: VERIFIED
- Evidence: the host snapshot is missing
`
	if err := validateExplanationReview(report, req); err == nil || !strings.Contains(err.Error(), "Status: FAIL") {
		t.Fatalf("invalid handoff was not failed: %v", err)
	}
}

// writeACSVerdictSkip writes a verdict with both red_count and skip_count set,
// mirroring the post-SKIP-convention schema (a fresh clone produces skips).
func writeACSVerdictSkip(t *testing.T, ws string, redCount, skipCount int) {
	t.Helper()
	// Verdict is derived from red_count (PASS ⟺ red_count==0) so the fixture
	// stays internally consistent with the gate it feeds.
	verdict := "PASS"
	if redCount > 0 {
		verdict = "FAIL"
	}
	v := map[string]any{
		"cycle":      42,
		"red_count":  redCount,
		"skip_count": skipCount,
		"verdict":    verdict,
		"predicate_suite": map[string]any{
			"total":         redCount + skipCount,
			"skipped_count": skipCount,
		},
		"results": []any{
			map[string]any{"ac_id": "cycle-42/001", "result": "skip", "exit_code": 77},
		},
	}
	b, _ := json.Marshal(v)
	if err := os.WriteFile(filepath.Join(ws, "acs-verdict.json"), b, 0o644); err != nil {
		t.Fatalf("write verdict: %v", err)
	}
}

// EGPS gate keys solely off red_count: skip_count>0 with red_count==0 must PASS
// (the fresh-clone case where runtime-only predicates SKIP).
func TestRun_SkipCountWithRedZero_PASS(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdictSkip(t, ws, 0, 4)
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS (red_count==0 with skip_count=4)", resp.Verdict)
	}
}

// A genuine red alongside skips must still FAIL — SKIP cannot mask a RED.
func TestRun_RedCountWithSkipsPresent_FAIL(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdictSkip(t, ws, 2, 3)
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL (red_count=2 even with skips)", resp.Verdict)
	}
}

func TestRun_HappyPath_PASS(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	body := "# Audit Report\n\n## Verdict\n**PASS**\n\nNo defects found.\n"
	fb := &fakeBridge{writeArtifact: body, resp: core.BridgeResponse{CostUSD: 0.30}}
	clock := fixtures.FixedClock(time.Unix(1_700_000_000, 0), 60*time.Millisecond)
	phase := New(Config{
		Bridge:  fb,
		Prompts: fakePromptsFS("# Auditor body"),
		NowFn:   clock,
	})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 42, ProjectRoot: "/tmp/proj", Workspace: ws,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
	if resp.NextPhase != "ship" {
		t.Errorf("NextPhase=%q, want ship", resp.NextPhase)
	}
	if resp.DurationMS != 60 {
		t.Errorf("DurationMS=%d, want 60", resp.DurationMS)
	}
	wantArtifact := filepath.Join(ws, "audit-report.md")
	if fb.gotReq.ArtifactPath != wantArtifact {
		t.Errorf("ArtifactPath=%q", fb.gotReq.ArtifactPath)
	}
}

func TestRun_AuditPASSButRedCountNonZero_FAIL(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 3)
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL (EGPS red_count=3)", resp.Verdict)
	}
	gotEGPSDiag := false
	for _, d := range resp.Diagnostics {
		if strings.Contains(d.Message, "red_count") {
			gotEGPSDiag = true
		}
	}
	if !gotEGPSDiag {
		t.Errorf("missing red_count diagnostic; got %+v", resp.Diagnostics)
	}
}

func TestRun_AuditFAIL_FAIL(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	body := "# Audit Report\n\n## Verdict\n**FAIL**\n\nDefect: missing auth check.\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL (audit said FAIL)", resp.Verdict)
	}
}

func TestRun_AuditWARN_WARN(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	body := "# Audit Report\n\n## Verdict\n**WARN**\n\nMinor cleanup recommended.\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if resp.Verdict != core.VerdictWARN {
		t.Errorf("Verdict=%q, want WARN", resp.Verdict)
	}
}

func TestRun_StrictAuditMode_WARNBecomesFAIL(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	// Strict mode is now sourced from .evolve/policy.json (workflow.strict_audit),
	// not an env dial — replaces the EVOLVE_STRICT_AUDIT read (flag-reduction, ADR-0064).
	proj := t.TempDir()
	if err := os.MkdirAll(filepath.Join(proj, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, ".evolve", "policy.json"),
		[]byte(`{"workflow":{"strict_audit":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	body := "# Audit Report\n\n## Verdict\n**WARN**\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: proj, Workspace: ws,
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL (strict-audit promotes WARN→FAIL)", resp.Verdict)
	}
}

func TestRun_NoVerdictHeading_FAIL(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	body := "# Audit Report\n\nSome prose without a verdict heading.\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
}

func TestRun_MissingACSVerdict_FAIL(t *testing.T) {
	// No acs-verdict.json on disk = cycle cannot prove EGPS gate → FAIL.
	ws := t.TempDir()
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL (no ACS verdict file)", resp.Verdict)
	}
}

func TestRun_ACSVerdictMalformed_FAIL(t *testing.T) {
	ws := t.TempDir()
	_ = os.WriteFile(filepath.Join(ws, "acs-verdict.json"), []byte("not json"), 0o644)
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL (ACS verdict unparseable)", resp.Verdict)
	}
}

func TestRun_EmptyArtifact_FAIL(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	fb := &fakeBridge{writeArtifact: ""}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
}

func TestRun_BridgeError_FAIL(t *testing.T) {
	bridgeErr := errors.New("bridge fail")
	fb := &fakeBridge{err: bridgeErr}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
	})
	if !errors.Is(err, bridgeErr) {
		t.Errorf("err=%v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
}

func TestRun_MissingBridge_ReturnsError(t *testing.T) {
	phase := New(Config{Prompts: fakePromptsFS("body")})
	_, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1})
	if err == nil || !strings.Contains(err.Error(), "bridge required") {
		t.Fatalf("err=%v", err)
	}
}

func TestRun_MissingPrompts_ReturnsError(t *testing.T) {
	phase := New(Config{Bridge: &fakeBridge{}})
	_, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1})
	if err == nil || !strings.Contains(err.Error(), "prompts loader required") {
		t.Fatalf("err=%v", err)
	}
}

func TestRun_AgentLoadFails_ReturnsError(t *testing.T) {
	phase := New(Config{Bridge: &fakeBridge{}, Prompts: prompts.NewFromFS(fstest.MapFS{})})
	_, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1})
	if err == nil {
		t.Fatal("err=nil")
	}
}

func TestName(t *testing.T) {
	p := New(Config{})
	if p.Name() != "audit" {
		t.Errorf("Name=%q, want audit", p.Name())
	}
}

// cycle-138/139 fix: when acs-verdict.json is ABSENT, the audit phase
// generates it (via the injected GenerateVerdict seam → acssuite in prod)
// before reading red_count, so a clean autonomous cycle reaches PASS→ship
// instead of being forced to FAIL on the missing file. The generator
// stand-in here writes a red_count==0 verdict, mimicking a green suite.
func TestRun_MissingACSVerdict_GeneratedThenPASS(t *testing.T) {
	ws := t.TempDir()
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	fb := &fakeBridge{writeArtifact: body}
	genCalls := 0
	phase := New(Config{
		Bridge:  fb,
		Prompts: fakePromptsFS("body"),
		GenerateVerdict: func(req core.PhaseRequest) error {
			genCalls++
			writeACSVerdict(t, req.Workspace, 0) // green suite
			return nil
		},
	})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if genCalls != 1 {
		t.Errorf("GenerateVerdict called %d times, want 1 (file was absent)", genCalls)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS (verdict generated, red_count=0)", resp.Verdict)
	}
	if resp.NextPhase != "ship" {
		t.Errorf("NextPhase=%q, want ship", resp.NextPhase)
	}
}

// A pre-staged acs-verdict.json must be honored as-is: the generator is
// NOT invoked when the file already exists (operator/CI pre-stage path).
func TestRun_ACSVerdictPresent_GeneratorNotCalled(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	fb := &fakeBridge{writeArtifact: body}
	genCalls := 0
	phase := New(Config{
		Bridge:          fb,
		Prompts:         fakePromptsFS("body"),
		GenerateVerdict: func(core.PhaseRequest) error { genCalls++; return nil },
	})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if genCalls != 0 {
		t.Errorf("GenerateVerdict called %d times, want 0 (file pre-staged)", genCalls)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
}

// When the generator runs but produces no verdict file (e.g. zero
// predicates discovered), the missing-file FAIL floor still holds — a
// cycle with nothing to prove must NOT auto-pass.
func TestRun_GeneratorWritesNothing_FAILFloorHolds(t *testing.T) {
	ws := t.TempDir()
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{
		Bridge:          fb,
		Prompts:         fakePromptsFS("body"),
		GenerateVerdict: func(core.PhaseRequest) error { return nil }, // writes no file
	})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL (no verdict produced → floor holds)", resp.Verdict)
	}
}

// TestNewDefault_WiresVerdictGenerator is in audit_integration_test.go
// (//go:build integration) — it spawns a real `go test` subprocess.

// --- v12.1 Capability 1: phaseflags wiring tests ---

func writeAuditProfile(t *testing.T, contents string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir profiles: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "auditor.json"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return root
}

// When the wired GenerateVerdict seam returns an error (and the file stays
// absent), Classify must surface a WARNING diagnostic naming the failure and
// fall through to the missing-file FAIL floor — the generation error never
// silently passes the gate.
func TestRun_GeneratorReturnsError_WarnDiagAndFAIL(t *testing.T) {
	ws := t.TempDir()
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{
		Bridge:          fb,
		Prompts:         fakePromptsFS("body"),
		GenerateVerdict: func(core.PhaseRequest) error { return errors.New("acssuite boom") },
	})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL (generation failed → no verdict file → floor holds)", resp.Verdict)
	}
	var found bool
	for _, d := range resp.Diagnostics {
		if d.Severity == "warning" && strings.Contains(d.Message, "acs-verdict generation failed") && strings.Contains(d.Message, "acssuite boom") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a warning diagnostic naming the generation failure; got %+v", resp.Diagnostics)
	}
}

// --- generateACSVerdict (the production GenerateVerdict default) ---
// writeGoPredFixture, TestGenerateACSVerdict_EmptyWorktree_FallsBackToProjectRoot,
// and TestGenerateACSVerdict_WriteVerdictError_Propagates are in
// audit_integration_test.go (//go:build integration) — they spawn real subprocesses.

// A Cycle <= 0 makes acssuite.Run reject the request; generateACSVerdict must
// wrap and return that error rather than swallowing it.
func TestGenerateACSVerdict_SuiteRunError_Propagates(t *testing.T) {
	err := generateACSVerdict(core.PhaseRequest{
		Cycle: 0, ProjectRoot: t.TempDir(), Worktree: t.TempDir(), Workspace: t.TempDir(),
	})
	if err == nil {
		t.Fatal("err=nil, want acssuite run error (Cycle<=0)")
	}
	if !strings.Contains(err.Error(), "acssuite run") {
		t.Errorf("err=%v, want wrapped 'acssuite run'", err)
	}
}

// Zero predicates discovered → generateACSVerdict writes NOTHING and returns
// nil, leaving the audit missing-file FAIL floor to fail the cycle.
func TestGenerateACSVerdict_ZeroPredicates_WritesNothing(t *testing.T) {
	root := t.TempDir() // no acs/ dir → empty suite
	evolveDir := t.TempDir()
	ws := filepath.Join(evolveDir, "runs", "cycle-9")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir ws: %v", err)
	}

	if err := generateACSVerdict(core.PhaseRequest{
		Cycle: 9, ProjectRoot: root, Worktree: root, Workspace: ws,
	}); err != nil {
		t.Fatalf("generateACSVerdict: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(ws, "acs-verdict.json")); !os.IsNotExist(statErr) {
		t.Errorf("verdict file should be absent for a zero-predicate suite; stat err=%v", statErr)
	}
}

// TestGenerateACSVerdict_WriteVerdictError_Propagates is in
// audit_integration_test.go (//go:build integration) — it spawns a real subprocess.

// The registry init() must publish an "audit" factory that builds a runnable
// PhaseRunner with the production defaults wired (exercises the init closure).
func TestRegistry_AuditFactory_BuildsRunner(t *testing.T) {
	factory, ok := registry.For(string(core.PhaseAudit))
	if !ok {
		t.Fatal(`registry.For("audit") returned ok=false; init() did not register`)
	}
	runner := factory(core.PhaseRequest{ProjectRoot: t.TempDir()})
	if runner == nil {
		t.Fatal("factory returned nil runner")
	}
	if runner.Name() != string(core.PhaseAudit) {
		t.Errorf("Name=%q, want audit", runner.Name())
	}
}

// --- verdict-format robustness (cycle-148 mis-grade fix) ---

func TestExtractAuditVerdict_Formats(t *testing.T) {
	cases := []struct {
		name      string
		content   string
		want      string
		wantFound bool
	}{
		{"canonical", "## Verdict\n**PASS**\n", core.VerdictPASS, true},
		{"canonical no bold", "## Verdict\nPASS\n", core.VerdictPASS, true},
		{"canonical blank line", "## Verdict\n\n**WARN**\n", core.VerdictWARN, true},
		{"inline bold colon", "**Verdict: PASS**\n", core.VerdictPASS, true},
		{"inline bold split colon", "**Verdict:** PASS\n", core.VerdictPASS, true},
		{"inline heading colon", "## Verdict: PASS\n", core.VerdictPASS, true},
		{"inline plain colon", "Verdict: FAIL\n", core.VerdictFAIL, true},
		{"inline preserves FAIL", "**Verdict: FAIL**\n", core.VerdictFAIL, true},
		{"inline preserves SKIPPED", "Verdict: SKIPPED\n", core.VerdictSKIPPED, true},
		{"real report cycle-148 shape", "# Audit\n<!-- token -->\n\n**Verdict: PASS**\n**Confidence: 0.92**\n", core.VerdictPASS, true},
		{"empty", "", "", false},
		{"no verdict declared", "# Audit Report\n\nLooks fine to me.\n", "", false},
		{"lowercase json key not matched", "  \"verdict\": \"PASS\",\n", "", false},
		{"prose mentioning verdict not matched", "The verdict criteria require PASS or FAIL.\n", "", false},
		{"no-colon prose not matched", "Verdict PASS is required before shipping.\n", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, found := extractAuditVerdict(tc.content, config.StageOff)
			if found != tc.wantFound {
				t.Fatalf("found=%v, want %v (verdict=%q)", found, tc.wantFound, got)
			}
			if found && got != tc.want {
				t.Errorf("verdict=%q, want %q", got, tc.want)
			}
		})
	}
}

// Regression for cycle-148: a genuine PASS written inline as "**Verdict: PASS**"
// with red_count==0 must grade PASS and route to ship — not be mis-graded FAIL.
func TestRun_InlineVerdictFormat_PASS(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	body := "# Audit Report — Cycle 148\n<!-- audit_bound_tree_sha: deadbeef -->\n\n**Verdict: PASS**\n**Confidence: 0.92**\n\nNo defects.\n"
	fb := &fakeBridge{writeArtifact: body, resp: core.BridgeResponse{CostUSD: 0.3}}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("# body"), NowFn: fixtures.FixedClock(time.Unix(1_700_000_000, 0), 60*time.Millisecond)})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 148, ProjectRoot: t.TempDir(), Workspace: ws})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS (inline verdict + red_count=0 must ship)", resp.Verdict)
	}
	if resp.NextPhase != "ship" {
		t.Errorf("NextPhase=%q, want ship", resp.NextPhase)
	}
}

// A non-empty report with red_count==0 but NO parseable verdict must FAIL
// LOUDLY (an explicit error diagnostic), not sink the cycle silently.
func TestRun_NonEmptyNoVerdict_RedZero_LoudDiag(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	body := "# Audit Report\n\nThe change looks acceptable but I forgot the verdict line.\n"
	fb := &fakeBridge{writeArtifact: body, resp: core.BridgeResponse{CostUSD: 0.3}}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("# body"), NowFn: fixtures.FixedClock(time.Unix(1_700_000_000, 0), 60*time.Millisecond)})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: t.TempDir(), Workspace: ws})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL (unparseable verdict)", resp.Verdict)
	}
	var found bool
	for _, d := range resp.Diagnostics {
		if d.Severity == "error" && strings.Contains(d.Message, "no parseable verdict") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a loud error diagnostic about the unparseable verdict; got %+v", resp.Diagnostics)
	}
}
