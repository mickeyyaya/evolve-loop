package cmdutil

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

func TestNewPromptsLoader_RootsAtProjectRoot(t *testing.T) {
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

func TestNewPromptsLoader_TheEnvironmentNamesNoPromptDir(t *testing.T) {
	root := t.TempDir()
	override := t.TempDir()
	writeAgent(t, override, "evolve-override")
	t.Setenv("EVOLVE_PROMPTS_DIR", override)

	l := NewPromptsLoader(root)
	if _, err := l.Agent("evolve-override"); err == nil {
		t.Fatal("NewPromptsLoader read EVOLVE_PROMPTS_DIR; the personas come from the project root only (I-16, X18e)")
	}
	writeAgent(t, root, "evolve-project-only")
	if _, err := l.Agent("evolve-project-only"); err != nil {
		t.Errorf("Agent(evolve-project-only) from the project root: %v", err)
	}
}
