package runner

import (
	"context"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRunner_TheWriteAxisOfTheRequestSelectsEngineeringCraft(t *testing.T) {
	code := map[string]string{config.SignalDeliverableKind: config.DeliverableKindCode}
	for _, tc := range []struct {
		name     string
		phase    string
		agent    string
		readOnly bool
		writable []string
		want     []string
	}{
		{"balanced build", "build", "evolve-builder", false, nil, []string{"engineering-craft", "code-review-simplify"}},
		{"balanced tdd", "tdd", "evolve-tdd-engineer", false, nil, []string{"engineering-craft", "code-review-simplify"}},
		{"debugger resolving its conflicts", "debugger", "evolve-debugger", true, []string{"go/internal/x.go"}, []string{"engineering-craft", "code-review-simplify"}},
		{"fenced scout", "scout", "evolve-scout", true, nil, nil},
		{"fenced triage", "triage", "evolve-triage", true, nil, nil},
		{"fenced audit", "audit", "evolve-auditor", true, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := writeFallbackProfile(t, tc.agent, "claude-tmux", nil)
			hooks := &fakeHooks{phase: tc.phase, agent: tc.agent, model: "auto", prompt: "x", verdict: core.VerdictPASS}
			fb := &fakeBridge{writeArtifact: "x"}
			r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS(tc.agent, "x")})
			req := core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir(), Signals: code, WorktreeReadOnly: tc.readOnly, WorktreeWritablePaths: tc.writable}
			if _, err := r.Run(context.Background(), req); err != nil {
				t.Fatalf("Run: %v", err)
			}
			if fb.gotReq.Model != "sonnet" {
				t.Fatalf("precondition: dispatched Model=%q, want sonnet (a non-deep tier)", fb.gotReq.Model)
			}
			if len(fb.gotReq.Skills) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(fb.gotReq.Skills, tc.want) {
				t.Errorf("Skills=%v, want %v", fb.gotReq.Skills, tc.want)
			}
		})
	}
}
