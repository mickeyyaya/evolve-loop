package runner

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRunner_OverlaySignalsRideThePhaseRequest(t *testing.T) {
	document := map[string]string{config.SignalDeliverableKind: "document"}
	for _, tc := range []struct {
		name    string
		phase   string
		agent   string
		signals map[string]string
		want    string
	}{
		{"scout/document", "scout", "evolve-scout", document, "solution-scout"},
		{"build/document", "build", "evolve-builder", document, "solution-build"},
		{"build/code", "build", "evolve-builder", map[string]string{config.SignalDeliverableKind: "code"}, ""},
		{"build/no signals", "build", "evolve-builder", nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := writeFallbackProfile(t, tc.agent, "claude-tmux", nil)
			hooks := &fakeHooks{phase: tc.phase, agent: tc.agent, model: "auto", prompt: "x", verdict: core.VerdictPASS}
			fb := &fakeBridge{writeArtifact: "x"}
			r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS(tc.agent, "x")})
			req := core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir(), Signals: tc.signals}
			if _, err := r.Run(context.Background(), req); err != nil {
				t.Fatalf("Run: %v", err)
			}
			if tc.want == "" {
				if len(fb.gotReq.Skills) != 0 {
					t.Fatalf("Skills=%v, want none", fb.gotReq.Skills)
				}
				return
			}
			found := false
			for _, s := range fb.gotReq.Skills {
				if s == tc.want {
					found = true
				}
			}
			if !found {
				t.Errorf("Skills=%v, want %s", fb.gotReq.Skills, tc.want)
			}
		})
	}
}
