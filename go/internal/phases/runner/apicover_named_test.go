package runner

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestBaseRunner_TypeNamedAndRuns(t *testing.T) {
	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "auto",
		prompt: "composed", verdict: core.VerdictPASS, nextPhase: "triage"}
	fb := &fakeBridge{writeArtifact: "# scout-report\n"}

	var br *BaseRunner = New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-scout", "agent body"),
		VerifyFn: alwaysOKVerify,
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

func TestSkipper_SatisfiedAndExercised(t *testing.T) {

	h := &skippingHooks{
		fakeHooks: fakeHooks{phase: "triage", agent: "evolve-triage", model: "auto"},
		skip:      true,
	}
	fb := &fakeBridge{}
	var skipper Skipper = h
	if skipped, verdict, _, _ := skipper.ShouldSkip(core.PhaseRequest{}); !skipped || verdict != core.VerdictSKIPPED {
		t.Errorf("Skipper.ShouldSkip = %v %q, want skipped with SKIPPED", skipped, verdict)
	}
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

func TestInlinePromptProvider_SatisfiedAndExercised(t *testing.T) {

	base := &fakeHooks{phase: "minted-q", agent: "evolve-minted-q", model: "auto",
		prompt: "composed", verdict: core.VerdictPASS}
	hk := inlineHooks{fakeHooks: base, body: "INLINE Q", hasIt: true}
	var provider InlinePromptProvider = hk
	if body, ok := provider.InlinePromptBody(); !ok || body != "INLINE Q" {
		t.Errorf("InlinePromptBody = %q %v, want INLINE Q", body, ok)
	}
	fb := &fakeBridge{writeArtifact: "# q\n"}
	br := New(Options{Hooks: hk, Bridge: fb, Prompts: emptyPromptsFS()})

	resp, err := br.Run(context.Background(), core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("inline-prompt phase must not touch the disk loader; got %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
	if got := base.gotComposeBody; got != "INLINE Q" {
		t.Errorf("ComposePrompt body=%q, want the inline body 'INLINE Q'", got)
	}
}
