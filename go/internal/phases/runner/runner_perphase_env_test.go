package runner

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// runner_perphase_env_test.go — Bug A regression guard. cmd_loop.go writes
// per-agent model overrides as `EVOLVE_<AGENT>_MODEL` (e.g. `EVOLVE_BUILDER_
// MODEL`), matching the same convention as `EVOLVE_<AGENT>_CLI` and
// `EVOLVE_<AGENT>_PERMISSION_MODE` already use. The runner's model resolver
// at runner.go:284 had drifted to read `EVOLVE_<PHASE>_MODEL`, which silently
// dropped the override for every phase where phase ≠ agent (tdd/tdd-engineer,
// build/builder, audit/auditor, retro/retrospective). Cycle-124 V1 verification
// proved the drop in production: `--model builder=gpt-5.5` reached the loop
// dispatcher but never reached the runner, so codex fell back to the profile
// default (`sonnet` → `gpt-5.4`) and the operator's ChatGPT account 400'd.
// This table-driven test pins the agent-keyed contract — one row per
// known-mismatch phase pair.

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
		// Sanity row: phase == profileName works under BOTH the buggy and
		// fixed code paths. Kept so future refactors don't accidentally
		// regress the scout-style happy path.
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
