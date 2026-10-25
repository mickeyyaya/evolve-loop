package bridge

import (
	"slices"
	"strings"
	"testing"
)

func TestParseManifest_EnvPassthroughIsRead(t *testing.T) {
	t.Parallel()
	m, err := parseManifest("hypo", []byte(`{"cli":"hypo","binary":"h","env_passthrough":["HTTPS_PROXY","_ok_9"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.EnvPassthrough, []string{"HTTPS_PROXY", "_ok_9"}) {
		t.Fatalf("env_passthrough not read: %v", m.EnvPassthrough)
	}
}

func TestParseManifest_EnvPassthroughRefusesNonIdentifiersAndReservedNames(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"A B", "", "1ABC", "K=V", "ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL", "OPENAI_API_KEY", "EVOLVE_PROJECT_ROOT", "BRIDGE_TESTING"} {
		_, err := parseManifest("hypo", []byte(`{"cli":"hypo","binary":"h","env_passthrough":["`+name+`"]}`))
		if err == nil || !strings.Contains(err.Error(), "env_passthrough") {
			t.Fatalf("name %q must be refused by env_passthrough; got %v", name, err)
		}
	}
}

func TestRealize_EnvPassthroughIsCopiedIntoTheRealization(t *testing.T) {
	t.Parallel()
	m := Manifest{CLI: "hypo", EnvPassthrough: []string{"A", "B"}}
	got := Realize(m, LaunchIntent{})
	m.EnvPassthrough[0] = "changed"
	if !slices.Equal(got.EnvPassthrough, []string{"A", "B"}) {
		t.Fatalf("the realization must hold its own copy of env_passthrough; got %v", got.EnvPassthrough)
	}
}

func TestManifests_EachFamilyPassesItsConfigDirAndProxies(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		"claude-tmux": "CLAUDE_CONFIG_DIR", "claude-p": "CLAUDE_CONFIG_DIR",
		"codex": "CODEX_HOME", "codex-tmux": "CODEX_HOME",
		"agy": "XDG_CONFIG_HOME", "agy-tmux": "XDG_CONFIG_HOME", "agy-claude-tmux": "XDG_CONFIG_HOME",
		"ollama-tmux": "OLLAMA_HOST",
	}
	for _, cli := range manifestCLIs(t) {
		m, err := LoadManifest(cli)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{want[cli], "HTTPS_PROXY", "NO_PROXY"} {
			if !slices.Contains(m.EnvPassthrough, name) {
				t.Errorf("%s: env_passthrough %v lacks %s", cli, m.EnvPassthrough, name)
			}
		}
	}
}
