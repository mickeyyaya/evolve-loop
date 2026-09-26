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

// sampleAgent mirrors the frontmatter shapes of the real agents/evolve-scout.md.
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

func fixtureFS() fstest.MapFS {
	return fstest.MapFS{
		"agents/evolve-scout.md":       &fstest.MapFile{Data: []byte(sampleAgent)},
		"agents/evolve-builder.md":     &fstest.MapFile{Data: []byte("---\nname: evolve-builder\ndescription: builder\n---\nBody.")},
		"skills/loop/SKILL.md":         &fstest.MapFile{Data: []byte(sampleSkill)},
		"skills/santa-loop/SKILL.md":   &fstest.MapFile{Data: []byte("---\nname: santa-loop\ndescription: santa\n---\nBody.")},
		"skills/empty/notSkillFile.md": &fstest.MapFile{Data: []byte("ignored")}, // must not appear in Skills()
	}
}

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
	if p.Body == "" {
		t.Error("Body empty")
	}
	if !contains(p.Body, "# Evolve Scout") {
		t.Errorf("Body missing heading: %q", p.Body[:min(60, len(p.Body))])
	}
}

func TestNewFromFS_Skill_PathConvention(t *testing.T) {
	l := NewFromFS(fixtureFS())
	p, err := l.Skill("loop")
	if err != nil {
		t.Fatalf("Skill: %v", err)
	}
	if p.Name != "loop" {
		t.Errorf("Name=%q, want loop", p.Name)
	}
	if !contains(p.Body, "# Evolve Loop") {
		t.Errorf("Body missing skill heading: %q", p.Body[:min(60, len(p.Body))])
	}
}

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

func TestAgents_SortedList(t *testing.T) {
	l := NewFromFS(fixtureFS())
	names, err := l.Agents()
	if err != nil {
		t.Fatalf("Agents: %v", err)
	}
	want := []string{"evolve-builder", "evolve-scout"}
	sort.Strings(want)
	if !reflect.DeepEqual(names, want) {
		t.Errorf("Agents()=%v, want %v", names, want)
	}
}

func TestSkills_OnlySKILL_Md(t *testing.T) {
	l := NewFromFS(fixtureFS())
	names, err := l.Skills()
	if err != nil {
		t.Fatalf("Skills: %v", err)
	}
	want := []string{"loop", "santa-loop"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("Skills()=%v, want %v (sorted, no skills/empty)", names, want)
	}
}

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

func TestParseFrontmatter_UnterminatedBlock(t *testing.T) {
	raw := "---\nname: foo\n(no closing fence)\n"
	_, _, err := ParseFrontmatter(raw)
	if err == nil {
		t.Error("ParseFrontmatter: want error for unterminated frontmatter")
	}
}

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

func TestNewFromDir_EmptyDir(t *testing.T) {
	l := NewFromDir("")
	_, err := l.Agent("anything")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Agent on empty loader: err=%v, want fs.ErrNotExist", err)
	}
}

func TestZeroLoader_AgentReadsErrNotExist(t *testing.T) {
	l := NewFromFS(nil)
	_, err := l.Agent("any")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err=%v, want fs.ErrNotExist on zero loader", err)
	}
}

func TestZeroLoader_AgentsAndSkills(t *testing.T) {
	l := NewFromFS(nil)
	if got, err := l.Agents(); err != nil || got != nil {
		t.Errorf("Agents()=%v,%v; want nil,nil", got, err)
	}
	if got, err := l.Skills(); err != nil || got != nil {
		t.Errorf("Skills()=%v,%v; want nil,nil", got, err)
	}
}

func TestAgents_SkipsDirsAndNonMD(t *testing.T) {
	fsys := fstest.MapFS{
		"agents/foo.md":      &fstest.MapFile{Data: []byte("---\nname: foo\n---\nb")},
		"agents/subdir/x.md": &fstest.MapFile{Data: []byte("ignored")}, // creates subdir entry
		"agents/.DS_Store":   &fstest.MapFile{Data: []byte("junk")},
		"agents/notes.txt":   &fstest.MapFile{Data: []byte("not md")},
	}
	got, err := NewFromFS(fsys).Agents()
	if err != nil {
		t.Fatalf("Agents: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"foo"}) {
		t.Errorf("Agents()=%v, want [foo]", got)
	}
}

func TestSkills_SkipsFileEntries(t *testing.T) {
	fsys := fstest.MapFS{
		"skills/README.md":     &fstest.MapFile{Data: []byte("not a skill")},
		"skills/real/SKILL.md": &fstest.MapFile{Data: []byte("---\nname: real\n---\nb")},
	}
	got, err := NewFromFS(fsys).Skills()
	if err != nil {
		t.Fatalf("Skills: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"real"}) {
		t.Errorf("Skills()=%v, want [real]", got)
	}
}

func TestAgent_ParseErrorSurfacesAtLoad(t *testing.T) {
	fsys := fstest.MapFS{
		"agents/bad.md": &fstest.MapFile{Data: []byte("---\nname: bad\n(missing close fence)\n")},
	}
	_, err := NewFromFS(fsys).Agent("bad")
	if err == nil {
		t.Fatal("want parse error")
	}
	if !contains(err.Error(), "parse") {
		t.Errorf("err=%v missing 'parse' context", err)
	}
}

func TestParseFrontmatter_LineWithoutColonIsSkipped(t *testing.T) {
	raw := "---\nname: foo\nbarewordnocolon\ndescription: bar\n---\nbody"
	fm, _, err := ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if fm["name"] != "foo" || fm["description"] != "bar" {
		t.Errorf("fm=%v, want name=foo description=bar", fm)
	}
	if _, present := fm["barewordnocolon"]; present {
		t.Errorf("bareword line created key: %v", fm)
	}
}

func TestParseFrontmatter_EmptyKey(t *testing.T) {
	raw := "---\nname: foo\n: orphan\n---\nbody"
	fm, _, err := ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if _, present := fm[""]; present {
		t.Errorf("empty key recorded: %v", fm)
	}
}

func TestParseValue_EmptyValue(t *testing.T) {
	raw := "---\nkey:\n---\nbody"
	fm, _, err := ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if fm["key"] != "" {
		t.Errorf("key=%v, want empty string", fm["key"])
	}
}

func TestParseFrontmatter_CRLFLines(t *testing.T) {
	raw := "---\r\nname: foo\r\n---\r\nbody"
	fm, _, err := ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if fm["name"] != "foo" {
		t.Errorf("name=%v, want foo (CRLF tolerated)", fm["name"])
	}
}

func TestSmoke_RealAgentFiles(t *testing.T) {
	root := "../../../" // go/internal/prompts → repo root
	if _, err := os.Stat(filepath.Join(root, "agents")); err != nil {
		t.Skipf("repo agents/ not reachable from test dir: %v", err)
	}
	l := NewFromDir(root)
	agents, err := l.Agents()
	if err != nil {
		t.Fatalf("Agents: %v", err)
	}
	if len(agents) == 0 {
		t.Fatal("expected at least one real agent file")
	}
	// Subagent dispatch keys on the frontmatter `name:`; *-reference.md, AGENTS.md and
	// agent-templates.md are supplementary docs that carry none.
	skipNoFM := func(n string) bool {
		return contains(n, "-reference") || contains(n, "AGENTS") || contains(n, "agent-templates")
	}
	for _, name := range agents {
		p, err := l.Agent(name)
		if err != nil {
			t.Errorf("Agent(%s): %v", name, err)
			continue
		}
		if skipNoFM(name) {
			continue
		}
		if p.Frontmatter == nil {
			t.Errorf("Agent(%s): no frontmatter; expected a `name:` field", name)
		}
	}
}

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

func TestZeroLoader_CarriesBothSentinels(t *testing.T) {
	_, err := NewFromFS(nil).Agent("any")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("zero-loader contract broken: err=%v, want fs.ErrNotExist", err)
	}
	if !errors.Is(err, ErrNoSource) {
		t.Errorf("err=%v, want ErrNoSource discriminator", err)
	}
}
