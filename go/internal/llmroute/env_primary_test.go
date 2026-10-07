package llmroute

import "testing"

func TestEnvPrimary_TheAgentKeyOutranksTheGlobalOverride(t *testing.T) {
	cases := []struct {
		name       string
		env        map[string]string
		wantCLI    string
		wantSource string
	}{
		{"agent key", map[string]string{"EVOLVE_TDD_ENGINEER_CLI": "claude-tmux", "EVOLVE_CLI": "agy-tmux"}, "claude-tmux", "env(EVOLVE_TDD_ENGINEER_CLI)"},
		{"global only", map[string]string{"EVOLVE_CLI": "agy-tmux"}, "agy-tmux", "env(EVOLVE_CLI)"},
		{"neither", map[string]string{"EVOLVE_OTHER_CLI": "codex-tmux"}, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("EVOLVE_CLI", "")
			cli, source := EnvPrimary("tdd-engineer", tc.env)
			if cli != tc.wantCLI || source != tc.wantSource {
				t.Errorf("EnvPrimary = (%q, %q), want (%q, %q)", cli, source, tc.wantCLI, tc.wantSource)
			}
		})
	}
}
