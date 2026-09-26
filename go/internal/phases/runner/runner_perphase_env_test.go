package runner

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRun_PerAgentModelEnvKey_AgentKeyedNotPhaseKeyed(t *testing.T) {
	cases := []struct {
		name        string
		phase       string // PhaseName() — what the runner thinks the phase is
		agentPrompt string // AgentPromptName() — `evolve-<profile>`
		envKey      string // the env var cmd_loop.go writes
		want        string // the model value the runner MUST resolve to
	}{
		{
			name:        "tdd phase / tdd-engineer agent — EVOLVE_TDD_ENGINEER_MODEL beats profile default",
			phase:       "tdd",
			agentPrompt: "evolve-tdd-engineer",
			envKey:      "EVOLVE_TDD_ENGINEER_MODEL",
			want:        "marker-tdd",
		},
		{
			name:        "build phase / builder agent — EVOLVE_BUILDER_MODEL beats profile default",
			phase:       "build",
			agentPrompt: "evolve-builder",
			envKey:      "EVOLVE_BUILDER_MODEL",
			want:        "marker-build",
		},
		{
			name:        "audit phase / auditor agent — EVOLVE_AUDITOR_MODEL beats profile default",
			phase:       "audit",
			agentPrompt: "evolve-auditor",
			envKey:      "EVOLVE_AUDITOR_MODEL",
			want:        "marker-audit",
		},
		{
			name:        "retro phase / retrospective agent — EVOLVE_RETROSPECTIVE_MODEL beats profile default",
			phase:       "retro",
			agentPrompt: "evolve-retrospective",
			envKey:      "EVOLVE_RETROSPECTIVE_MODEL",
			want:        "marker-retro",
		},
		{
			name:        "scout phase / scout agent (sanity, phase == profileName)",
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
				model:   "auto", // simulates the production "auto" sentinel
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
			if fb.gotReq.Model != tc.want {
				t.Errorf("Model=%q, want %q — env key %s was either silently dropped or overridden by profile default", fb.gotReq.Model, tc.want, tc.envKey)
			}
		})
	}
}
