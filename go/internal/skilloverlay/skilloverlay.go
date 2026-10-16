// Package skilloverlay materializes policy-resolved skill overlays into a
// prompt prefix.
package skilloverlay

import (
	"os"
	"path/filepath"
	"strings"
)

func Materialize(skillsDir string, names []string) (prefix string, missing []string) {
	if len(names) == 0 {
		return "", nil
	}
	var b strings.Builder
	for _, name := range names {
		if !safeName(name) {
			missing = append(missing, name)
			continue
		}
		body := readSkillBody(skillsDir, name)
		if body == "" {
			missing = append(missing, name)
			continue
		}
		b.WriteString("=== PRELOADED SKILL: " + name + " (operating-discipline overlay) ===\n")
		b.WriteString(body)
		b.WriteString("\n=== END SKILL: " + name + " ===\n\n")
	}
	return b.String(), missing
}

func readSkillBody(skillsDir, name string) string {
	for _, file := range []string{"COMPACT.md", "SKILL.md"} {
		raw, err := os.ReadFile(filepath.Join(skillsDir, name, file))
		if err != nil {
			continue
		}
		if body := strings.TrimSpace(stripFrontmatter(string(raw))); body != "" {
			return body
		}
	}
	return ""
}

func safeName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\`)
}

func stripFrontmatter(s string) string {
	lines := strings.Split(s, "\n")
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) || strings.TrimSpace(lines[i]) != "---" {
		return s
	}
	for j := i + 1; j < len(lines); j++ {
		if strings.TrimSpace(lines[j]) == "---" {
			return strings.TrimLeft(strings.Join(lines[j+1:], "\n"), "\r\n")
		}
	}
	return s
}
