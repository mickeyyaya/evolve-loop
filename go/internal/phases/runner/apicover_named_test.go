package runner

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// TestBaseRunner_TypeNamedAndRuns names the BaseRunner type (every phase's
// BaseRunner() returns one, but the identifier is never named in runner's own
// test package) and exercises its Name() + Run() path: a PASS-verdict hooks
// drives one Classify call and a PASS/next-phase response.
func TestBaseRunner_TypeNamedAndRuns(t *testing.T) {
	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "auto",
		prompt: "composed", verdict: core.VerdictPASS, nextPhase: "triage"}
	fb := &fakeBridge{writeArtifact: "# scout-report\n"}

	var br *BaseRunner = New(Options{
		Hooks:   hooks,
		Bridge:  fb,
		Prompts: fakePromptsFS("evolve-scout", "agent body"),
	})
	if br == nil {
		t.Fatal("New returned nil *BaseRunner")
	}
	if br.Name() != "scout" {
		t.Errorf("BaseRunner.Name()=%q, want scout", br.Name())
	}

	resp, err := br.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: t.TempDir(), Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("BaseRunner.Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS || resp.NextPhase != "triage" {
		t.Errorf("BaseRunner.Run resp=%+v, want PASS/triage", resp)
	}
	if hooks.classifyCalls != 1 {
		t.Errorf("Classify calls=%d, want 1 (bridge path exercised)", hooks.classifyCalls)
	}
}

// TestSkipper_SatisfiedAndExercised names the Skipper interface, proves
// *skippingHooks satisfies it, and exercises the Run short-circuit: a skipping
// phase returns SKIPPED without ever calling the bridge.
func TestSkipper_SatisfiedAndExercised(t *testing.T) {
	var _ Skipper = (*skippingHooks)(nil)

	h := &skippingHooks{
		fakeHooks: fakeHooks{phase: "triage", agent: "evolve-triage", model: "auto"},
		skip:      true,
	}
	fb := &fakeBridge{}
	br := New(Options{Hooks: h, Bridge: fb, Prompts: fakePromptsFS("evolve-triage", "x")})

	resp, err := br.Run(context.Background(), core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictSKIPPED {
		t.Errorf("Skipper short-circuit verdict=%q, want SKIPPED", resp.Verdict)
	}
	if fb.gotReq.Agent != "" {
		t.Errorf("Skipper must not call the bridge; gotReq.Agent=%q", fb.gotReq.Agent)
	}
}

// TestInlinePromptProvider_SatisfiedAndExercised names the InlinePromptProvider
// interface, proves inlineHooks satisfies it, and exercises the inline-body
// branch: the composed prompt uses the inline body and never touches the disk
// loader (empty FS, no error).
func TestInlinePromptProvider_SatisfiedAndExercised(t *testing.T) {
	var _ InlinePromptProvider = inlineHooks{}

	base := &fakeHooks{phase: "minted-q", agent: "evolve-minted-q", model: "auto",
		prompt: "composed", verdict: core.VerdictPASS}
	hk := inlineHooks{fakeHooks: base, body: "INLINE Q", hasIt: true}
	fb := &fakeBridge{writeArtifact: "# q\n"}
	br := New(Options{Hooks: hk, Bridge: fb, Prompts: emptyPromptsFS()})

	resp, err := br.Run(context.Background(), core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("inline-prompt phase must not touch the disk loader; got %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
	if base.gotComposeBody != "INLINE Q" {
		t.Errorf("ComposePrompt body=%q, want the inline body 'INLINE Q'", base.gotComposeBody)
	}
}
