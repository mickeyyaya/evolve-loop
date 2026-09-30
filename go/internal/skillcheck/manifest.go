package skillcheck

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func ManifestProblems(projectRoot string) ([]string, error) {
	manifestPath := filepath.Join(projectRoot, ".claude-plugin", "plugin.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
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

	problems, declared := declaredSkillProblems(projectRoot, manifest.Skills)

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
			continue
		}
		problems = append(problems, fmt.Sprintf(
			"MANIFEST: skills/%s/SKILL.md exists on disk but is not listed in .claude-plugin/plugin.json skills[] — Claude Code will not register it (\"Unknown skill\")", e.Name()))
	}

	problems = append(problems, agentProblems(projectRoot, manifest.Agents)...)

	sort.Strings(problems)
	return problems, nil
}

func declaredSkillProblems(projectRoot string, skills []string) ([]string, map[string]bool) {
	var problems []string
	declared := map[string]bool{}
	for _, entry := range skills {
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
	return problems, declared
}

func agentProblems(projectRoot string, agents []string) []string {
	var problems []string
	for _, entry := range agents {
		rel := strings.TrimPrefix(entry, "./")
		if rel == "" {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(projectRoot, rel)); statErr != nil {
			problems = append(problems, fmt.Sprintf("MANIFEST: agents[] lists %q but the file is missing", entry))
		}
	}
	return problems
}

func skillEntryName(entry string) string {
	p := strings.TrimSuffix(strings.TrimPrefix(entry, "./"), "/")
	if !strings.HasPrefix(p, "skills/") {
		return ""
	}
	name := strings.TrimPrefix(p, "skills/")
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return ""
	}
	return name
}
