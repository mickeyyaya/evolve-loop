// Package installer is the native-Go port of install.sh / uninstall.sh — the
// manual "copy evolve-loop agents + the loop skill into ~/.claude/" path plus
// the CI-mode structure validator. It is the deliverable behind the `evolve
// install` / `evolve uninstall` subcommands (Wave D of the bash→Go migration).
//
// Two surfaces, matching the bash exactly:
//
//   - Validate (CI mode): the single replacement for the whole legacy CI
//     workflow (Wave E). It folds every check the six ci.yml validation steps
//     ran — the install.sh structure check, the two python3 plugin/marketplace
//     JSON validators, and the three inline-bash file-existence loops (agent
//     frontmatter, loop skill files, reference docs) — into one pass with no
//     python or bash dependency. It asserts:
//     plugin.json exists, is valid JSON, and carries every required field
//     (name/version/description/agents/skills) with agents+skills as JSON
//     arrays; marketplace.json exists, is valid JSON, and has a non-empty
//     plugins array; every agents/evolve-*.md (EXCEPT *-reference.md) opens
//     with a `---` fence and a frontmatter name: + description:; the required
//     loop skill files exist; and the reference docs exist. Any hard failure
//     increments Errors so the command exits 1. CI mode copies nothing.
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

	"github.com/mickeyyaya/evolveloop/go/internal/atomicwrite"
)

// coreAgents are the four canonical evolve agents. The CI validator now globs
// every agents/evolve-*.md for frontmatter (so it no longer pins these by exact
// name), and install/uninstall glob the same set; coreAgents remains the
// minimal fixture set the tests build a fake layout from.
var coreAgents = []string{
	"evolve-scout",
	"evolve-builder",
	"evolve-auditor",
	"evolve-operator",
}

// loopSkillFiles are the skill files the CI validator requires under
// skills/loop/. install.sh and ci.yml step 5 require the first four; the
// installer additionally requires online-researcher.md (a strict superset of
// ci.yml step 5, so it subsumes it).
var loopSkillFiles = []string{
	"SKILL.md",
	"phases.md",
	"memory-protocol.md",
	"eval-runner.md",
	"online-researcher.md",
}

// pluginRequiredFields are the top-level keys ci.yml step 2 (the python3
// plugin.json validator) required to be present. agents and skills must
// additionally be JSON arrays (checked in validateManifestFields).
var pluginRequiredFields = []string{"name", "version", "description", "agents", "skills"}

// referenceDocs are the docs/reference/*.md files ci.yml step 6 required to
// exist (paths post the v8.48.0 docs/ reorg; island-model.md was removed in the
// skill-publishing cleanup, so it is not required).
var referenceDocs = []string{
	"docs/reference/genes.md",
	"docs/reference/instincts.md",
	"docs/reference/configuration.md",
}

// Version is the human-facing release line the manual installer prints. It
// mirrors the "Installing Evolve Loop v6..." banner the bash emitted; the
// update-install-sh-version-string eval asserts on this exact substring.
const Version = "v6"

// UsageLine is the one-line /evo:loop usage string the installer prints on
// success. It mirrors install.sh line 146; the fix-install-usage-and-ci-docs-check
// eval asserts it is the three-argument [cycles] [strategy] [goal] form.
const UsageLine = "Usage: /evo:loop [cycles] [strategy] [goal]"

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
// nothing on disk. This single pass is the Go replacement for all six legacy
// ci.yml validation steps (Wave E of the bash→Go migration).
func Validate(srcDir string, out io.Writer) ValidateResult {
	res := ValidateResult{}

	// Plugin manifest: existence, valid JSON, and required fields/array types.
	// (ci.yml step 1 existence + step 2's python3 plugin.json validator.)
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
		validateManifestFields(manifest, &res, out)
	}

	// Marketplace manifest: existence, valid JSON, non-empty plugins array.
	// (ci.yml step 3's python3 marketplace.json validator.)
	validateMarketplace(filepath.Join(srcDir, ".claude-plugin", "marketplace.json"), &res, out)

	// Agent frontmatter: EVERY agents/evolve-*.md (except *-reference.md) must
	// open with a `---` fence AND carry name: + description: in the frontmatter.
	// (ci.yml step 4 — the bash loop over agents/evolve-*.md.)
	validateAgentFrontmatter(srcDir, &res, out)

	// Loop skill files: existence only. (ci.yml step 5.)
	for _, skill := range loopSkillFiles {
		path := filepath.Join(srcDir, "skills", "loop", skill)
		if !fileExists(path) {
			fmt.Fprintf(out, "FAIL: skills/loop/%s not found\n", skill)
			res.Errors++
			continue
		}
		fmt.Fprintf(out, "OK: skills/loop/%s\n", skill)
	}

	// Reference docs: existence only. (ci.yml step 6.)
	for _, doc := range referenceDocs {
		if !fileExists(filepath.Join(srcDir, doc)) {
			fmt.Fprintf(out, "FAIL: %s not found\n", doc)
			res.Errors++
			continue
		}
		fmt.Fprintf(out, "OK: %s\n", doc)
	}

	res.Agents = len(globAgentsIn(filepath.Join(srcDir, "agents")))
	res.Skills = len(globSkills(srcDir))

	fmt.Fprintln(out, "EVOLVE_LOOP_VALIDATED=true")
	fmt.Fprintf(out, "EVOLVE_LOOP_AGENTS=%d\n", res.Agents)
	fmt.Fprintf(out, "EVOLVE_LOOP_SKILLS=%d\n", res.Skills)
	fmt.Fprintf(out, "EVOLVE_LOOP_ERRORS=%d\n", res.Errors)
	return res
}

// validateManifestFields asserts plugin.json carries every required top-level
// field and that agents+skills are JSON arrays — the Go fold of ci.yml step 2's
// python3 validator. A missing field or wrong-typed agents/skills is a hard
// failure (res.Errors++). A manifest that does not parse is left to the
// ValidateJSONFile WARN above (this returns early without piling on).
func validateManifestFields(manifest string, res *ValidateResult, out io.Writer) {
	raw, err := os.ReadFile(manifest)
	if err != nil {
		return
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return // already reported as a JSON WARN.
	}
	errs := 0
	for _, field := range pluginRequiredFields {
		if _, ok := m[field]; !ok {
			fmt.Fprintf(out, "FAIL: plugin.json missing field: %s\n", field)
			res.Errors++
			errs++
		}
	}
	for _, field := range []string{"agents", "skills"} {
		if rawField, ok := m[field]; ok && !isJSONArray(rawField) {
			fmt.Fprintf(out, "FAIL: plugin.json field %q must be an array\n", field)
			res.Errors++
			errs++
		}
	}
	if errs == 0 {
		fmt.Fprintln(out, "OK: plugin.json has all required fields")
	}
}

// validateMarketplace asserts marketplace.json exists, parses, and has a
// non-empty plugins array — the Go fold of ci.yml step 3's python3 validator.
func validateMarketplace(path string, res *ValidateResult, out io.Writer) {
	if !fileExists(path) {
		fmt.Fprintln(out, "FAIL: .claude-plugin/marketplace.json not found")
		res.Errors++
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(out, "FAIL: marketplace.json unreadable: %v\n", err)
		res.Errors++
		return
	}
	var m struct {
		Plugins []json.RawMessage `json:"plugins"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		fmt.Fprintf(out, "FAIL: marketplace.json invalid JSON: %v\n", err)
		res.Errors++
		return
	}
	if len(m.Plugins) == 0 {
		fmt.Fprintln(out, "FAIL: marketplace.json has no plugins")
		res.Errors++
		return
	}
	fmt.Fprintf(out, "OK: marketplace.json has %d plugin(s)\n", len(m.Plugins))
}

// validateAgentFrontmatter checks EVERY agents/evolve-*.md (excluding the
// *-reference.md Layer-3 docs, which intentionally have no frontmatter) for a
// `---` fence and name: + description: keys — the Go fold of ci.yml step 4's
// bash loop. Each offending file is a hard failure.
func validateAgentFrontmatter(srcDir string, res *ValidateResult, out io.Writer) {
	for _, path := range globAgentsIn(filepath.Join(srcDir, "agents")) {
		if strings.HasSuffix(path, "-reference.md") {
			continue
		}
		rel := filepath.Join("agents", filepath.Base(path))
		if !hasFrontmatter(path) {
			fmt.Fprintf(out, "FAIL: %s missing YAML frontmatter\n", rel)
			res.Errors++
			continue
		}
		missing := frontmatterMissingKeys(path, "name", "description")
		if len(missing) > 0 {
			for _, key := range missing {
				fmt.Fprintf(out, "FAIL: %s frontmatter missing %q field\n", rel, key)
				res.Errors++
			}
			continue
		}
		fmt.Fprintf(out, "OK: %s\n", rel)
	}
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
// uses this to warn about creating duplicate /evo:loop entries, exactly as
// install.sh lines 84-99 did.
func PluginAlreadyInstalled(homeDir string) bool {
	cache := filepath.Join(homeDir, ".claude", "plugins", "cache", "evo")
	marketplace := filepath.Join(homeDir, ".claude", "plugins", "marketplaces", "evo")
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

// isJSONArray reports whether raw is a JSON array value. It is the Go form of
// the python `isinstance(p['agents'], list)` checks in ci.yml step 2.
func isJSONArray(raw json.RawMessage) bool {
	var arr []json.RawMessage
	return json.Unmarshal(raw, &arr) == nil
}

// frontmatterMissingKeys returns which of wantKeys are NOT present as `key:`
// lines inside the file's leading `---`…`---` YAML frontmatter block. It mirrors
// the bash `sed -n '2,/^---$/p' | grep -q "^name:"` checks in ci.yml step 4:
// a key matches a line whose first non-blank token is "<key>:". The opening
// fence is assumed (callers gate on hasFrontmatter first); scanning stops at the
// closing fence so body text can never satisfy a key.
func frontmatterMissingKeys(path string, wantKeys ...string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return append([]string(nil), wantKeys...)
	}
	found := make(map[string]bool, len(wantKeys))
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		if i > 0 && trimmed == "---" {
			break // closing fence — stop before the body.
		}
		if i == 0 {
			continue // opening fence.
		}
		for _, key := range wantKeys {
			if strings.HasPrefix(trimmed, key+":") {
				found[key] = true
			}
		}
	}
	var missing []string
	for _, key := range wantKeys {
		if !found[key] {
			missing = append(missing, key)
		}
	}
	return missing
}
