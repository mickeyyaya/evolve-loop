package detectcli

import (
	"errors"
	"testing"
)

func TestDetect_AllProbes(t *testing.T) {
	cases := []struct {
		name     string
		env      map[string]string
		hasAgy   bool
		wantCLI  string
		wantReas string
	}{
		{
			name:     "explicit_override",
			env:      map[string]string{"EVOLVE_PLATFORM": "custom"},
			wantCLI:  "custom",
			wantReas: "explicit override via EVOLVE_PLATFORM",
		},
		{
			name:     "claude_interactive",
			env:      map[string]string{"CLAUDE_CODE_INTERACTIVE": "1"},
			wantCLI:  "claude",
			wantReas: "CLAUDE_CODE_* env detected",
		},
		{
			name:     "claude_session_id",
			env:      map[string]string{"CLAUDE_CODE_SESSION_ID": "abc"},
			wantCLI:  "claude",
			wantReas: "CLAUDE_CODE_* env detected",
		},
		{
			name:     "gemini_cli",
			env:      map[string]string{"GEMINI_CLI": "1"},
			wantCLI:  "gemini",
			wantReas: "GEMINI_* env detected",
		},
		{
			name:     "gemini_api_key",
			env:      map[string]string{"GEMINI_API_KEY": "x"},
			wantCLI:  "gemini",
			wantReas: "GEMINI_* env detected",
		},
		{
			name:     "codex_home",
			env:      map[string]string{"CODEX_HOME": "/x"},
			wantCLI:  "codex",
			wantReas: "CODEX_* env detected",
		},
		{
			name:     "codex_api_key",
			env:      map[string]string{"CODEX_API_KEY": "x"},
			wantCLI:  "codex",
			wantReas: "CODEX_* env detected",
		},
		{
			name:     "antigravity_agy_on_path",
			env:      map[string]string{},
			hasAgy:   true,
			wantCLI:  "antigravity",
			wantReas: "agy binary on PATH detected",
		},
		{
			name:     "unknown_fallback",
			env:      map[string]string{},
			wantCLI:  "unknown",
			wantReas: "no probe matched",
		},
		{
			name:     "explicit_override_beats_claude",
			env:      map[string]string{"EVOLVE_PLATFORM": "gemini", "CLAUDE_CODE_SESSION_ID": "x"},
			wantCLI:  "gemini",
			wantReas: "explicit override via EVOLVE_PLATFORM",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := c.env
			r := Detect(Options{
				Env: func(name string) string { return env[name] },
				LookPath: func(name string) (string, error) {
					if c.hasAgy && name == "agy" {
						return "/usr/local/bin/agy", nil
					}
					return "", errors.New("not found")
				},
			})
			if r.CLI != c.wantCLI {
				t.Errorf("CLI=%q want %q", r.CLI, c.wantCLI)
			}
			if r.Reason != c.wantReas {
				t.Errorf("Reason=%q want %q", r.Reason, c.wantReas)
			}
		})
	}
}

// TestDetect_ZeroValueOptions exercises the default-fall-through branches
// (production defaults; no overrides).
func TestDetect_ZeroValueOptions(t *testing.T) {
	r := Detect(Options{})
	// Result depends on the test environment — just verify a CLI is set.
	if r.CLI == "" {
		t.Errorf("empty CLI")
	}
	if r.Reason == "" {
		t.Errorf("empty Reason")
	}
}
