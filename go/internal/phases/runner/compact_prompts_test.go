package runner

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
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

func TestRun_ActiveExplanationContractAppendsExecutableReferenceAfterCompaction(t *testing.T) {
	loader := prompts.NewFromFS(fstest.MapFS{
		"agents/evolve-builder.md":           {Data: []byte("---\nname: evolve-builder\n---\n# Builder\n\n## Reference Index\ntrimmed\n")},
		"agents/evolve-builder-reference.md": {Data: []byte("---\nname: evolve-builder-reference\n---\n# Ref\n\n## Section: explanation-documentation-contract\nCANONICAL-PATH docs/explain/builds/cycle-<cycle>-<run>.md\n\n## Section: other\nignore\n")},
		"agents/evolve-auditor.md":           {Data: []byte("---\nname: evolve-auditor\n---\n# Auditor\n\n## Reference Index\ntrimmed\n")},
		"agents/evolve-auditor-reference.md": {Data: []byte("---\nname: evolve-auditor-reference\n---\n# Ref\n\n## Section: explanation-documentation-review\nREVIEW-EVIDENCE implementation and tests\n\n## Section: other\nignore\n")},
	})
	for _, tc := range []struct{ phase, agent, required string }{
		{phase: "build", agent: "evolve-builder", required: "CANONICAL-PATH docs/explain/builds"},
		{phase: "audit", agent: "evolve-auditor", required: "REVIEW-EVIDENCE implementation and tests"},
	} {
		t.Run(tc.phase, func(t *testing.T) {
			baseHooks := &fakeHooks{phase: tc.phase, agent: tc.agent, model: "auto", verdict: core.VerdictPASS}
			hooks := bodyPromptHooks{fakeHooks: baseHooks}
			bridge := &fakeBridge{writeArtifact: "x"}
			runner := New(Options{Hooks: hooks, Bridge: bridge, Prompts: loader, CompactPrompts: true})
			if _, err := runner.Run(context.Background(), core.PhaseRequest{
				ProjectRoot: t.TempDir(), Workspace: t.TempDir(), ExplanationDocumentationVersion: 1,
			}); err != nil {
				t.Fatalf("Run: %v", err)
			}
			if !strings.Contains(bridge.gotReq.Prompt, tc.required) || strings.Contains(bridge.gotReq.Prompt, "Section: other") {
				t.Fatalf("dispatched compact %s prompt lacks the selected executable reference section: %q", tc.phase, bridge.gotReq.Prompt)
			}
		})
	}
}

type bodyPromptHooks struct{ *fakeHooks }

func (h bodyPromptHooks) ComposePrompt(body string, req core.PhaseRequest) string {
	h.gotComposeBody = body
	h.gotComposeReq = req
	return body
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
