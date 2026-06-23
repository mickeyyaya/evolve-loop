package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// compact_prompts_test.go — contract for prompt-ondemand-section-strip (runner half).
//
// When Options.CompactPrompts is true, BaseRunner strips the on-demand
// "## Reference Index" tail from the DISK-loaded agent body BEFORE ComposePrompt,
// shrinking the dispatched prompt. Default false preserves the body
// byte-for-byte (the safe boundary in the design's Risks section). Inline
// prompt bodies (minted/spec phases) are NEVER stripped (R7).
//
// The fakeHooks.ComposePrompt records gotComposeBody — exactly what the runner
// hands the hook — so these assertions read the real composed input, not source.

// agentDocBody is a fixture agent doc whose tail is an on-demand reference
// section. The loader returns this verbatim as Prompt.Body.
const agentDocBody = "# Builder\n\nDo the work.\n\n## Reference Index\n\n- ref one\n- ref two\n"

// agentDocStripped is agentDocBody with the reference section removed.
const agentDocStripped = "# Builder\n\nDo the work.\n\n"

func runCompact(t *testing.T, compactPrompts bool) *fakeHooks {
	t.Helper()
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "auto", prompt: "composed", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "x"}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-builder", agentDocBody), CompactPrompts: compactPrompts})
	if _, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: t.TempDir(), Workspace: t.TempDir(),
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return hooks
}

// TestRun_CompactPrompts_StripsDiskBody — with CompactPrompts=true the
// reference tail is gone and the composed body is strictly shorter than the
// full agent body.
func TestRun_CompactPrompts_StripsDiskBody(t *testing.T) {
	hooks := runCompact(t, true)
	if strings.Contains(hooks.gotComposeBody, "## Reference Index") {
		t.Errorf("compact mode left the reference section in the body; got %q", hooks.gotComposeBody)
	}
	if hooks.gotComposeBody != agentDocStripped {
		t.Errorf("compact body = %q, want %q", hooks.gotComposeBody, agentDocStripped)
	}
	if len(hooks.gotComposeBody) >= len(agentDocBody) {
		t.Errorf("compact body len=%d must be < full body len=%d", len(hooks.gotComposeBody), len(agentDocBody))
	}
}

// TestRun_CompactPrompts_DefaultUnchanged — CompactPrompts=false leaves the
// disk-loaded body byte-for-byte identical (no silent behavior change).
func TestRun_CompactPrompts_DefaultUnchanged(t *testing.T) {
	hooks := runCompact(t, false)
	if hooks.gotComposeBody != agentDocBody {
		t.Errorf("compact off: composed body = %q, want full body %q (byte-for-byte)", hooks.gotComposeBody, agentDocBody)
	}
}

// TestRun_CompactPrompts_InlineBodyNotStripped — compact mode targets
// disk-loaded agent docs; an inline body (minted/spec phase, supplied via the
// existing inlineHooks/InlinePromptProvider seam) keeps its reference section
// intact even with CompactPrompts=true (R7).
func TestRun_CompactPrompts_InlineBodyNotStripped(t *testing.T) {
	base := &fakeHooks{phase: "spec", agent: "evolve-spec", model: "auto", prompt: "composed", verdict: core.VerdictPASS}
	hk := inlineHooks{fakeHooks: base, body: agentDocBody, hasIt: true}
	fb := &fakeBridge{writeArtifact: "x"}
	r := New(Options{Hooks: hk, Bridge: fb, Prompts: emptyPromptsFS(), CompactPrompts: true})
	if _, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: t.TempDir(), Workspace: t.TempDir(),
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if base.gotComposeBody != agentDocBody {
		t.Errorf("inline body must NOT be compacted; got %q, want %q", base.gotComposeBody, agentDocBody)
	}
}
