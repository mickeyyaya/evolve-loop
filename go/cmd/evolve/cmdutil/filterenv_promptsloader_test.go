package cmdutil

// filterenv_promptsloader_test.go — RED contract for cycle-549's
// cli-command-layer-test-coverage task (see cmd_worktree_test.go's package
// doc in cmd/evolve for the full task/lane background). FilterEvolveEnv and
// NewPromptsLoader had ZERO direct test coverage (0.0% per `go tool
// cover -func`).

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFilterEvolveEnv(t *testing.T) {
	cases := []struct {
		name    string
		environ []string
		want    map[string]string
	}{
		{
			"mixed_prefixes",
			[]string{
				"EVOLVE_CYCLE=42",
				"BRIDGE_SOCKET=/tmp/x",
				"PATH=/usr/bin",
				"HOME=/root",
			},
			map[string]string{"EVOLVE_CYCLE": "42", "BRIDGE_SOCKET": "/tmp/x"},
		},
		{"no_matches", []string{"PATH=/usr/bin", "HOME=/root"}, map[string]string{}},
		{"empty_environ", nil, map[string]string{}},
		// Negative/edge: malformed entries (no '=', or an empty key) are skipped,
		// not panicked on or admitted with a garbage key.
		{"no_equals_skipped", []string{"EVOLVE_BROKEN"}, map[string]string{}},
		{"empty_key_skipped", []string{"=noKey"}, map[string]string{}},
		{"value_with_embedded_equals", []string{"EVOLVE_X=a=b=c"}, map[string]string{"EVOLVE_X": "a=b=c"}},
		{"empty_value_kept", []string{"EVOLVE_EMPTY="}, map[string]string{"EVOLVE_EMPTY": ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FilterEvolveEnv(tc.environ)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("FilterEvolveEnv(%v) = %v, want %v", tc.environ, got, tc.want)
			}
		})
	}
}

// writeAgent creates <dir>/agents/<name>.md with minimal valid frontmatter.
func writeAgent(t *testing.T, dir, name string) {
	t.Helper()
	agentsDir := filepath.Join(dir, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("mkdir agents: %v", err)
	}
	body := "---\nname: " + name + "\n---\n\n# " + name + "\nbody\n"
	if err := os.WriteFile(filepath.Join(agentsDir, name+".md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write agent: %v", err)
	}
}

func TestNewPromptsLoader_RootsAtProjectRoot_WhenEnvUnset(t *testing.T) {
	t.Setenv("EVOLVE_PROMPTS_DIR", "")
	root := t.TempDir()
	writeAgent(t, root, "evolve-test")

	l := NewPromptsLoader(root)
	p, err := l.Agent("evolve-test")
	if err != nil {
		t.Fatalf("Agent(evolve-test) from project root: %v", err)
	}
	if p.Name != "evolve-test" {
		t.Errorf("loaded prompt name = %q, want evolve-test", p.Name)
	}
}

func TestNewPromptsLoader_HonorsEnvOverride(t *testing.T) {
	root := t.TempDir()     // has NO agents/ dir — must not be used
	override := t.TempDir() // has the real agents/ dir
	writeAgent(t, override, "evolve-override")
	t.Setenv("EVOLVE_PROMPTS_DIR", override)

	l := NewPromptsLoader(root)
	if _, err := l.Agent("evolve-override"); err != nil {
		t.Fatalf("Agent(evolve-override) via EVOLVE_PROMPTS_DIR override: %v", err)
	}
	// Negative: an agent that only exists under the (ignored) project root
	// must NOT be found once the override is set.
	writeAgent(t, root, "evolve-project-only")
	if _, err := l.Agent("evolve-project-only"); err == nil {
		t.Error("Agent(evolve-project-only) unexpectedly found — NewPromptsLoader must prefer EVOLVE_PROMPTS_DIR over projectRoot")
	}
}
