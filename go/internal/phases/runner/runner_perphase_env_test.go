package runner

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRun_ThePerAgentModelEnvKeyIsNotRead(t *testing.T) {
	cases := []struct {
		name        string
		phase       string
		agentPrompt string
		envKey      string
		want        string
	}{
		{
			name:        "tdd phase / tdd-engineer agent — EVOLVE_TDD_ENGINEER_MODEL is not read",
			phase:       "tdd",
			agentPrompt: "evolve-tdd-engineer",
			envKey:      "EVOLVE_TDD_ENGINEER_MODEL",
			want:        "marker-tdd",
		},
		{
			name:        "build phase / builder agent — EVOLVE_BUILDER_MODEL is not read",
			phase:       "build",
			agentPrompt: "evolve-builder",
			envKey:      "EVOLVE_BUILDER_MODEL",
			want:        "marker-build",
		},
		{
			name:        "audit phase / auditor agent — EVOLVE_AUDITOR_MODEL is not read",
			phase:       "audit",
			agentPrompt: "evolve-auditor",
			envKey:      "EVOLVE_AUDITOR_MODEL",
			want:        "marker-audit",
		},
		{
			name:        "retro phase / retrospective agent — EVOLVE_RETROSPECTIVE_MODEL is not read",
			phase:       "retro",
			agentPrompt: "evolve-retrospective",
			envKey:      "EVOLVE_RETROSPECTIVE_MODEL",
			want:        "marker-retro",
		},
		{
			name:        "scout phase / scout agent — EVOLVE_SCOUT_MODEL is not read",
			phase:       "scout",
			agentPrompt: "evolve-scout",
			envKey:      "EVOLVE_SCOUT_MODEL",
			want:        "marker-scout",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hooks := &fakeHooks{
				phase:   tc.phase,
				agent:   tc.agentPrompt,
				model:   "auto",
				verdict: core.VerdictPASS,
			}
			fb := &fakeBridge{writeArtifact: "x"}
			r := New(Options{
				Hooks:   hooks,
				Bridge:  fb,
				Prompts: fakePromptsFS(tc.agentPrompt, "x"),
			})

			_, err := r.Run(context.Background(), core.PhaseRequest{
				Cycle:       1,
				ProjectRoot: t.TempDir(),
				Workspace:   t.TempDir(),
				Env:         map[string]string{tc.envKey: tc.want},
			})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if fb.gotReq.Model == tc.want {
				t.Errorf("Model=%q came from the env key %s; a model override is the --model parameter or a policy pin", fb.gotReq.Model, tc.envKey)
			}
		})
	}
}
