package recovery

import "strings"

func StripAgentContent(pane, injectedPrompt string, protected []string) string {
	echo := strings.TrimSpace(injectedPrompt) != ""
	lines := strings.Split(pane, "\n")
	for i, ln := range lines {
		if isAgentDiffLine(ln) {
			lines[i] = ""
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

func isAgentDiffLine(ln string) bool {
	rest := strings.TrimLeft(ln, " \t")
	if strings.HasPrefix(rest, "+++") || strings.HasPrefix(rest, "---") {
		return false
	}
	if digits := len(rest) - len(strings.TrimLeft(rest, "0123456789")); digits > 0 {
		if after := rest[digits:]; strings.TrimLeft(after, " \t") != after {
			rest = strings.TrimLeft(after, " \t")
		}
	}
	return strings.HasPrefix(rest, "+") || strings.HasPrefix(rest, "-")
}

func carriesProtectedSignature(line string, protected []string) bool {
	for _, sig := range protected {
		if s := strings.TrimSpace(sig); s != "" && strings.Contains(line, s) {
			return true
		}
	}
	return false
}
