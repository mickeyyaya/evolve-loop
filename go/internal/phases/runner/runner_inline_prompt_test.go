package runner

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

// fakeHooks is embedded by pointer so the test reads back what ComposePrompt recorded; a value embed records into a copy.
type inlineHooks struct {
	*fakeHooks
	body  string
	hasIt bool
}

func (h inlineHooks) InlinePromptBody() (string, bool) { return h.body, h.hasIt }

// emptyPromptsFS fails every agent load, so only an inline body can compose.
func emptyPromptsFS() *prompts.Loader {
	return prompts.NewFromFS(fstest.MapFS{})
}

func TestRun_InlinePrompt_UsesBodyAndSkipsLoader(t *testing.T) {
	base := &fakeHooks{phase: "minted-x", agent: "evolve-minted-x", model: "sonnet",
		prompt: "composed", verdict: core.VerdictPASS, nextPhase: ""}
	hk := inlineHooks{fakeHooks: base, body: "INLINE BODY", hasIt: true}
	fb := &fakeBridge{writeArtifact: "# minted artifact\n"}
	r := New(Options{Hooks: hk, Bridge: fb, Prompts: emptyPromptsFS()})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir(), ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("inline-prompt phase must NOT touch the disk loader; got %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
	if base.gotComposeBody != "INLINE BODY" {
		t.Errorf("ComposePrompt body=%q, want the inline body", base.gotComposeBody)
	}
}

func TestRun_InlinePrompt_EmptyBodyButOptedIn_SkipsLoader(t *testing.T) {
	base := &fakeHooks{phase: "minted-z", agent: "evolve-minted-z", model: "sonnet",
		prompt: "composed", verdict: core.VerdictPASS}
	hk := inlineHooks{fakeHooks: base, body: "", hasIt: true}
	fb := &fakeBridge{writeArtifact: "# z\n"}
	r := New(Options{Hooks: hk, Bridge: fb, Prompts: emptyPromptsFS()})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir(), ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("ok=true must skip the loader even with an empty body; got %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
	if base.gotComposeBody != "" {
		t.Errorf("ComposePrompt body=%q, want empty (the opted-in inline body)", base.gotComposeBody)
	}
}

func TestRun_InlinePrompt_EmptyFallsBackToLoader(t *testing.T) {
	base := &fakeHooks{phase: "minted-y", agent: "evolve-minted-y", model: "sonnet",
		prompt: "composed", verdict: core.VerdictPASS}
	hk := inlineHooks{fakeHooks: base, body: "", hasIt: false}
	fb := &fakeBridge{writeArtifact: "# y\n"}
	r := New(Options{Hooks: hk, Bridge: fb, Prompts: fakePromptsFS("evolve-minted-y", "DISK BODY")})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir(), ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
	if base.gotComposeBody != "DISK BODY" {
		t.Errorf("ComposePrompt body=%q, want the on-disk agent body", base.gotComposeBody)
	}
}
