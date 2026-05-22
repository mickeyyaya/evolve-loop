// Package prompts loads agent/skill markdown with YAML frontmatter.
// This test file pins the front-matter parsing semantics and the
// fs.FS-backed lookup contract that phase impls will rely on.
package prompts

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"testing/fstest"
)

// sampleAgent mirrors the real agents/evolve-scout.md frontmatter shape:
// flat string fields, an array of identifiers, quoted long strings with
// embedded em-dashes, and an unquoted multi-word description.
const sampleAgent = `---
name: evolve-scout
description: Discovery and planning agent for the Evolve Loop. Scans codebase.
model: tier-2
capabilities: [file-read, search, shell]
tools: ["Read", "Grep", "Glob", "Bash"]
perspective: "discovery + risk surface mapping"
---

# Evolve Scout

Body goes here.
`

const sampleSkill = `---
name: evolve-loop
description: Skill description
argument-hint: "[--budget-usd N | --cycles N | --resume]"
---

# Evolve Loop

The body.
`

// fixtureFS builds a virtual filesystem matching the repo layout
// (agents/<name>.md + skills/<name>/SKILL.md) so the loader can be
// exercised without touching the real .md files.
func fixtureFS() fstest.MapFS {
	return fstest.MapFS{
		"agents/evolve-scout.md":         &fstest.MapFile{Data: []byte(sampleAgent)},
		"agents/evolve-builder.md":       &fstest.MapFile{Data: []byte("---\nname: evolve-builder\ndescription: builder\n---\nBody.")},
		"skills/evolve-loop/SKILL.md":    &fstest.MapFile{Data: []byte(sampleSkill)},
		"skills/santa-loop/SKILL.md":     &fstest.MapFile{Data: []byte("---\nname: santa-loop\ndescription: santa\n---\nBody.")},
		"skills/empty/notSkillFile.md":   &fstest.MapFile{Data: []byte("ignored")}, // must not appear in Skills()
	}
}

// TestNewFromFS_Agent_HappyPath verifies the full read+parse path for
// a typical agent file. Body must be content AFTER the closing --- of
// the frontmatter block.
func TestNewFromFS_Agent_HappyPath(t *testing.T) {
	l := NewFromFS(fixtureFS())
	p, err := l.Agent("evolve-scout")
	if err != nil {
		t.Fatalf("Agent: %v", err)
	}
	if p.Name != "evolve-scout" {
		t.Errorf("Name=%q, want evolve-scout", p.Name)
	}
	if p.Frontmatter["model"] != "tier-2" {
		t.Errorf("Frontmatter[model]=%v, want tier-2", p.Frontmatter["model"])
	}
	if desc, _ := p.Frontmatter["description"].(string); desc == "" {
		t.Errorf("Frontmatter[description] missing/empty: %v", p.Frontmatter["description"])
	}
	caps, ok := p.Frontmatter["capabilities"].([]string)
	if !ok {
		t.Fatalf("capabilities not []string, got %T", p.Frontmatter["capabilities"])
	}
	wantCaps := []string{"file-read", "search", "shell"}
	if !reflect.DeepEqual(caps, wantCaps) {
		t.Errorf("capabilities=%v, want %v", caps, wantCaps)
	}
	// Body must start with the heading line, not contain any "---".
	if p.Body == "" {
		t.Error("Body empty")
	}
	if !contains(p.Body, "# Evolve Scout") {
		t.Errorf("Body missing heading: %q", p.Body[:min(60, len(p.Body))])
	}
}

// TestNewFromFS_Skill_PathConvention verifies skills/<name>/SKILL.md
// resolution (NOT skills/<name>.md). Mirrors the repo convention.
func TestNewFromFS_Skill_PathConvention(t *testing.T) {
	l := NewFromFS(fixtureFS())
	p, err := l.Skill("evolve-loop")
	if err != nil {
		t.Fatalf("Skill: %v", err)
	}
	if p.Name != "evolve-loop" {
		t.Errorf("Name=%q, want evolve-loop", p.Name)
	}
	if !contains(p.Body, "# Evolve Loop") {
		t.Errorf("Body missing skill heading: %q", p.Body[:min(60, len(p.Body))])
	}
}

// TestNewFromFS_Agent_NotFound returns fs.ErrNotExist-wrapped error.
func TestNewFromFS_Agent_NotFound(t *testing.T) {
	l := NewFromFS(fixtureFS())
	_, err := l.Agent("nonexistent")
	if err == nil {
		t.Fatal("want error for missing agent")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err=%v, want errors.Is fs.ErrNotExist", err)
	}
}

// TestAgents_SortedList — discovery surface for the orchestrator.
// Skipped helper files like .DS_Store (not in fixture, but loader
// must accept only *.md). Order: sorted by name.
func TestAgents_SortedList(t *testing.T) {
	l := NewFromFS(fixtureFS())
	names, err := l.Agents()
	if err != nil {
		t.Fatalf("Agents: %v", err)
	}
	want := []string{"evolve-builder", "evolve-scout"}
	sort.Strings(want) // defensive — verify sort property
	if !reflect.DeepEqual(names, want) {
		t.Errorf("Agents()=%v, want %v", names, want)
	}
}

// TestSkills_OnlySKILL_Md — skills/ must only enumerate directories
// containing a SKILL.md file. The "empty/notSkillFile.md" entry in
// fixtureFS must be ignored.
func TestSkills_OnlySKILL_Md(t *testing.T) {
	l := NewFromFS(fixtureFS())
	names, err := l.Skills()
	if err != nil {
		t.Fatalf("Skills: %v", err)
	}
	want := []string{"evolve-loop", "santa-loop"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("Skills()=%v, want %v (sorted, no skills/empty)", names, want)
	}
}

// TestParseFrontmatter_NoFrontmatter — files without --- markers
// return nil map and the entire content as Body.
func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	raw := "# Just a heading\n\nNo frontmatter here."
	fm, body, err := ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if fm != nil {
		t.Errorf("Frontmatter=%v, want nil", fm)
	}
	if body != raw {
		t.Errorf("Body=%q, want full raw", body)
	}
}

// TestParseFrontmatter_EmptyBlock — opening and closing --- with no
// content between → empty map + body.
func TestParseFrontmatter_EmptyBlock(t *testing.T) {
	raw := "---\n---\n\nbody"
	fm, body, err := ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if fm == nil || len(fm) != 0 {
		t.Errorf("Frontmatter=%v, want empty map", fm)
	}
	if body != "\nbody" {
		t.Errorf("Body=%q, want '\\nbody'", body)
	}
}

// TestParseFrontmatter_QuotedStrings — both single and double quotes
// strip cleanly; embedded em-dashes survive.
func TestParseFrontmatter_QuotedStrings(t *testing.T) {
	raw := "---\nperspective: \"a — b\"\noutput: 'plain'\n---\nbody"
	fm, _, err := ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if fm["perspective"] != "a — b" {
		t.Errorf("perspective=%v, want 'a — b'", fm["perspective"])
	}
	if fm["output"] != "plain" {
		t.Errorf("output=%v, want 'plain'", fm["output"])
	}
}

// TestParseFrontmatter_InlineArray — bracketed arrays with mixed
// quoted/unquoted elements both parse to []string.
func TestParseFrontmatter_InlineArray(t *testing.T) {
	raw := "---\ntools: [\"Read\", \"Grep\", Bash]\nnums: []\n---\nbody"
	fm, _, err := ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	tools, ok := fm["tools"].([]string)
	if !ok {
		t.Fatalf("tools not []string: %T", fm["tools"])
	}
	want := []string{"Read", "Grep", "Bash"}
	if !reflect.DeepEqual(tools, want) {
		t.Errorf("tools=%v, want %v", tools, want)
	}
	nums, ok := fm["nums"].([]string)
	if !ok {
		t.Fatalf("nums not []string: %T", fm["nums"])
	}
	if len(nums) != 0 {
		t.Errorf("nums=%v, want []", nums)
	}
}

// TestParseFrontmatter_ColonInValue — descriptions often contain
// colons (e.g., "Phase X: do Y"). The first colon must split key/value,
// remaining colons stay in the value.
func TestParseFrontmatter_ColonInValue(t *testing.T) {
	raw := "---\ndescription: Phase 2: do Y\n---\nbody"
	fm, _, err := ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if fm["description"] != "Phase 2: do Y" {
		t.Errorf("description=%v, want 'Phase 2: do Y'", fm["description"])
	}
}

// TestParseFrontmatter_UnterminatedBlock — opening --- with no closing
// returns an error so callers can surface clearly.
func TestParseFrontmatter_UnterminatedBlock(t *testing.T) {
	raw := "---\nname: foo\n(no closing fence)\n"
	_, _, err := ParseFrontmatter(raw)
	if err == nil {
		t.Error("ParseFrontmatter: want error for unterminated frontmatter")
	}
}

// TestParseFrontmatter_BlankAndCommentLines — blank lines and #-comments
// inside frontmatter must be skipped (don't crash, don't create empty
// keys).
func TestParseFrontmatter_BlankAndCommentLines(t *testing.T) {
	raw := "---\n# This is a comment\nname: foo\n\ndescription: bar\n---\nbody"
	fm, _, err := ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if fm["name"] != "foo" {
		t.Errorf("name=%v, want foo", fm["name"])
	}
	if fm["description"] != "bar" {
		t.Errorf("description=%v, want bar", fm["description"])
	}
	if _, present := fm[""]; present {
		t.Errorf("blank/comment line produced empty-string key: %v", fm)
	}
}

// TestNewFromDir_ReadsFromDisk — the dev-override path
// ($EVOLVE_PROMPTS_DIR) uses os.DirFS under the hood. Verify the disk
// round-trip works for at least one real agent file.
func TestNewFromDir_ReadsFromDisk(t *testing.T) {
	tmp := t.TempDir()
	if err := writeFile(t, tmp, "agents/test-agent.md", "---\nname: test-agent\ndescription: x\n---\nbody"); err != nil {
		t.Fatal(err)
	}
	l := NewFromDir(tmp)
	p, err := l.Agent("test-agent")
	if err != nil {
		t.Fatalf("Agent: %v", err)
	}
	if p.Name != "test-agent" {
		t.Errorf("Name=%q, want test-agent", p.Name)
	}
}

// TestNewFromDir_EmptyDir — empty path treated as "no source"; all
// loads fail with fs.ErrNotExist.
func TestNewFromDir_EmptyDir(t *testing.T) {
	l := NewFromDir("")
	_, err := l.Agent("anything")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Agent on empty loader: err=%v, want fs.ErrNotExist", err)
	}
}

// Helpers ------------------------------------------------------------

func writeFile(t *testing.T, root, rel, content string) error {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(content), 0o644)
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
