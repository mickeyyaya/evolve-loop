package guards

import (
	"regexp"
	"strings"
)

// stripHeredocs removes heredoc body content from a multi-line shell command
// string. This mirrors the awk pre-processor in
// legacy/scripts/guards/ship-gate.sh that was added to prevent commit-message
// bodies (which legitimately contain literal "git push" / "git commit"
// describing what a script does) from tripping the ship-verb regex.
//
// Handles bash heredoc syntax:
//
//	cat <<EOF        — unquoted
//	cat <<'EOF'      — single-quoted (no var expansion in bash; same here)
//	cat <<"EOF"      — double-quoted
//	cat <<-EOF       — tab-stripping form
//
// The marker is any identifier matching [A-Za-z_][A-Za-z0-9_]*. Body content
// between `<<MARKER` (or `<<-MARKER` / `<<'MARKER'` / `<<"MARKER"`) and the
// matching marker on its own line (with optional leading whitespace) is
// dropped from the returned string. The marker lines themselves are
// preserved so byte offsets stay roughly aligned for downstream regex
// matchers that want to see the heredoc opener / closer.
//
// Multiple sequential heredocs in one command are handled. Unterminated
// heredocs leave the rest of the input dropped (matches bash's behavior of
// continuing the heredoc to EOF).
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
				out = append(out, line) // keep the closing marker line
			}
			// else: drop the body line (don't append)
			continue
		}
		// Detect heredoc start on this line.
		if m := heredocStartRE.FindStringSubmatch(line); m != nil {
			// m[1] is the marker (after stripping -, optional quotes).
			marker = stripHeredocMarker(m[1])
			inHeredoc = true
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// heredocStartRE matches `<<MARKER` / `<<-MARKER` / `<<'MARKER'` / `<<"MARKER"`
// with optional whitespace before the marker.
var heredocStartRE = regexp.MustCompile(`<<-?[ \t]*(['"]?[A-Za-z_][A-Za-z0-9_]*['"]?)`)

// stripHeredocMarker removes surrounding ' or " quotes from a captured
// heredoc marker. Bash treats `<<'EOF'` and `<<EOF` identically as
// terminator-matching goes; both end at a line whose only content is `EOF`.
func stripHeredocMarker(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
