package bridge

import "testing"

func TestResolveBinary_TakesTheInjectedBinaryOfTheCLIOrItsDefaultName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		binaries    map[string]string
		env         map[string]string
		defaultName string
		want        string
	}{
		{name: "no injected binary: default", defaultName: "claude", want: "claude"},
		{name: "injected binary of the CLI", binaries: map[string]string{"claude": "/fake/claude"}, defaultName: "claude", want: "/fake/claude"},
		{name: "another CLI keeps its default", binaries: map[string]string{"claude": "/fake/claude"}, defaultName: "codex", want: "codex"},
		{name: "empty injected binary: default", binaries: map[string]string{"agy": ""}, defaultName: "agy", want: "agy"},
		{name: "the environment never names a binary", env: map[string]string{"BRIDGE_TESTING": "1", "BRIDGE_CLAUDE_BINARY": "/env/claude"}, defaultName: "claude", want: "claude"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveBinary(Deps{Binaries: tc.binaries, Env: tc.env, LookupEnv: mapLookup(tc.env)}, tc.defaultName)
			if got != tc.want {
				t.Errorf("resolveBinary(%q) = %q, want %q", tc.defaultName, got, tc.want)
			}
		})
	}
}
