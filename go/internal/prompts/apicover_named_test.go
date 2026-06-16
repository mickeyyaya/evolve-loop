package prompts

import (
	"os"
	"path/filepath"
	"testing"
)

// apicover_named_test.go — public-API coverage (ADR-0050 Phase 5). Names and
// exercises exported symbols apicover flagged uncovered in this package:
//   - func NewForProject (prompts.go)
//   - type Loader (prompts.go)
//   - type Prompt (prompts.go)
// Each test asserts a real contract (Rule 9), not a no-op reference.

// seedAgentDir writes a project layout (agents/<name>.md) on disk and returns
// the project root.
func seedAgentDir(t *testing.T, name, contents string) string {
	t.Helper()
	root := t.TempDir()
	agentsDir := filepath.Join(root, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, name+".md"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

const namedAgent = `---
name: evolve-scout
description: Discovery agent
model: tier-2
---

# Evolve Scout

Body content.
`

// TestNewForProject_OverrideBranch exercises the EVOLVE_PROMPTS_DIR-override
// branch: the loader resolves agents from the override dir, not projectRoot.
func TestNewForProject_OverrideBranch(t *testing.T) {
	overrideRoot := seedAgentDir(t, "evolve-scout", namedAgent)
	emptyProject := t.TempDir() // no agents/ here — proves the override wins

	t.Setenv("EVOLVE_PROMPTS_DIR", overrideRoot)
	var l *Loader = NewForProject(emptyProject)
	if l == nil {
		t.Fatal("NewForProject returned nil")
	}
	p, err := l.Agent("evolve-scout")
	if err != nil {
		t.Fatalf("Agent via override dir: %v", err)
	}
	if p.Name != "evolve-scout" {
		t.Errorf("Prompt.Name = %q, want evolve-scout", p.Name)
	}
}

// TestNewForProject_UnsetBranch exercises the unset branch: with
// EVOLVE_PROMPTS_DIR cleared, the loader resolves from projectRoot.
func TestNewForProject_UnsetBranch(t *testing.T) {
	root := seedAgentDir(t, "evolve-builder", "---\nname: evolve-builder\ndescription: builder\n---\nBody.")
	os.Unsetenv("EVOLVE_PROMPTS_DIR")

	l := NewForProject(root)
	if l == nil {
		t.Fatal("NewForProject returned nil")
	}
	p, err := l.Agent("evolve-builder")
	if err != nil {
		t.Fatalf("Agent from projectRoot: %v", err)
	}
	if p.Name != "evolve-builder" {
		t.Errorf("Prompt.Name = %q, want evolve-builder", p.Name)
	}
}

// TestLoaderAndPrompt_PublicFields binds a *Loader and a Prompt and asserts the
// Prompt's public fields (Name/Body/Raw/Frontmatter) are populated from the
// parsed source.
func TestLoaderAndPrompt_PublicFields(t *testing.T) {
	dir := seedAgentDir(t, "evolve-scout", namedAgent)

	var l *Loader = NewFromDir(dir)
	if l == nil {
		t.Fatal("NewFromDir returned nil")
	}

	var p Prompt
	p, err := l.Agent("evolve-scout")
	if err != nil {
		t.Fatalf("Agent: %v", err)
	}

	if p.Name != "evolve-scout" {
		t.Errorf("Prompt.Name = %q, want evolve-scout", p.Name)
	}
	if p.Frontmatter == nil {
		t.Fatal("Prompt.Frontmatter is nil; want parsed map")
	}
	if got := p.Frontmatter["model"]; got != "tier-2" {
		t.Errorf("Prompt.Frontmatter[model] = %v, want tier-2", got)
	}
	if p.Body == "" {
		t.Error("Prompt.Body empty; want content after the frontmatter fence")
	}
	// Body is the content after the closing fence — must not retain the fence.
	if want := "# Evolve Scout"; !contains(p.Body, want) {
		t.Errorf("Prompt.Body = %q, want it to contain %q", p.Body, want)
	}
	// Raw is the verbatim file — must retain the frontmatter fence.
	if !contains(p.Raw, "---") || !contains(p.Raw, "name: evolve-scout") {
		t.Errorf("Prompt.Raw = %q, want verbatim source including frontmatter", p.Raw)
	}
}
