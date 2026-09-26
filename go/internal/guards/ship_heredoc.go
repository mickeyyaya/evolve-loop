package guards

import (
	"regexp"
	"strings"
)

// stripHeredocs drops heredoc body lines from a shell command and keeps the opener and closing marker lines.
// An unterminated heredoc drops the rest of the input, as bash reads it to EOF.
func stripHeredocs(cmd string) string {
	if !strings.Contains(cmd, "<<") {
		return cmd
	}
	lines := strings.Split(cmd, "\n")
	out := make([]string, 0, len(lines))
	inHeredoc := false
	marker := ""

	for _, line := range lines {
		if inHeredoc {
			stripped := strings.TrimLeft(line, " \t")
			if stripped == marker {
				inHeredoc = false
				out = append(out, line)
			}
			continue
		}
		if m := heredocStartRE.FindStringSubmatch(line); m != nil {
			marker = stripHeredocMarker(m[1])
			inHeredoc = true
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

var heredocStartRE = regexp.MustCompile(`<<-?[ \t]*(['"]?[A-Za-z_][A-Za-z0-9_]*['"]?)`)

// stripHeredocMarker unquotes a captured marker: quoting changes expansion, not the terminator line.
func stripHeredocMarker(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
