package runner

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/digest"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// TestMain unsets EVOLVE_CLI so an operator shell value cannot leak into the profile and default CLI tiers.
func TestMain(m *testing.M) {
	os.Unsetenv("EVOLVE_CLI")
	os.Exit(m.Run())
}

type fakeHooks struct {
	phase         string
	agent         string
	artifact      string
	model         string
	prompt        string
	verdict       string
	diagnostics   []core.Diagnostic
	nextPhase     string
	classifyCalls int
	gotArtifact   string
	gotComposeReq core.PhaseRequest
	// onClassify observes the request at classify time, after the fence has restored the tree.
	onClassify     func(core.PhaseRequest)
	gotComposeBody string
}

func (h *fakeHooks) PhaseName() string       { return h.phase }
func (h *fakeHooks) AgentPromptName() string { return h.agent }
func (h *fakeHooks) ArtifactFilename(req core.PhaseRequest) string {
	_ = req
	if h.artifact == "" {
		return h.phase + "-report.md"
	}
	return h.artifact
}
func (h *fakeHooks) DefaultModel() string { return h.model }
func (h *fakeHooks) ComposePrompt(body string, req core.PhaseRequest) string {
	h.gotComposeBody = body
	h.gotComposeReq = req
	return h.prompt
}
func (h *fakeHooks) Classify(artifact string, req core.PhaseRequest, bres core.BridgeResponse) (string, []core.Diagnostic, string) {
	h.classifyCalls++
	h.gotArtifact = artifact
	if h.onClassify != nil {
		h.onClassify(req)
	}
	return h.verdict, h.diagnostics, h.nextPhase
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

func fakePromptsFS(agentName, body string) *prompts.Loader {
	return prompts.NewFromFS(fstest.MapFS{
		"agents/" + agentName + ".md": &fstest.MapFile{
			Data: []byte("---\nname: " + agentName + "\n---\n" + body),
		},
	})
}

func TestRun_HappyPath_DelegatesToHooksAndBridge(t *testing.T) {
	hooks := &fakeHooks{
		phase:     "build",
		agent:     "evolve-builder",
		model:     "sonnet",
		prompt:    "composed body",
		verdict:   core.VerdictPASS,
		nextPhase: "audit",
	}
	fb := &fakeBridge{writeArtifact: "# build artifact\n## Files Modified\n- a.go\n"}
	clock := fixtures.FixedClock(time.Unix(1_700_000_000, 0), 200*time.Millisecond)
	r := New(Options{
		Hooks:   hooks,
		Bridge:  fb,
		Prompts: fakePromptsFS("evolve-builder", "agent body"),
		NowFn:   clock,
	})

	if r.Name() != "build" {
		t.Errorf("Name=%q, want build", r.Name())
	}

	resp, err := r.Run(context.Background(), core.PhaseRequest{
		Cycle: 9, ProjectRoot: t.TempDir(), Workspace: t.TempDir(),
		RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
	if resp.NextPhase != "audit" {
		t.Errorf("NextPhase=%q, want audit", resp.NextPhase)
	}
	if resp.DurationMS != 200 {
		t.Errorf("DurationMS=%d, want 200", resp.DurationMS)
	}
	if fb.gotReq.Agent != "build" {
		t.Errorf("BridgeRequest.Agent=%q, want build", fb.gotReq.Agent)
	}
	if fb.gotReq.RunID != "01ARZ3NDEKTSV4RRFFQ69G5FAV" {
		t.Errorf("BridgeRequest.RunID=%q, want the PhaseRequest's run id", fb.gotReq.RunID)
	}
	if fb.gotReq.Prompt != "composed body" {
		t.Errorf("BridgeRequest.Prompt=%q, want 'composed body'", fb.gotReq.Prompt)
	}
	if fb.gotReq.Model != "sonnet" {
		t.Errorf("BridgeRequest.Model=%q, want sonnet (DefaultModel)", fb.gotReq.Model)
	}
	if hooks.classifyCalls != 1 {
		t.Errorf("Classify call count=%d, want 1", hooks.classifyCalls)
	}
	if !strings.Contains(hooks.gotArtifact, "Files Modified") {
		t.Errorf("Classify did not receive artifact contents; got %q", hooks.gotArtifact)
	}
}

func TestRun_RoleScopedDigestMaterializesBeforeCompose(t *testing.T) {
	const source = `UNTAGGED
<!-- digest:role=scout -->
SCOUT ONLY
<!-- /digest -->
<!-- digest:role=build -->
BUILD ONLY
<!-- /digest -->`
	hooks := &fakeHooks{
		phase:   "scout",
		agent:   "evolve-scout",
		model:   "sonnet",
		prompt:  "composed body",
		verdict: core.VerdictPASS,
	}
	var diagnostics bytes.Buffer
	r := New(Options{
		Hooks:   hooks,
		Bridge:  &fakeBridge{writeArtifact: "# scout artifact\n"},
		Prompts: fakePromptsFS("evolve-scout", source),
		Diag:    log.Console{Out: &diagnostics, Err: &diagnostics},
	})

	if _, err := r.Run(context.Background(), core.PhaseRequest{
		Cycle: 1460, ProjectRoot: t.TempDir(), Workspace: t.TempDir(),
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got, want := strings.TrimSpace(hooks.gotComposeBody), "SCOUT ONLY"; got != want {
		t.Errorf("ComposePrompt body = %q, want role-scoped %q", got, want)
	}
	if !strings.Contains(diagnostics.String(), "digest-shadow outcome=matched") {
		t.Errorf("runner diagnostics = %q, want matched digest shadow record", diagnostics.String())
	}
	if got := FormatDigestShadowLog("scout", digest.ShadowRecord{
		FullBytes: 10, DigestBytes: 5, Parity: true, Outcome: digest.OutcomeMatched,
	}); got != "[runner] phase=scout digest-shadow outcome=matched full_bytes=10 digest_bytes=5 parity=true" {
		t.Errorf("FormatDigestShadowLog = %q", got)
	}
}

func writeBudgetProfile(t *testing.T, profileJSON string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scout.json"), []byte(profileJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func runScoutWithProfile(t *testing.T, root string) *fakeBridge {
	t.Helper()
	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "sonnet", prompt: "scout body", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "## Proposed Tasks\n1. x\n"}
	r := New(Options{
		Hooks: hooks, Bridge: fb,
		Prompts: fakePromptsFS("evolve-scout", "agent body"),
		NowFn:   fixtures.FixedClock(time.Unix(1_700_000_000, 0), 0),
	})
	if _, err := r.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: root, Workspace: t.TempDir()}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return fb
}

func TestRun_InjectsAdvisoryBudgetHint(t *testing.T) {
	root := writeBudgetProfile(t, `{"name":"scout","role":"scout","cli":"claude-tmux","turn_budget_hint":15}`)
	fb := runScoutWithProfile(t, root)
	if !strings.HasPrefix(fb.gotReq.Prompt, "scout body") {
		t.Errorf("budget hint should append to the prompt, not replace it; got %q", fb.gotReq.Prompt)
	}
	if !strings.Contains(fb.gotReq.Prompt, "Advisory turn budget") || !strings.Contains(fb.gotReq.Prompt, "~15 turns") {
		t.Errorf("prompt missing advisory budget hint; got %q", fb.gotReq.Prompt)
	}
}

func TestRun_NoBudgetHintWhenProfileOmitsIt(t *testing.T) {
	root := writeBudgetProfile(t, `{"name":"scout","role":"scout","cli":"claude-tmux"}`)
	fb := runScoutWithProfile(t, root)
	if fb.gotReq.Prompt != "scout body" {
		t.Errorf("no hint expected; prompt should be unchanged, got %q", fb.gotReq.Prompt)
	}
}

func TestRun_InvokesEventsProducer(t *testing.T) {
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "sonnet", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "x"}
	ws := t.TempDir()
	var gotWS, gotPhase, gotCLI string
	var gotCycle, calls int
	r := New(Options{
		Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-builder", "x"),
		EventsProducer: func(workspace, phase, cli string, cycle int, prompt string) error {
			calls++
			gotWS, gotPhase, gotCLI, gotCycle = workspace, phase, cli, cycle
			return nil
		},
	})
	if _, err := r.Run(context.Background(), core.PhaseRequest{
		Cycle: 9, ProjectRoot: t.TempDir(), Workspace: ws,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if calls != 1 {
		t.Fatalf("EventsProducer calls=%d, want 1", calls)
	}
	if gotWS != ws || gotPhase != "build" || gotCycle != 9 {
		t.Errorf("EventsProducer args: ws=%q phase=%q cycle=%d (want %q/build/9)", gotWS, gotPhase, gotCycle, ws)
	}
	if gotCLI != "claude-tmux" {
		t.Errorf("EventsProducer cli=%q, want claude-tmux (default)", gotCLI)
	}
}

func TestRun_EventsProducer_RunsOnBridgeError(t *testing.T) {
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "sonnet"}
	fb := &fakeBridge{err: errors.New("bridge timeout")}
	var calls int
	r := New(Options{
		Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-builder", "x"),
		EventsProducer: func(_, _, _ string, _ int, _ string) error { calls++; return nil },
	})
	resp, err := r.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: t.TempDir(), Workspace: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected bridge error to propagate")
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
	if calls != 1 {
		t.Fatalf("EventsProducer calls=%d on bridge-error path, want 1 (infra classification depends on it)", calls)
	}
}

func TestRun_EventsProducerError_NonBlocking(t *testing.T) {
	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "sonnet", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "x"}
	r := New(Options{
		Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-scout", "x"),
		VerifyFn:       alwaysOKVerify,
		EventsProducer: func(_, _, _ string, _ int, _ string) error { return errors.New("disk full") },
	})
	resp, err := r.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: t.TempDir(), Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("producer error must not fail the phase: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS despite producer error", resp.Verdict)
	}
}

// scout is used because its phase and agent names coincide; runner_perphase_env_test.go pins the pairs that differ.
func TestRun_EnvOverridesModel(t *testing.T) {
	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "auto", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "x"}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-scout", "x")})

	_, err := r.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: t.TempDir(), Workspace: t.TempDir(),
		Env: map[string]string{"EVOLVE_SCOUT_MODEL": "opus"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fb.gotReq.Model != "opus" {
		t.Errorf("Model=%q, want opus (env override)", fb.gotReq.Model)
	}
}

func TestRun_BridgeError_ReturnsFAILWithDiagnostic(t *testing.T) {
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "auto"}
	bridgeErr := errors.New("bridge boom")
	fb := &fakeBridge{err: bridgeErr}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-builder", "x")})

	resp, err := r.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: t.TempDir(), Workspace: t.TempDir(),
	})
	if !errors.Is(err, bridgeErr) {
		t.Errorf("err=%v, want wraps bridge err", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
	if hooks.classifyCalls != 0 {
		t.Errorf("Classify must not be called on bridge error; got %d calls", hooks.classifyCalls)
	}
	if len(resp.Diagnostics) == 0 || resp.Diagnostics[0].Severity != "error" {
		t.Errorf("expected error diagnostic; got %v", resp.Diagnostics)
	}
}

func TestRun_ArtifactFileFallback(t *testing.T) {
	ws := t.TempDir()
	body := "# from disk\n## Files Modified\n- y.go\n"
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "auto", verdict: core.VerdictPASS}
	fb := &fakeBridge{}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-builder", "x")})

	_, err := r.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: t.TempDir(), Workspace: ws,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(hooks.gotArtifact, "from disk") {
		t.Errorf("Classify did not receive file-fallback artifact; got %q", hooks.gotArtifact)
	}
}

func TestRun_MissingDeps_ReturnsErrors(t *testing.T) {
	hooks := &fakeHooks{phase: "audit"}
	t.Run("no-bridge", func(t *testing.T) {
		r := New(Options{Hooks: hooks, Prompts: fakePromptsFS("evolve-audit", "x")})
		_, err := r.Run(context.Background(), core.PhaseRequest{})
		if err == nil || !strings.Contains(err.Error(), "audit") || !strings.Contains(err.Error(), "bridge") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("no-prompts", func(t *testing.T) {
		r := New(Options{Hooks: hooks, Bridge: &fakeBridge{}})
		_, err := r.Run(context.Background(), core.PhaseRequest{})
		if err == nil || !strings.Contains(err.Error(), "audit") || !strings.Contains(err.Error(), "prompts") {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestRun_AgentLoadFails_ReturnsError(t *testing.T) {
	hooks := &fakeHooks{phase: "build", agent: "evolve-missing"}
	r := New(Options{
		Hooks:   hooks,
		Bridge:  &fakeBridge{},
		Prompts: prompts.NewFromFS(fstest.MapFS{}),
	})
	_, err := r.Run(context.Background(), core.PhaseRequest{})
	if err == nil || !strings.Contains(err.Error(), "load agent") {
		t.Fatalf("err=%v, want 'load agent' error", err)
	}
}

func TestNew_PanicsOnNilHooks(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("New with nil Hooks did not panic")
		}
	}()
	_ = New(Options{Bridge: &fakeBridge{}, Prompts: fakePromptsFS("x", "y")})
}

func TestNew_DefaultClockIsTimeNow(t *testing.T) {
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "auto", verdict: core.VerdictPASS}
	r := New(Options{Hooks: hooks, Bridge: &fakeBridge{writeArtifact: "x"}, Prompts: fakePromptsFS("evolve-builder", "x")})
	resp, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if resp.DurationMS < 0 {
		t.Errorf("DurationMS=%d (must be >= 0 with real clock)", resp.DurationMS)
	}
}

func TestRun_ArtifactPathDerivedFromHooks(t *testing.T) {
	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", artifact: "custom.md", model: "auto", verdict: core.VerdictPASS}
	ws := t.TempDir()
	fb := &fakeBridge{writeArtifact: "x"}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-scout", "x")})
	_, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: ws})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(ws, "custom.md")
	if fb.gotReq.ArtifactPath != want {
		t.Errorf("ArtifactPath=%q, want %q", fb.gotReq.ArtifactPath, want)
	}
}

type skippingHooks struct {
	fakeHooks
	skip       bool
	skipReason string
}

func (s *skippingHooks) ShouldSkip(req core.PhaseRequest) (bool, string, string, []core.Diagnostic) {
	if !s.skip {
		return false, "", "", nil
	}
	return true, core.VerdictSKIPPED, "next", []core.Diagnostic{{Severity: "info", Message: s.skipReason}}
}

func TestRun_SkipperSkipsBeforeBridge(t *testing.T) {
	h := &skippingHooks{
		fakeHooks:  fakeHooks{phase: "triage", agent: "evolve-triage", model: "auto"},
		skip:       true,
		skipReason: "disabled via EVOLVE_X",
	}
	fb := &fakeBridge{}
	r := New(Options{Hooks: h, Bridge: fb, Prompts: fakePromptsFS("evolve-triage", "x")})

	resp, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictSKIPPED {
		t.Errorf("Verdict=%q, want SKIPPED", resp.Verdict)
	}
	if resp.NextPhase != "next" {
		t.Errorf("NextPhase=%q, want next", resp.NextPhase)
	}
	if fb.gotReq.Agent != "" {
		t.Errorf("Bridge was called despite skip; gotReq.Agent=%q", fb.gotReq.Agent)
	}
	if h.classifyCalls != 0 {
		t.Errorf("Classify was called despite skip; count=%d", h.classifyCalls)
	}
	if len(resp.Diagnostics) != 1 || resp.Diagnostics[0].Message != "disabled via EVOLVE_X" {
		t.Errorf("Diagnostics not propagated; got %v", resp.Diagnostics)
	}
}

func TestRun_SkipperReturnsFalse_BridgeStillRuns(t *testing.T) {
	h := &skippingHooks{
		fakeHooks: fakeHooks{phase: "triage", agent: "evolve-triage", model: "auto", verdict: core.VerdictPASS},
		skip:      false,
	}
	fb := &fakeBridge{writeArtifact: "x"}
	r := New(Options{Hooks: h, Bridge: fb, Prompts: fakePromptsFS("evolve-triage", "x")})

	_, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fb.gotReq.Agent != "triage" {
		t.Errorf("Bridge not called; gotReq.Agent=%q", fb.gotReq.Agent)
	}
	if h.classifyCalls != 1 {
		t.Errorf("Classify count=%d, want 1", h.classifyCalls)
	}
}

func TestRun_PermissionModeOverride(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "builder.json"),
		[]byte(`{"name":"builder","cli":"claude-tmux","permission_mode":"acceptEdits"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "auto", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "x"}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-builder", "x")})
	_, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root, Workspace: t.TempDir(),
		Env: map[string]string{"EVOLVE_BUILDER_PERMISSION_MODE": "plan"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if fb.gotReq.PermissionMode != "plan" {
		t.Errorf("PermissionMode = %q, want \"plan\" (from EVOLVE_BUILDER_PERMISSION_MODE)", fb.gotReq.PermissionMode)
	}
	if len(fb.gotReq.ExtraFlags) != 0 {
		t.Errorf("ExtraFlags should be empty (permission is typed config, not a raw flag); got %v", fb.gotReq.ExtraFlags)
	}
}

func TestRun_PermissionModeFromProfile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "builder.json"),
		[]byte(`{"name":"builder","cli":"claude-tmux","permission_mode":"plan"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "auto", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "x"}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-builder", "x")})
	if _, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root, Workspace: t.TempDir(),
	}); err != nil {
		t.Fatal(err)
	}
	if fb.gotReq.PermissionMode != "plan" {
		t.Errorf("PermissionMode = %q, want \"plan\" from profile", fb.gotReq.PermissionMode)
	}
}

func TestRunnerMissingProfileFastFail(t *testing.T) {
	root := t.TempDir()
	profileDir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "auto", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "x"}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-builder", "x")})
	resp, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})
	if err == nil {
		t.Fatal("Run succeeded with .evolve/profiles present but builder.json missing")
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Fatalf("Verdict=%q, want FAIL", resp.Verdict)
	}
	if fb.gotReq.Profile != "" {
		t.Fatalf("bridge was launched despite missing profile: %+v", fb.gotReq)
	}
}

func TestRunnerMissingProfileDiagnostic(t *testing.T) {
	root := t.TempDir()
	profileDir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(profileDir, "builder.json")
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "auto", verdict: core.VerdictPASS}
	r := New(Options{Hooks: hooks, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-builder", "x")})
	resp, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})
	if err == nil {
		t.Fatal("Run succeeded with missing profile")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Fatalf("error %q does not name missing profile path %q", err, missing)
	}
	if len(resp.Diagnostics) == 0 || !strings.Contains(resp.Diagnostics[0].Message, missing) {
		t.Fatalf("diagnostics do not name missing profile path %q: %+v", missing, resp.Diagnostics)
	}
}

func TestRun_CLIResolutionPrecedence(t *testing.T) {
	mkProfile := func(t *testing.T, cli string) string {
		t.Helper()
		root := t.TempDir()
		dir := filepath.Join(root, ".evolve", "profiles")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := `{"cli":"` + cli + `","model_tier_default":"sonnet"}`
		if err := os.WriteFile(filepath.Join(dir, "scout.json"),
			[]byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return root
	}
	runOne := func(t *testing.T, profileCLI string, envOverride map[string]string) string {
		t.Helper()
		root := mkProfile(t, profileCLI)
		hooks := &fakeHooks{phase: "scout", agent: "evolve-scout",
			model: "sonnet", verdict: core.VerdictPASS}
		fb := &fakeBridge{writeArtifact: "x"}
		r := New(Options{Hooks: hooks, Bridge: fb,
			Prompts: fakePromptsFS("evolve-scout", "x")})
		_, err := r.Run(context.Background(), core.PhaseRequest{
			ProjectRoot: root, Workspace: t.TempDir(), Env: envOverride,
		})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		return fb.gotReq.CLI
	}

	cases := []struct {
		name       string
		profileCLI string
		envCLI     string
		wantCLI    string
	}{
		{"profile_codex_no_env", "codex", "", "codex"},
		{"profile_agy_no_env", "agy", "", "agy"},
		{"env_wins_over_profile", "codex", "claude-tmux", "claude-tmux"},
		{"empty_profile_default", "", "", "claude-tmux"},
		{"env_overrides_empty_profile", "", "codex", "codex"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string(nil)
			if tc.envCLI != "" {
				env = map[string]string{"EVOLVE_CLI": tc.envCLI}
			} else {
				t.Setenv("EVOLVE_CLI", "") // isolate from an operator shell's EVOLVE_CLI
			}
			got := runOne(t, tc.profileCLI, env)
			if got != tc.wantCLI {
				t.Errorf("CLI=%q, want %q", got, tc.wantCLI)
			}
		})
	}
}

func TestRunnerAutoModel_ResolvesConcreteModel(t *testing.T) {
	const wantModel = "claude-opus-4-7-20251001"
	hooks := &fakeHooks{
		phase: "audit", agent: "evolve-auditor",
		model: "auto", verdict: core.VerdictPASS,
	}
	fb := &fakeBridge{writeArtifact: "PASS"}
	stubCalled := false
	r := New(Options{
		Hooks:   hooks,
		Bridge:  fb,
		Prompts: fakePromptsFS("evolve-auditor", "body"),
		ResolveLLM: func(phase string, opts resolvellm.Options) (resolvellm.Result, error) {
			stubCalled = true
			// resolvellm yields a tier, which reaches the bridge as the model; the realizer maps it downstream.
			return resolvellm.Result{ModelTier: wantModel, CLI: "claude-p"}, nil
		},
	})
	_, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: t.TempDir(),
		Workspace:   t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !stubCalled {
		t.Error("ResolveLLM seam was not called for auto model")
	}
	if fb.gotReq.Model != wantModel {
		t.Errorf("BridgeRequest.Model=%q, want %q", fb.gotReq.Model, wantModel)
	}
}
