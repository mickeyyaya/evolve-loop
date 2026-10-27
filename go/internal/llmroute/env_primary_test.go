package llmroute

import "testing"

func TestEnvPrimary_ReadsOnlyTheGlobalCLIMarker(t *testing.T) {
	cases := []struct {
		name       string
		env        map[string]string
		wantCLI    string
		wantSource string
	}{
		{"global", map[string]string{"EVOLVE_TDD_ENGINEER_CLI": "claude-tmux", "EVOLVE_CLI": "agy-tmux"}, "agy-tmux", "env(EVOLVE_CLI)"},
		{"per-agent key only", map[string]string{"EVOLVE_TDD_ENGINEER_CLI": "codex-tmux"}, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("EVOLVE_CLI", "")
			cli, source := EnvPrimary(tc.env)
			if cli != tc.wantCLI || source != tc.wantSource {
				t.Errorf("EnvPrimary = (%q, %q), want (%q, %q)", cli, source, tc.wantCLI, tc.wantSource)
			}
		})
	}
}

func TestOverrideSource_NamesTheCLIParameter(t *testing.T) {
	if got := OverrideSource("tdd-engineer"); got != "--cli tdd-engineer" || !IsOperatorChoice(got) {
		t.Errorf("OverrideSource = %q, want an operator choice that names --cli tdd-engineer", got)
	}
}
