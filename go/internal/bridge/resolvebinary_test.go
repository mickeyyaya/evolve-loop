package bridge

import "testing"

// resolvebinary_test.go — the BRIDGE_TESTING offline binary-override seam
// (ported from the bash bridge). The override only applies under
// BRIDGE_TESTING=1 so a stray BRIDGE_*_BINARY can never redirect a real
// production launch.
func TestResolveBinary(t *testing.T) {
	cases := []struct {
		name        string
		env         map[string]string
		defaultName string
		want        string
	}{
		{
			name:        "testing off: override ignored",
			env:         map[string]string{"BRIDGE_CLAUDE_BINARY": "/fake/claude"},
			defaultName: "claude",
			want:        "claude",
		},
		{
			name:        "testing on: override applied",
			env:         map[string]string{"BRIDGE_TESTING": "1", "BRIDGE_CLAUDE_BINARY": "/fake/claude"},
			defaultName: "claude",
			want:        "/fake/claude",
		},
		{
			name:        "testing on: codex maps to BRIDGE_CODEX_BINARY",
			env:         map[string]string{"BRIDGE_TESTING": "1", "BRIDGE_CODEX_BINARY": "/fake/codex"},
			defaultName: "codex",
			want:        "/fake/codex",
		},
		{
			name:        "testing on: agy maps to BRIDGE_AGY_BINARY",
			env:         map[string]string{"BRIDGE_TESTING": "1", "BRIDGE_AGY_BINARY": "/fake/agy"},
			defaultName: "agy",
			want:        "/fake/agy",
		},
		{
			name:        "testing on, no override: default",
			env:         map[string]string{"BRIDGE_TESTING": "1"},
			defaultName: "claude",
			want:        "claude",
		},
		{
			name:        "testing on, empty override: default",
			env:         map[string]string{"BRIDGE_TESTING": "1", "BRIDGE_CLAUDE_BINARY": ""},
			defaultName: "claude",
			want:        "claude",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveBinary(Deps{LookupEnv: mapLookup(tc.env)}, tc.defaultName)
			if got != tc.want {
				t.Errorf("resolveBinary(%q) = %q, want %q", tc.defaultName, got, tc.want)
			}
		})
	}
}
