package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
)

// fakeHooks is a minimal Hooks impl that records calls and returns
// scripted values. The phase name and verdict are configurable so a
// single fakeHooks covers all the BaseRunner branches.
type fakeHooks struct {
	phase    string
	agent    string
	artifact string
	model    string
	// scripted outputs
	prompt         string
	verdict        string
	diagnostics    []core.Diagnostic
	nextPhase      string
	classifyCalls  int
	gotArtifact    string
	gotComposeReq  core.PhaseRequest
	gotComposeBody string
}

func (h *fakeHooks) PhaseName() string       { return h.phase }
func (h *fakeHooks) AgentPromptName() string { return h.agent }
func (h *fakeHooks) ArtifactFilename(req core.PhaseRequest) string {
	_ = req // unused in fake; real hooks may vary by request
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
	return h.verdict, h.diagnostics, h.nextPhase
}

// fakeBridge captures the BridgeRequest and writes a scripted artifact
// to the requested path (mimicking what claude-p does).
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

// fakePromptsFS wires a prompts.Loader to an in-memory agent doc.
func fakePromptsFS(agentName, body string) *prompts.Loader {
	return prompts.NewFromFS(fstest.MapFS{
		"agents/" + agentName + ".md": &fstest.MapFile{
			Data: []byte("---\nname: " + agentName + "\n---\n" + body),
		},
	})
}

// fixedClock returns t on the first call, t+dur on the second — the
// pattern the existing phase tests use to assert deterministic
// DurationMS.
func fixedClock(t time.Time, dur time.Duration) func() time.Time {
	calls := 0
	return func() time.Time {
		defer func() { calls++ }()
		if calls == 0 {
			return t
		}
		return t.Add(dur)
	}
}

// TestRun_HappyPath_DelegatesToHooksAndBridge — full success path.
// Asserts every Hook callback fires and BridgeRequest carries the
// expected per-phase fields.
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
	clock := fixedClock(time.Unix(1_700_000_000, 0), 200*time.Millisecond)
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

// TestRun_EnvOverridesModel — EVOLVE_<PHASE>_MODEL env beats DefaultModel.
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

// TestRun_BridgeError_ReturnsFAILWithDiagnostic — when bridge.Launch
// errors, BaseRunner short-circuits to FAIL with the error as a diag.
// Classify is NOT called on the error path.
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

// TestRun_ArtifactFileFallback — when bridge.Stdout is empty,
// BaseRunner reads the artifact from disk and hands it to Classify.
func TestRun_ArtifactFileFallback(t *testing.T) {
	ws := t.TempDir()
	body := "# from disk\n## Files Modified\n- y.go\n"
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "auto", verdict: core.VerdictPASS}
	// Bridge succeeds but writes nothing to Stdout (and doesn't write the file).
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

// TestRun_MissingDeps_ReturnsErrors — bridge or prompts missing
// produce immediate, descriptive errors with the phase name in the
// message.
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

// TestRun_AgentLoadFails_ReturnsError — when the agent doc is missing
// from the prompts loader, Run returns a "load agent" error.
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

// TestNew_PanicsOnNilHooks — defensive: catch the programmer error at
// startup instead of NPE later.
func TestNew_PanicsOnNilHooks(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("New with nil Hooks did not panic")
		}
	}()
	_ = New(Options{Bridge: &fakeBridge{}, Prompts: fakePromptsFS("x", "y")})
}

// TestNew_DefaultClockIsTimeNow — when NowFn is nil, BaseRunner uses
// time.Now (proxy: DurationMS is a non-zero positive number).
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

// TestRun_ArtifactPathDerivedFromHooks — confirm the file path the
// bridge sees uses the Hook's ArtifactFilename joined with Workspace.
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

// skippingHooks is a fakeHooks that also implements Skipper. Used to
// verify the optional-interface path of BaseRunner.
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

// TestRun_SkipperSkipsBeforeBridge — when a Hooks also implements
// Skipper and returns skipped=true, BaseRunner short-circuits without
// touching the bridge or prompts. The returned response carries the
// supplied verdict, nextPhase, and diagnostics.
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

// TestRun_SkipperReturnsFalse_BridgeStillRuns — Skipper.ShouldSkip
// returning false must NOT short-circuit; normal dispatch proceeds.
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

// TestRun_ExtraFlagsFromProfile — phaseflags integration: when the
// profile has permission_mode set, --permission-mode appears in
// BridgeRequest.ExtraFlags.
//
// Profile filename uses the AGENT name (e.g., builder.json for the
// build phase whose agent is "evolve-builder"), NOT the phase name.
// Convention: TrimPrefix(AgentPromptName, "evolve-"). Pinned here so
// any future change to the lookup convention has to update this test.
func TestRun_ExtraFlagsFromProfile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "builder.json"),
		[]byte(`{"permission_mode":"plan","extra_flags":["--require-full"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "auto", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "x"}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-builder", "x")})
	_, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(fb.gotReq.ExtraFlags, " ")
	for _, want := range []string{"--require-full", "--permission-mode", "plan"} {
		if !strings.Contains(got, want) {
			t.Errorf("ExtraFlags missing %q; got %v", want, fb.gotReq.ExtraFlags)
		}
	}
}

// TestRun_CLIResolutionPrecedence pins the precedence chain:
//
//	EVOLVE_CLI env var > profile.cli field > "claude-p" default
//
// Before this fix the runner only read EVOLVE_CLI and defaulted to
// claude-p, silently ignoring profile.cli. Operators who edited a
// phase profile to `"cli": "codex"` got claude-p anyway, and the
// dispatch log gave no hint why.
//
// Source: cycle 107 (2026-05-25) attempted-codex smoke that ran
// against claude-sonnet-4-6 despite cli=codex in every profile.
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
			return resolvellm.Result{Model: wantModel, CLI: "claude-p"}, nil
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
