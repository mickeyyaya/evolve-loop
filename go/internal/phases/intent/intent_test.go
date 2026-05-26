// Tests for the intent phase. Drives the phase with a fake core.Bridge
// that captures the BridgeRequest and either writes a scripted artifact
// to disk or returns an error. The prompts loader is backed by fstest.MapFS
// so no disk fixtures are needed.
package intent

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

// fakeBridge captures the most recent Launch call and produces a
// scripted response. If WriteArtifact is non-empty, fakeBridge writes
// it to req.ArtifactPath before returning (mirroring real bridge
// behavior — the adapter reads the file back into BridgeResponse.Stdout).
type fakeBridge struct {
	resp          core.BridgeResponse
	err           error
	writeArtifact string

	gotReq core.BridgeRequest
	calls  int
}

func (f *fakeBridge) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	f.calls++
	f.gotReq = req
	if f.writeArtifact != "" && req.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		if err := os.WriteFile(req.ArtifactPath, []byte(f.writeArtifact), 0o644); err != nil {
			return core.BridgeResponse{}, err
		}
		// Mimic adapter behavior: stdout carries the artifact body.
		f.resp.Stdout = f.writeArtifact
	}
	return f.resp, f.err
}

func (f *fakeBridge) Probe(ctx context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

// fixedClock returns t for the first Now() call and t+dur for subsequent.
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

// fakePromptsFS builds a Loader that resolves agents/evolve-intent.md.
func fakePromptsFS(body string) *prompts.Loader {
	mapFS := fstest.MapFS{
		"agents/evolve-intent.md": &fstest.MapFile{
			Data: []byte("---\nname: evolve-intent\ndescription: intent\n---\n" + body),
		},
	}
	return prompts.NewFromFS(mapFS)
}

func TestRun_HappyPath_WritesIntent(t *testing.T) {
	ws := t.TempDir()
	artifactBody := `---
name: intent
goal: rewrite evolve-loop in Go
non_goals: [maintain bash compatibility]
acceptance_checks:
  - go build ./... succeeds
  - all ACS predicates port to Go
challenged_premises:
  - "premise: rewrite has lower TCO than maintenance — challenged: only if bridge stays bash"
risk_level: high
---
Body here.
`
	fb := &fakeBridge{writeArtifact: artifactBody}
	clock := fixedClock(time.Unix(1_700_000_000, 0), 50*time.Millisecond)
	phase := New(Config{
		Bridge:  fb,
		Prompts: fakePromptsFS("# Intent Architect body"),
		NowFn:   clock,
	})

	req := core.PhaseRequest{
		Cycle:       42,
		ProjectRoot: "/tmp/proj",
		Workspace:   ws,
		Worktree:    "/tmp/proj/worktree",
		GoalHash:    "abc12345",
	}
	resp, err := phase.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
	if resp.Phase != string(core.PhaseIntent) {
		t.Errorf("Phase=%q, want intent", resp.Phase)
	}
	if resp.NextPhase != string(core.PhaseScout) {
		t.Errorf("NextPhase=%q, want scout", resp.NextPhase)
	}
	if resp.DurationMS != 50 {
		t.Errorf("DurationMS=%d, want 50", resp.DurationMS)
	}
	if resp.ArtifactsDir != ws {
		t.Errorf("ArtifactsDir=%q, want %q", resp.ArtifactsDir, ws)
	}
	// Inspect captured BridgeRequest for plumbed inputs.
	if fb.gotReq.Cycle != 42 {
		t.Errorf("BridgeRequest.Cycle=%d, want 42", fb.gotReq.Cycle)
	}
	if fb.gotReq.Agent != "intent" {
		t.Errorf("BridgeRequest.Agent=%q, want intent", fb.gotReq.Agent)
	}
	if fb.gotReq.ArtifactPath != filepath.Join(ws, "intent.md") {
		t.Errorf("BridgeRequest.ArtifactPath=%q, want %s/intent.md", fb.gotReq.ArtifactPath, ws)
	}
	wantProfile := filepath.Join("/tmp/proj", ".evolve", "profiles", "intent.json")
	if fb.gotReq.Profile != wantProfile {
		t.Errorf("BridgeRequest.Profile=%q, want %q", fb.gotReq.Profile, wantProfile)
	}
	if !strings.Contains(fb.gotReq.Prompt, "# Intent Architect body") {
		t.Errorf("Prompt missing agent body; got %q", fb.gotReq.Prompt)
	}
	if !strings.Contains(fb.gotReq.Prompt, "cycle: 42") {
		t.Errorf("Prompt missing cycle context; got %q", fb.gotReq.Prompt)
	}
	if !strings.Contains(fb.gotReq.Prompt, "goal_hash: abc12345") {
		t.Errorf("Prompt missing goal_hash; got %q", fb.gotReq.Prompt)
	}
}

func TestRun_DeltaMode_IntentUnchanged_SKIPPED(t *testing.T) {
	ws := t.TempDir()
	fb := &fakeBridge{writeArtifact: "[intent-unchanged] goal_hash=abc12345\n"}
	phase := New(Config{
		Bridge:  fb,
		Prompts: fakePromptsFS("# body"),
	})
	req := core.PhaseRequest{
		Cycle:       1,
		ProjectRoot: "/tmp/proj",
		Workspace:   ws,
		GoalHash:    "abc12345",
		Env:         map[string]string{"EVOLVE_INTENT_DELTA": "1"},
	}
	resp, err := phase.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictSKIPPED {
		t.Errorf("Verdict=%q, want SKIPPED", resp.Verdict)
	}
	wantArtifact := filepath.Join(ws, "intent-delta.md")
	if fb.gotReq.ArtifactPath != wantArtifact {
		t.Errorf("ArtifactPath=%q, want %q (delta mode)", fb.gotReq.ArtifactPath, wantArtifact)
	}
	if !strings.Contains(fb.gotReq.Prompt, "mode: delta") {
		t.Errorf("Prompt missing delta mode hint")
	}
}

func TestRun_EmptyArtifact_FAIL(t *testing.T) {
	ws := t.TempDir()
	fb := &fakeBridge{writeArtifact: ""}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle:       1,
		ProjectRoot: "/tmp/proj",
		Workspace:   ws,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
}

func TestRun_MissingRequiredSections_FAIL(t *testing.T) {
	ws := t.TempDir()
	fb := &fakeBridge{writeArtifact: "Some prose without YAML frontmatter or goal."}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle:       1,
		ProjectRoot: "/tmp/proj",
		Workspace:   ws,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
}

func TestRun_BridgeError_PropagatesAndMarksFAIL(t *testing.T) {
	ws := t.TempDir()
	bridgeErr := errors.New("bridge: launch failed")
	fb := &fakeBridge{err: bridgeErr}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle:       1,
		ProjectRoot: "/tmp/proj",
		Workspace:   ws,
	})
	if !errors.Is(err, bridgeErr) {
		t.Errorf("err=%v, want wraps bridgeErr", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
	if len(resp.Diagnostics) == 0 {
		t.Errorf("Diagnostics empty; want a recorded bridge error")
	}
}

func TestRun_MissingBridge_ReturnsError(t *testing.T) {
	phase := New(Config{Bridge: nil, Prompts: fakePromptsFS("body")})
	_, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1})
	if err == nil {
		t.Fatal("err=nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "bridge required") {
		t.Errorf("err=%v, want bridge-required message", err)
	}
}

func TestRun_MissingPrompts_ReturnsError(t *testing.T) {
	fb := &fakeBridge{}
	phase := New(Config{Bridge: fb, Prompts: nil})
	_, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1})
	if err == nil {
		t.Fatal("err=nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "prompts loader required") {
		t.Errorf("err=%v, want prompts-required message", err)
	}
}

func TestRun_AgentLoadFails_ReturnsError(t *testing.T) {
	// Loader without the agents/evolve-intent.md file.
	emptyLoader := prompts.NewFromFS(fstest.MapFS{})
	fb := &fakeBridge{}
	phase := New(Config{Bridge: fb, Prompts: emptyLoader})
	_, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1})
	if err == nil {
		t.Fatal("err=nil, want non-nil")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err=%v, want fs.ErrNotExist", err)
	}
}

func TestRun_EnvOverridesCLIAndModel(t *testing.T) {
	ws := t.TempDir()
	fb := &fakeBridge{writeArtifact: "goal: x\nacceptance_checks: [a]\n"}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	_, _ = phase.Run(context.Background(), core.PhaseRequest{
		Cycle:       1,
		ProjectRoot: "/tmp/proj",
		Workspace:   ws,
		Env: map[string]string{
			"EVOLVE_CLI":          "codex",
			"EVOLVE_INTENT_MODEL": "gpt-5",
		},
	})
	if fb.gotReq.CLI != "codex" {
		t.Errorf("CLI=%q, want codex", fb.gotReq.CLI)
	}
	if fb.gotReq.Model != "gpt-5" {
		t.Errorf("Model=%q, want gpt-5", fb.gotReq.Model)
	}
}

func TestName(t *testing.T) {
	p := New(Config{})
	if p.Name() != "intent" {
		t.Errorf("Name=%q, want intent", p.Name())
	}
}

// --- v12.1 Capability 1: phaseflags wiring tests ---

func writeIntentProfile(t *testing.T, contents string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir profiles: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "intent.json"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return root
}

// TestRun_PopulatesExtraFlagsFromProfile — profile.extra_flags and
// profile.permission_mode surface in the BridgeRequest the phase sends
// to the bridge.
func TestRun_PopulatesExtraFlagsFromProfile(t *testing.T) {
	root := writeIntentProfile(t, `{"extra_flags":["--require-full"],"permission_mode":"acceptEdits"}`)
	fb := &fakeBridge{}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	_, _ = phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: root, Workspace: t.TempDir(),
	})
	got := strings.Join(fb.gotReq.ExtraFlags, " ")
	for _, want := range []string{"--require-full", "--permission-mode", "acceptEdits"} {
		if !strings.Contains(got, want) {
			t.Errorf("ExtraFlags missing %q; got %v", want, fb.gotReq.ExtraFlags)
		}
	}
}

// TestRun_EnvOverridesProfilePermissionMode — EVOLVE_INTENT_PERMISSION_MODE
// in req.Env beats the profile's permission_mode.
func TestRun_EnvOverridesProfilePermissionMode(t *testing.T) {
	root := writeIntentProfile(t, `{"permission_mode":"acceptEdits"}`)
	fb := &fakeBridge{}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	_, _ = phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: root, Workspace: t.TempDir(),
		Env: map[string]string{"EVOLVE_INTENT_PERMISSION_MODE": "plan"},
	})
	got := strings.Join(fb.gotReq.ExtraFlags, " ")
	if !strings.Contains(got, "--permission-mode plan") {
		t.Errorf("env override not honored; got %v", fb.gotReq.ExtraFlags)
	}
	if strings.Contains(got, "acceptEdits") {
		t.Errorf("profile value should be overridden, not appended; got %v", fb.gotReq.ExtraFlags)
	}
}

// TestComposePrompt_InjectsGoalTextFromContext is the regression test
// for cycle-108 bug #3. When the operator passed --goal-text "foo",
// the dispatcher routes it through Context["goal"]; the intent
// persona's prompt MUST surface it in the Cycle Context block so the
// persona structures intent.md around the real goal rather than
// inferring from workspace leftovers.
func TestComposePrompt_InjectsGoalTextFromContext(t *testing.T) {
	h := hooks{}
	req := core.PhaseRequest{
		Cycle:       108,
		GoalHash:    "abc123",
		ProjectRoot: "/p",
		Workspace:   "/p/.evolve/runs/cycle-108",
		Context:     map[string]string{"goal": "auto-review pipeline for non-stop autonomy"},
	}
	got := h.ComposePrompt("BODY", req)
	if !strings.Contains(got, "- goal: auto-review pipeline for non-stop autonomy") {
		t.Errorf("expected goal text line in prompt, got: %q", got)
	}
}

func TestComposePrompt_OmitsGoalLineWhenContextEmpty(t *testing.T) {
	h := hooks{}
	req := core.PhaseRequest{Cycle: 1, GoalHash: "x", ProjectRoot: "/p", Workspace: "/w"}
	got := h.ComposePrompt("BODY", req)
	if strings.Contains(got, "- goal:") {
		t.Errorf("no goal in Context should omit the line; got: %q", got)
	}
	// Other context still emitted (cycle, goal_hash, project_root, workspace, mode).
	if !strings.Contains(got, "- cycle: 1") {
		t.Errorf("cycle line missing: %q", got)
	}
}
