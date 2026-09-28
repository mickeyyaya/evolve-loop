package runner

import (
	"context"
	"errors"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

type sandboxRecordingBridge struct {
	*scriptedBridge
	sandboxed []bool
}

func (b *sandboxRecordingBridge) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.sandboxed = append(b.sandboxed, req.RequireSandbox)
	return b.scriptedBridge.Launch(ctx, req)
}

func explanationContractPrompts() *prompts.Loader {
	return prompts.NewFromFS(fstest.MapFS{
		"agents/evolve-builder.md":           {Data: []byte("---\nname: evolve-builder\n---\nx")},
		"agents/evolve-builder-reference.md": {Data: []byte("---\nname: evolve-builder-reference\n---\n# Ref\n\n## Section: explanation-documentation-contract\nCANONICAL-PATH\n")},
		"agents/evolve-auditor.md":           {Data: []byte("---\nname: evolve-auditor\n---\nx")},
		"agents/evolve-auditor-reference.md": {Data: []byte("---\nname: evolve-auditor-reference\n---\n# Ref\n\n## Section: explanation-documentation-review\nREVIEW-EVIDENCE\n")},
	})
}

func sandboxRequirementPerRung(t *testing.T, phase, agent string, version int) []bool {
	t.Helper()
	hooks := &fakeHooks{phase: phase, agent: agent, model: "sonnet", prompt: "x", verdict: core.VerdictPASS}
	br := &sandboxRecordingBridge{scriptedBridge: &scriptedBridge{responses: map[string]scriptedResp{
		"codex-tmux":  {resp: core.BridgeResponse{ExitCode: 80, Stderr: "REPL boot timeout"}, err: errors.New("bridge: launch exit=80")},
		"claude-tmux": {},
	}}}
	root := writeFallbackProfile(t, agent, "codex-tmux", []string{"claude-tmux"})
	r := New(Options{Hooks: hooks, Bridge: br, Prompts: explanationContractPrompts()})
	req := core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir(), ExplanationDocumentationVersion: version}
	if _, err := r.Run(context.Background(), req); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return br.sandboxed
}

func TestRun_AVersionedBuildRequiresTheSandboxOnEveryRung(t *testing.T) {
	cases := []struct {
		name, phase, agent string
		version            int
		want               []bool
	}{
		{"a versioned build", "build", "evolve-builder", 1, []bool{true, true}},
		{"an unversioned build", "build", "evolve-builder", 0, []bool{false, false}},
		{"a versioned audit", "audit", "evolve-auditor", 1, []bool{false, false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sandboxRequirementPerRung(t, tc.phase, tc.agent, tc.version); !slices.Equal(got, tc.want) {
				t.Errorf("RequireSandbox per rung = %v, want %v", got, tc.want)
			}
		})
	}
}
