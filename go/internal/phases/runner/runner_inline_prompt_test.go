package runner

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

// inlineHooks wraps fakeHooks and additionally satisfies the optional
// InlinePromptProvider interface. It models a minted phase that ships its
// prompt in-band rather than as an agents/<name>.md file on disk.
//
// fakeHooks is embedded by POINTER on purpose: ComposePrompt records the
// body it received into gotComposeBody, and the test reads that field back
// through the same pointer after Run. A value embed would record into a copy
// and the assertions would silently see the zero value.
type inlineHooks struct {
	*fakeHooks
	body  string
	hasIt bool
}

func (h inlineHooks) InlinePromptBody() (string, bool) { return h.body, h.hasIt }

// emptyPromptsFS returns a loader backed by an empty filesystem: ANY
// prompts.Agent(name) call returns an error. A phase that loads from disk
// fails; a phase that uses an inline body never touches the loader.
func emptyPromptsFS() *prompts.Loader {
	return prompts.NewFromFS(fstest.MapFS{})
}

// TestRun_InlinePrompt_UsesBodyAndSkipsLoader proves that when the hooks
// supply an inline prompt body, BaseRunner composes from it and never reads
// agents/<name>.md (the loader here has no file, so a disk read would error).
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

// TestRun_InlinePrompt_EmptyBodyButOptedIn_SkipsLoader locks the generic
// runner contract: ok=true wins even when the body is empty. The provider
// opted in, so the loader is NOT consulted (empty FS would error) and an
// empty body is composed. specrunner never produces this shape, but the
// shared runner must honor it for any future InlinePromptProvider.
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

// TestRun_InlinePrompt_EmptyFallsBackToLoader proves the inline path is
// opt-in PER CALL: a hooks impl that satisfies the interface but returns
// (\"\", false) still loads agents/<name>.md — byte-identical to the legacy
// path. Here the loader HAS the file, so the load must succeed.
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
