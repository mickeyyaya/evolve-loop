package installer

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile is a test helper that writes content under root/rel, creating
// parent dirs.
func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// fakePluginLayout builds a minimal but valid evolve-loop source tree under a
// fresh temp dir: a valid plugin.json, the four core agents (with frontmatter),
// the five required loop skill files, plus one extra agent/skill so the glob
// counts exceed the core minimums.
func fakePluginLayout(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, ".claude-plugin/plugin.json", `{"name":"evolve-loop","version":"6.0.0"}`)
	for _, a := range coreAgents {
		writeFile(t, root, "agents/"+a+".md", "---\nname: "+a+"\n---\nbody\n")
	}
	// An extra evolve-* agent so the glob count is core+1.
	writeFile(t, root, "agents/evolve-retrospective.md", "---\nname: evolve-retrospective\n---\n")
	// A non-evolve agent that must NOT be globbed.
	writeFile(t, root, "agents/other-agent.md", "---\nname: other\n---\n")
	for _, s := range loopSkillFiles {
		writeFile(t, root, "skills/loop/"+s, "# "+s+"\n")
	}
	// An extra skill file so the skill glob is len(loopSkillFiles)+1.
	writeFile(t, root, "skills/loop/extra.md", "# extra\n")
	return root
}

func TestValidate_AcceptsWellFormedLayout(t *testing.T) {
	root := fakePluginLayout(t)
	var out bytes.Buffer
	res := Validate(root, &out)

	if res.Errors != 0 {
		t.Fatalf("Errors = %d, want 0; output:\n%s", res.Errors, out.String())
	}
	if res.Agents != len(coreAgents)+1 {
		t.Errorf("Agents = %d, want %d", res.Agents, len(coreAgents)+1)
	}
	if res.Skills != len(loopSkillFiles)+1 {
		t.Errorf("Skills = %d, want %d", res.Skills, len(loopSkillFiles)+1)
	}
	s := out.String()
	for _, want := range []string{
		"OK: plugin.json exists",
		"OK: plugin.json is valid JSON",
		"OK: agents/evolve-scout.md",
		"OK: skills/loop/SKILL.md",
		"EVOLVE_LOOP_VALIDATED=true",
		"EVOLVE_LOOP_ERRORS=0",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("validate output missing %q\n%s", want, s)
		}
	}
}

func TestValidate_FlagsMissingManifestAndAgents(t *testing.T) {
	root := t.TempDir() // empty: nothing exists
	var out bytes.Buffer
	res := Validate(root, &out)

	// 1 manifest + 4 core agents + 5 skills = 10 hard failures.
	if res.Errors != 1+len(coreAgents)+len(loopSkillFiles) {
		t.Fatalf("Errors = %d, want %d", res.Errors, 1+len(coreAgents)+len(loopSkillFiles))
	}
	s := out.String()
	if !strings.Contains(s, "FAIL: .claude-plugin/plugin.json not found") {
		t.Errorf("expected manifest FAIL, got:\n%s", s)
	}
	if !strings.Contains(s, "FAIL: agents/evolve-builder.md not found") {
		t.Errorf("expected agent FAIL, got:\n%s", s)
	}
}

func TestValidate_FlagsAgentMissingFrontmatter(t *testing.T) {
	root := fakePluginLayout(t)
	// Overwrite one core agent without the leading --- fence.
	writeFile(t, root, "agents/evolve-auditor.md", "no frontmatter here\n")
	var out bytes.Buffer
	res := Validate(root, &out)

	if res.Errors != 1 {
		t.Fatalf("Errors = %d, want 1; output:\n%s", res.Errors, out.String())
	}
	if !strings.Contains(out.String(), "FAIL: agents/evolve-auditor.md missing YAML frontmatter") {
		t.Errorf("expected frontmatter FAIL, got:\n%s", out.String())
	}
}

func TestValidate_FlagsInvalidJSONAsWarn(t *testing.T) {
	root := fakePluginLayout(t)
	writeFile(t, root, ".claude-plugin/plugin.json", "{not valid json")
	var out bytes.Buffer
	res := Validate(root, &out)

	// Invalid JSON is a WARN (matches bash `|| echo "WARN..."`), not a hard error.
	if res.Errors != 0 {
		t.Fatalf("Errors = %d, want 0 (bad JSON is WARN-only)", res.Errors)
	}
	s := out.String()
	if !strings.Contains(s, "OK: plugin.json exists") {
		t.Errorf("expected manifest existence OK, got:\n%s", s)
	}
	if !strings.Contains(s, "WARN: could not validate JSON") {
		t.Errorf("expected JSON WARN, got:\n%s", s)
	}
}

func TestValidateJSONFile_AcceptReject(t *testing.T) {
	root := t.TempDir()
	good := filepath.Join(root, "good.json")
	if err := os.WriteFile(good, []byte(`{"a":1,"b":[2,3]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateJSONFile(good); err != nil {
		t.Errorf("valid JSON rejected: %v", err)
	}

	bad := filepath.Join(root, "bad.json")
	if err := os.WriteFile(bad, []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateJSONFile(bad); err == nil {
		t.Error("invalid JSON accepted, want error")
	}

	if err := ValidateJSONFile(filepath.Join(root, "absent.json")); err == nil {
		t.Error("missing file accepted, want read error")
	}
}

func TestInstall_CopiesAgentsAndSkills(t *testing.T) {
	src := fakePluginLayout(t)
	home := t.TempDir()
	var out bytes.Buffer

	res, err := Install(src, home, &out)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Agents != len(coreAgents)+1 {
		t.Errorf("copied Agents = %d, want %d", res.Agents, len(coreAgents)+1)
	}
	if res.Skills != len(loopSkillFiles)+1 {
		t.Errorf("copied Skills = %d, want %d", res.Skills, len(loopSkillFiles)+1)
	}

	// Same filesystem effects the bash produced: agents land in .claude/agents,
	// skills land in .claude/skills/loop, content preserved, non-evolve agent
	// NOT copied.
	scout := filepath.Join(home, ".claude", "agents", "evolve-scout.md")
	if b, err := os.ReadFile(scout); err != nil {
		t.Fatalf("evolve-scout.md not installed: %v", err)
	} else if !strings.Contains(string(b), "name: evolve-scout") {
		t.Errorf("evolve-scout.md content not preserved: %q", b)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "agents", "other-agent.md")); !os.IsNotExist(err) {
		t.Error("non-evolve agent was copied; install must glob only evolve-*.md")
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "loop", "SKILL.md")); err != nil {
		t.Errorf("SKILL.md not installed: %v", err)
	}

	if !strings.Contains(out.String(), "Installing:") {
		t.Errorf("expected first-install log line, got:\n%s", out.String())
	}
}

func TestInstall_OverwriteLogsOverwriting(t *testing.T) {
	src := fakePluginLayout(t)
	home := t.TempDir()
	// Pre-place a stale evolve-scout.md so the second install path is exercised.
	writeFile(t, home, ".claude/agents/evolve-scout.md", "stale\n")

	var out bytes.Buffer
	if _, err := Install(src, home, &out); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !strings.Contains(out.String(), "Overwriting: evolve-scout.md") {
		t.Errorf("expected Overwriting log for pre-existing agent, got:\n%s", out.String())
	}
	// Content must be the source's, not the stale one.
	b, _ := os.ReadFile(filepath.Join(home, ".claude", "agents", "evolve-scout.md"))
	if strings.Contains(string(b), "stale") {
		t.Error("overwrite did not replace stale content")
	}
}

func TestUninstall_RemovesAgentsAndSkillDir(t *testing.T) {
	src := fakePluginLayout(t)
	home := t.TempDir()
	if _, err := Install(src, home, &bytes.Buffer{}); err != nil {
		t.Fatalf("setup Install: %v", err)
	}
	// A user agent that is NOT evolve-* must survive uninstall.
	writeFile(t, home, ".claude/agents/my-agent.md", "keep me\n")

	var out bytes.Buffer
	res, err := Uninstall(home, &out)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if res.AgentsRemoved != len(coreAgents)+1 {
		t.Errorf("AgentsRemoved = %d, want %d", res.AgentsRemoved, len(coreAgents)+1)
	}
	if !res.SkillDirExisted {
		t.Error("SkillDirExisted = false, want true")
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "loop")); !os.IsNotExist(err) {
		t.Error("skills/loop dir was not removed")
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "agents", "evolve-scout.md")); !os.IsNotExist(err) {
		t.Error("evolve-scout.md was not removed")
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "agents", "my-agent.md")); err != nil {
		t.Error("non-evolve user agent was wrongly removed")
	}
	if !strings.Contains(out.String(), "Removing: evolve-scout.md") {
		t.Errorf("expected Removing log, got:\n%s", out.String())
	}
}

func TestUninstall_EmptyHomeIsNoOp(t *testing.T) {
	home := t.TempDir()
	var out bytes.Buffer
	res, err := Uninstall(home, &out)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if res.AgentsRemoved != 0 || res.SkillDirExisted {
		t.Errorf("empty home not a no-op: %+v", res)
	}
	s := out.String()
	if !strings.Contains(s, "No agents found") || !strings.Contains(s, "No skill found") {
		t.Errorf("expected not-found messages, got:\n%s", s)
	}
}

func TestUninstallDryRun_ListsButDeletesNothing(t *testing.T) {
	src := fakePluginLayout(t)
	home := t.TempDir()
	if _, err := Install(src, home, &bytes.Buffer{}); err != nil {
		t.Fatalf("setup Install: %v", err)
	}

	var out bytes.Buffer
	res := UninstallDryRun(home, &out)
	if res.AgentsRemoved != len(coreAgents)+1 {
		t.Errorf("would-remove count = %d, want %d", res.AgentsRemoved, len(coreAgents)+1)
	}
	if !res.SkillDirExisted {
		t.Error("SkillDirExisted = false, want true")
	}
	// Nothing actually deleted.
	if _, err := os.Stat(filepath.Join(home, ".claude", "agents", "evolve-scout.md")); err != nil {
		t.Error("dry-run deleted an agent")
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "loop", "SKILL.md")); err != nil {
		t.Error("dry-run deleted the skill dir")
	}
	s := out.String()
	for _, want := range []string{
		"Uninstall dry-run (CI mode)",
		"Would remove: evolve-scout.md",
		"EVOLVE_LOOP_UNINSTALL_VALIDATED=true",
		"EVOLVE_LOOP_SKILL_DIR_EXISTS=true",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("dry-run output missing %q\n%s", want, s)
		}
	}
}

func TestPluginAlreadyInstalled(t *testing.T) {
	home := t.TempDir()
	if PluginAlreadyInstalled(home) {
		t.Error("clean home reported as already-installed")
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude", "plugins", "cache", "evolve-loop"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !PluginAlreadyInstalled(home) {
		t.Error("plugin cache dir present but not detected")
	}

	home2 := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home2, ".claude", "plugins", "marketplaces", "evolve-loop"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !PluginAlreadyInstalled(home2) {
		t.Error("marketplace dir present but not detected")
	}
}

// TestExportedResultTypes names every exported result type via composite
// literals so the apicover gate sees them referenced in-package.
func TestExportedResultTypes(t *testing.T) {
	if (ValidateResult{Agents: 1, Skills: 2, Errors: 0}).Agents != 1 {
		t.Fatal("ValidateResult field access broken")
	}
	if (InstallResult{Agents: 3, Skills: 4}).Skills != 4 {
		t.Fatal("InstallResult field access broken")
	}
	if (UninstallResult{AgentsRemoved: 5, SkillDirExisted: true}).AgentsRemoved != 5 {
		t.Fatal("UninstallResult field access broken")
	}
	if Version == "" || UsageLine == "" {
		t.Fatal("Version/UsageLine consts must be non-empty")
	}
	if !strings.Contains(UsageLine, "[cycles] [strategy] [goal]") {
		t.Errorf("UsageLine drifted from the 3-arg form: %q", UsageLine)
	}
}
