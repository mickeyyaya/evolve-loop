// Package skilloverlay materializes policy-resolved skill overlays into a
// prompt prefix. The policy layer (internal/policy) decides WHICH skills a
// dispatch requires (Policy.ResolveOverlays, keyed on phase/cli/model/tier);
// this package reads those skills' SKILL.md persona bodies from disk and
// formats them as one prepend block, so a phase agent on ANY CLI begins its
// turn with the configured operating discipline preloaded. Pure + deterministic
// (regular code, no LLM) — the "which skill for which phase agent" decision is
// configuration (policy.json overlays), the materialization is here.
package skilloverlay

import (
	"os"
	"path/filepath"
	"strings"
)

// Materialize reads each named skill's SKILL.md from skillsDir/<name>/SKILL.md
// and returns a single prompt-prefix block concatenating their persona bodies
// (YAML frontmatter stripped) in the given order — the policy resolver's stable
// order is authoritative. A name that is unsafe (contains a path separator or a
// traversal segment), whose SKILL.md is missing/unreadable, or whose body is
// empty is skipped and reported in `missing`, so the caller WARNs loudly rather
// than silently dropping a configured overlay. An empty names slice returns
// ("", nil) — a phase with no overlay is byte-identical to pre-feature behavior.
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
		raw, err := os.ReadFile(filepath.Join(skillsDir, name, "SKILL.md"))
		if err != nil {
			missing = append(missing, name)
			continue
		}
		body := strings.TrimSpace(stripFrontmatter(string(raw)))
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

// safeName reports whether name is a single, non-traversal registry entry safe
// to join under skillsDir. On the wired overlay path (Policy.ResolveOverlays)
// skill names come straight from the compiled defaults / policy.json overlays
// with no registry clamp, so this is the ONLY guard preventing a crafted name
// from reading a SKILL.md outside skillsDir — it must stand on its own.
func safeName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\`)
}

// stripFrontmatter removes a leading YAML frontmatter block ("---" line …
// "---" line) if present and returns the persona body. Markdown horizontal
// rules ("---") in the body are untouched because only the FIRST closing
// delimiter after the opening is consumed. Content without leading frontmatter
// passes through unchanged.
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
	return s // no closing delimiter → treat as body, return unchanged
}
