package recovery

// strip.go imports only "strings": go/acs/cycle1123 compiles overlay mutants of
// StripAgentContent, and an import only that function uses would break them.
import "strings"

// StripAgentContent blanks agent diff lines and prompt echoes, sparing echoes that carry a protected signature.
func StripAgentContent(pane, injectedPrompt string, protected []string) string {
	echo := strings.TrimSpace(injectedPrompt) != ""
	lines := strings.Split(pane, "\n")
	for i, ln := range lines {
		if isAgentDiffLine(ln) {
			lines[i] = "" // blank, never delete: newline-anchored seeds need the "\n"
			continue
		}
		if !echo {
			continue
		}
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" || !strings.Contains(injectedPrompt, trimmed) {
			continue
		}
		if !carriesProtectedSignature(ln, protected) {
			lines[i] = ""
		}
	}
	return strings.Join(lines, "\n")
}

// isAgentDiffLine matches an optional line number then "+" or "-"; "+++" and "---" headers are not content.
// Hand-rolled rather than regexp so "strings" stays this file's only import.
func isAgentDiffLine(ln string) bool {
	rest := strings.TrimLeft(ln, " \t")
	if strings.HasPrefix(rest, "+++") || strings.HasPrefix(rest, "---") {
		return false
	}
	// A line number counts only when whitespace follows: "72+" is prose arithmetic.
	if digits := len(rest) - len(strings.TrimLeft(rest, "0123456789")); digits > 0 {
		if after := rest[digits:]; strings.TrimLeft(after, " \t") != after {
			rest = strings.TrimLeft(after, " \t")
		}
	}
	return strings.HasPrefix(rest, "+") || strings.HasPrefix(rest, "-")
}

// carriesProtectedSignature trims entries because anchored seeds carry a leading "\n" that a
// split line lacks, and skips blank entries because every line contains them.
func carriesProtectedSignature(line string, protected []string) bool {
	for _, sig := range protected {
		if s := strings.TrimSpace(sig); s != "" && strings.Contains(line, s) {
			return true
		}
	}
	return false
}
