// Package installer is the native-Go port of install.sh / uninstall.sh — the
// manual "copy evolve-loop agents + the loop skill into ~/.claude/" path plus
// the CI-mode structure validator. It is the deliverable behind the `evolve
// install` / `evolve uninstall` subcommands (Wave D of the bash→Go migration).
//
// Two surfaces, matching the bash exactly:
//
//   - Validate (CI mode): asserts the plugin manifest, the four core evolve-*
//     agents, and the five loop skill files exist, that each agent file opens
//     with a `---` YAML frontmatter fence, and that the manifest is valid JSON.
//     The JSON check folds the legacy `python3 -c "json.load(...)"` snippet into
//     encoding/json — no python dependency. CI mode copies nothing.
//   - Install / Uninstall: copies evolve-*.md agents and skills/loop/*.md into
//     $HOME/.claude (install) or removes them (uninstall). Reproduces the bash
//     "Overwriting:" vs "Installing:" / "Removing:" log lines and the
//     already-installed-as-plugin duplication warning.
//
// All filesystem mutation goes through atomicwrite so a partial copy never
// leaves a half-written agent file in ~/.claude.
package installer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

// coreAgents are the four agent files the CI validator requires by exact name
// (install.sh lines 41-53). The install/uninstall copy/remove steps instead
// glob agents/evolve-*.md, which is a superset of these.
var coreAgents = []string{
	"evolve-scout",
	"evolve-builder",
	"evolve-auditor",
	"evolve-operator",
}

// loopSkillFiles are the five skill files the CI validator requires under
// skills/loop/ (install.sh lines 56-63).
var loopSkillFiles = []string{
	"SKILL.md",
	"phases.md",
	"memory-protocol.md",
	"eval-runner.md",
	"online-researcher.md",
}

// Version is the human-facing release line the manual installer prints. It
// mirrors the "Installing Evolve Loop v6..." banner the bash emitted; the
// update-install-sh-version-string eval asserts on this exact substring.
const Version = "v6"

// UsageLine is the one-line /evolve-loop usage string the installer prints on
// success. It mirrors install.sh line 146; the fix-install-usage-and-ci-docs-check
// eval asserts it is the three-argument [cycles] [strategy] [goal] form.
const UsageLine = "Usage: /evolve-loop [cycles] [strategy] [goal]"

// ValidateResult is the machine-readable summary CI mode prints (the
// EVOLVE_LOOP_* key=value lines install.sh emitted). A non-zero Errors means the
// validator exits 1.
type ValidateResult struct {
	// Agents is the number of agents/evolve-*.md files found (the glob count,
	// matching the bash `ls agents/evolve-*.md | wc -l`).
	Agents int
	// Skills is the number of skills/loop/*.md files found.
	Skills int
	// Errors is the count of hard validation failures.
	Errors int
}

// Validate runs CI mode: it inspects the plugin layout rooted at srcDir,
// writing OK/FAIL/WARN lines to out, and returns the summary. It mutates
// nothing on disk. This is the Go form of `install.sh --ci`.
func Validate(srcDir string, out io.Writer) ValidateResult {
	res := ValidateResult{}

	// Plugin manifest: existence + valid JSON (the python3 json.load fold).
	manifest := filepath.Join(srcDir, ".claude-plugin", "plugin.json")
	if !fileExists(manifest) {
		fmt.Fprintln(out, "FAIL: .claude-plugin/plugin.json not found")
		res.Errors++
	} else {
		fmt.Fprintln(out, "OK: plugin.json exists")
		if ValidateJSONFile(manifest) == nil {
			fmt.Fprintln(out, "OK: plugin.json is valid JSON")
		} else {
			fmt.Fprintln(out, "WARN: could not validate JSON")
		}
	}

	// Core agents: existence + YAML frontmatter fence.
	for _, agent := range coreAgents {
		path := filepath.Join(srcDir, "agents", agent+".md")
		if !fileExists(path) {
			fmt.Fprintf(out, "FAIL: agents/%s.md not found\n", agent)
			res.Errors++
			continue
		}
		if !hasFrontmatter(path) {
			fmt.Fprintf(out, "FAIL: agents/%s.md missing YAML frontmatter\n", agent)
			res.Errors++
			continue
		}
		fmt.Fprintf(out, "OK: agents/%s.md\n", agent)
	}

	// Loop skill files: existence only.
	for _, skill := range loopSkillFiles {
		path := filepath.Join(srcDir, "skills", "loop", skill)
		if !fileExists(path) {
			fmt.Fprintf(out, "FAIL: skills/loop/%s not found\n", skill)
			res.Errors++
			continue
		}
		fmt.Fprintf(out, "OK: skills/loop/%s\n", skill)
	}

	res.Agents = len(globAgentsIn(filepath.Join(srcDir, "agents")))
	res.Skills = len(globSkills(srcDir))

	fmt.Fprintln(out, "EVOLVE_LOOP_VALIDATED=true")
	fmt.Fprintf(out, "EVOLVE_LOOP_AGENTS=%d\n", res.Agents)
	fmt.Fprintf(out, "EVOLVE_LOOP_SKILLS=%d\n", res.Skills)
	fmt.Fprintf(out, "EVOLVE_LOOP_ERRORS=%d\n", res.Errors)
	return res
}

// ValidateJSONFile reports whether path holds syntactically valid JSON. It is
// the Go replacement for the `python3 -c "json.load(open(...))"` snippet in
// install.sh: read the file and unmarshal into an empty interface. A read or
// parse failure is returned as an error so callers can map it to the WARN line.
func ValidateJSONFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

// InstallResult is the outcome of a manual install: the agent and skill file
// counts copied into the home tree (mirrors the bash AGENT_COUNT / SKILL_COUNT
// success summary).
type InstallResult struct {
	// Agents is the number of evolve-*.md agent files copied.
	Agents int
	// Skills is the number of skills/loop/*.md files copied.
	Skills int
}

// Install copies evolve-*.md agents and skills/loop/*.md from srcDir into
// homeDir/.claude (agents → .claude/agents, skills → .claude/skills/loop),
// logging "Installing:"/"Overwriting:" per file to out. It is the Go form of
// the manual-install branch of install.sh. The plugin-already-installed
// duplication warning is the caller's responsibility (see PluginAlreadyInstalled):
// the bash prompts interactively, and the Go command resolves that at the cmd
// layer.
func Install(srcDir, homeDir string, out io.Writer) (InstallResult, error) {
	agentsDst := filepath.Join(homeDir, ".claude", "agents")
	skillsDst := filepath.Join(homeDir, ".claude", "skills", "loop")
	if err := os.MkdirAll(agentsDst, 0o755); err != nil {
		return InstallResult{}, fmt.Errorf("mkdir agents dir: %w", err)
	}
	if err := os.MkdirAll(skillsDst, 0o755); err != nil {
		return InstallResult{}, fmt.Errorf("mkdir skills dir: %w", err)
	}

	res := InstallResult{}

	fmt.Fprintf(out, "Copying agents to %s/\n", agentsDst)
	for _, src := range globAgentsIn(filepath.Join(srcDir, "agents")) {
		name := filepath.Base(src)
		logCopy(out, filepath.Join(agentsDst, name), name)
		if err := copyFile(src, filepath.Join(agentsDst, name)); err != nil {
			return res, err
		}
		res.Agents++
	}

	fmt.Fprintf(out, "\nCopying skill to %s/\n", skillsDst)
	for _, src := range globSkills(srcDir) {
		name := filepath.Base(src)
		logCopy(out, filepath.Join(skillsDst, name), name)
		if err := copyFile(src, filepath.Join(skillsDst, name)); err != nil {
			return res, err
		}
		res.Skills++
	}
	return res, nil
}

// UninstallResult is the outcome of a manual uninstall: the agent files removed
// and whether the loop skill directory existed (and was removed).
type UninstallResult struct {
	// AgentsRemoved is the number of evolve-*.md agent files deleted.
	AgentsRemoved int
	// SkillDirExisted reports whether the skills/loop directory was present
	// (and therefore removed).
	SkillDirExisted bool
}

// Uninstall removes evolve-*.md agents from homeDir/.claude/agents and the
// whole skills/loop directory, logging "Removing:" per agent to out. It is the
// Go form of the deletion branch of uninstall.sh. It never touches the
// project's .evolve/ workspace (the bash prints the same caveat).
func Uninstall(homeDir string, out io.Writer) (UninstallResult, error) {
	agentsDst := filepath.Join(homeDir, ".claude", "agents")
	skillsDst := filepath.Join(homeDir, ".claude", "skills", "loop")

	res := UninstallResult{}
	agents := globAgentsIn(agentsDst)
	if len(agents) > 0 {
		fmt.Fprintf(out, "Removing agents from %s/\n", agentsDst)
		for _, a := range agents {
			fmt.Fprintf(out, "  Removing: %s\n", filepath.Base(a))
			if err := os.Remove(a); err != nil {
				return res, fmt.Errorf("remove %s: %w", a, err)
			}
			res.AgentsRemoved++
		}
	} else {
		fmt.Fprintf(out, "No agents found in %s/\n", agentsDst)
	}

	if dirExists(skillsDst) {
		fmt.Fprintf(out, "\nRemoving skill from %s/\n", skillsDst)
		if err := os.RemoveAll(skillsDst); err != nil {
			return res, fmt.Errorf("remove skill dir: %w", err)
		}
		res.SkillDirExisted = true
	} else {
		fmt.Fprintf(out, "No skill found at %s/\n", skillsDst)
	}
	return res, nil
}

// UninstallDryRun is the Go form of `uninstall.sh --ci`: it lists what would be
// removed from homeDir without deleting anything, printing the bash dry-run
// lines and the EVOLVE_LOOP_UNINSTALL_* summary to out, and returns the same
// counts Uninstall would.
func UninstallDryRun(homeDir string, out io.Writer) UninstallResult {
	agentsDst := filepath.Join(homeDir, ".claude", "agents")
	skillsDst := filepath.Join(homeDir, ".claude", "skills", "loop")

	fmt.Fprintln(out, "Uninstall dry-run (CI mode) — validating targets only")
	fmt.Fprintln(out, "")

	res := UninstallResult{}
	agents := globAgentsIn(agentsDst)
	if len(agents) > 0 {
		for _, a := range agents {
			fmt.Fprintf(out, "  Would remove: %s\n", filepath.Base(a))
			res.AgentsRemoved++
		}
	} else {
		fmt.Fprintf(out, "  No agents found in %s/\n", agentsDst)
	}

	if dirExists(skillsDst) {
		fmt.Fprintf(out, "  Would remove: %s/\n", skillsDst)
		res.SkillDirExisted = true
	} else {
		fmt.Fprintf(out, "  No skill found at %s/\n", skillsDst)
	}

	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "EVOLVE_LOOP_UNINSTALL_VALIDATED=true")
	fmt.Fprintf(out, "EVOLVE_LOOP_AGENTS_TO_REMOVE=%d\n", res.AgentsRemoved)
	fmt.Fprintf(out, "EVOLVE_LOOP_SKILL_DIR_EXISTS=%t\n", res.SkillDirExisted)
	return res
}

// PluginAlreadyInstalled reports whether evolve-loop is already present as an AI
// CLI plugin under homeDir (a cache or marketplace dir). The manual installer
// uses this to warn about creating duplicate /evolve-loop entries, exactly as
// install.sh lines 84-99 did.
func PluginAlreadyInstalled(homeDir string) bool {
	cache := filepath.Join(homeDir, ".claude", "plugins", "cache", "evolve-loop")
	marketplace := filepath.Join(homeDir, ".claude", "plugins", "marketplaces", "evolve-loop")
	return dirExists(cache) || dirExists(marketplace)
}

// --- unexported helpers -----------------------------------------------------

func logCopy(out io.Writer, dst, name string) {
	if fileExists(dst) {
		fmt.Fprintf(out, "  Overwriting: %s\n", name)
	} else {
		fmt.Fprintf(out, "  Installing:  %s\n", name)
	}
}

func globAgentsIn(dir string) []string {
	matches, _ := filepath.Glob(filepath.Join(dir, "evolve-*.md"))
	sort.Strings(matches)
	return matches
}

func globSkills(srcDir string) []string {
	matches, _ := filepath.Glob(filepath.Join(srcDir, "skills", "loop", "*.md"))
	sort.Strings(matches)
	return matches
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	if err := atomicwrite.Bytes(dst, data); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func hasFrontmatter(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	first := string(data)
	if i := strings.IndexByte(first, '\n'); i >= 0 {
		first = first[:i]
	}
	return strings.TrimRight(first, "\r") == "---"
}
