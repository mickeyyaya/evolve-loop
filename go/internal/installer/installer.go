// Package installer installs and removes the evolve-loop agents and loop skill under ~/.claude
// and validates the plugin layout. See docs/architecture/packages/internal-installer.md.
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

var coreAgents = []string{
	"evolve-scout",
	"evolve-builder",
	"evolve-auditor",
	"evolve-operator",
}

var loopSkillFiles = []string{
	"SKILL.md",
	"phases.md",
	"memory-protocol.md",
	"eval-runner.md",
	"online-researcher.md",
}

var pluginRequiredFields = []string{"name", "version", "description", "agents", "skills"}

var referenceDocs = []string{
	"docs/reference/genes.md",
	"docs/reference/instincts.md",
	"docs/reference/configuration.md",
}

const Version = "v6"

const UsageLine = "Usage: /evo:loop [cycles] [strategy] [goal]"

type ValidateResult struct {
	Agents int
	Skills int
	Errors int
}

func Validate(srcDir string, out io.Writer) ValidateResult {
	res := ValidateResult{}

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

	validateMarketplace(filepath.Join(srcDir, ".claude-plugin", "marketplace.json"), &res, out)

	validateAgentFrontmatter(srcDir, &res, out)

	validateSkillFiles(srcDir, &res, out)
	validateReferenceDocs(srcDir, &res, out)

	res.Agents = len(globAgentsIn(filepath.Join(srcDir, "agents")))
	res.Skills = len(globSkills(srcDir))

	fmt.Fprintln(out, "EVOLVE_LOOP_VALIDATED=true")
	fmt.Fprintf(out, "EVOLVE_LOOP_AGENTS=%d\n", res.Agents)
	fmt.Fprintf(out, "EVOLVE_LOOP_SKILLS=%d\n", res.Skills)
	fmt.Fprintf(out, "EVOLVE_LOOP_ERRORS=%d\n", res.Errors)
	return res
}

func validateSkillFiles(srcDir string, res *ValidateResult, out io.Writer) {
	for _, skill := range loopSkillFiles {
		if !fileExists(filepath.Join(srcDir, "skills", "loop", skill)) {
			fmt.Fprintf(out, "FAIL: skills/loop/%s not found\n", skill)
			res.Errors++
			continue
		}
		fmt.Fprintf(out, "OK: skills/loop/%s\n", skill)
	}
}

func validateReferenceDocs(srcDir string, res *ValidateResult, out io.Writer) {
	for _, doc := range referenceDocs {
		if !fileExists(filepath.Join(srcDir, doc)) {
			fmt.Fprintf(out, "FAIL: %s not found\n", doc)
			res.Errors++
			continue
		}
		fmt.Fprintf(out, "OK: %s\n", doc)
	}
}

func validateManifestFields(manifest string, res *ValidateResult, out io.Writer) {
	raw, err := os.ReadFile(manifest)
	if err != nil {
		return
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return
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

type InstallResult struct {
	Agents int
	Skills int
}

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

type UninstallResult struct {
	AgentsRemoved   int
	SkillDirExisted bool
}

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

func PluginAlreadyInstalled(homeDir string) bool {
	cache := filepath.Join(homeDir, ".claude", "plugins", "cache", "evo")
	marketplace := filepath.Join(homeDir, ".claude", "plugins", "marketplaces", "evo")
	return dirExists(cache) || dirExists(marketplace)
}

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

func isJSONArray(raw json.RawMessage) bool {
	var arr []json.RawMessage
	return json.Unmarshal(raw, &arr) == nil
}

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
			break
		}
		if i == 0 {
			continue
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
