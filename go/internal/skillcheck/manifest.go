package skillcheck

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ManifestProblems validates that .claude-plugin/plugin.json — the registry
// Claude Code's loader actually reads — is a bijection with the skills/ dirs on
// disk: every declared skill is a well-formed ./skills/<name>/ path with a
// backing SKILL.md, every well-formed skill dir on disk is declared, and every
// agents[] path resolves. This is the membership surface the phase-facts,
// command-stub, and codex projections never check: all three source from the
// disk skills/ dir, so a skill dir the manifest omits passes every other gate
// yet is invisible to the loader — a user sees "Unknown skill".
//
// Frontmatter validity (parseable, name == dir) is deliberately NOT re-checked
// here: nameMismatches already owns it and runs alongside this in both Run and
// Check, so duplicating it would double-report one defect. This function's sole
// concern is membership.
//
// Returns one problem per violation (empty slice = healthy). The contract
// mirrors Check: err is non-nil only on an infrastructure fault (an unreadable
// skills dir), so callers fail-open on infra; every logical inconsistency —
// including a present-but-corrupt manifest — is a problem string (fail-closed).
// An ABSENT manifest is "not a plugin tree", so a clean skip.
func ManifestProblems(projectRoot string) ([]string, error) {
	manifestPath := filepath.Join(projectRoot, ".claude-plugin", "plugin.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // not a plugin tree — nothing to validate
		}
		return []string{fmt.Sprintf("MANIFEST: cannot read .claude-plugin/plugin.json: %v", err)}, nil
	}

	var manifest struct {
		Skills []string `json:"skills"`
		Agents []string `json:"agents"`
	}
	if jerr := json.Unmarshal(raw, &manifest); jerr != nil {
		return []string{fmt.Sprintf("MANIFEST: .claude-plugin/plugin.json is not valid JSON: %v", jerr)}, nil
	}

	var problems []string

	// 1. Every declared skill must be a well-formed path with a backing SKILL.md.
	declared := map[string]bool{}
	for _, entry := range manifest.Skills {
		name := skillEntryName(entry)
		if name == "" {
			problems = append(problems, fmt.Sprintf("MANIFEST: skills[] entry %q is not a ./skills/<name>/ path", entry))
			continue
		}
		if declared[name] {
			problems = append(problems, fmt.Sprintf("MANIFEST: skills[] lists %q more than once", name))
		}
		declared[name] = true
		if _, statErr := os.Stat(filepath.Join(projectRoot, "skills", name, "SKILL.md")); statErr != nil {
			problems = append(problems, fmt.Sprintf("MANIFEST: skills[] lists %q but skills/%s/SKILL.md is missing — the install breaks", name, name))
		}
	}

	// 2. Every well-formed skill dir on disk must be declared — an unlisted one
	//    is invisible to the loader ("Unknown skill").
	skillsDir := filepath.Join(projectRoot, "skills")
	entries, derr := os.ReadDir(skillsDir)
	if derr != nil {
		return problems, fmt.Errorf("read skills dir: %w", derr)
	}
	for _, e := range entries {
		if !e.IsDir() || declared[e.Name()] {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(skillsDir, e.Name(), "SKILL.md")); statErr != nil {
			continue // a dir without SKILL.md is not a skill
		}
		problems = append(problems, fmt.Sprintf(
			"MANIFEST: skills/%s/SKILL.md exists on disk but is not listed in .claude-plugin/plugin.json skills[] — Claude Code will not register it (\"Unknown skill\")", e.Name()))
	}

	// 3. Every declared agent path must resolve to a file.
	for _, entry := range manifest.Agents {
		rel := strings.TrimPrefix(entry, "./")
		if rel == "" {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(projectRoot, rel)); statErr != nil {
			problems = append(problems, fmt.Sprintf("MANIFEST: agents[] lists %q but the file is missing", entry))
		}
	}

	sort.Strings(problems)
	return problems, nil
}

// skillEntryName normalizes a plugin.json skills[] entry ("./skills/loop/") to
// its bare skill name ("loop"). Returns "" when the entry is not a single,
// non-traversing skills/<name> segment. Rejecting "." and ".." is what keeps a
// "./skills/.." entry from silently resolving to projectRoot/SKILL.md (a false
// pass); rejecting embedded separators keeps a nested/non-skills entry malformed.
func skillEntryName(entry string) string {
	p := strings.TrimSuffix(strings.TrimPrefix(entry, "./"), "/")
	if !strings.HasPrefix(p, "skills/") {
		return "" // not under skills/ (e.g. "./skills/" itself, or "./agents/x")
	}
	name := strings.TrimPrefix(p, "skills/")
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return ""
	}
	return name
}
