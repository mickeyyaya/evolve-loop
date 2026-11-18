// Package panetrust is the single trust boundary for text derived from a tmux pane.
// See docs/architecture/packages/internal-panetrust.md.
package panetrust

import (
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/redact"
)

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]|\x1b\\][^\x07]*\x07")

const defangTag = "[untrusted]"

var markerBreaks = [...][2]string{
	{"evolve-verdict:", "evolve-verdict" + defangTag + ":"},
	{`"evolve_channel"`, `"evolve_channel` + defangTag + `"`},
}

func redactSecrets(s string) string { return redact.Secrets(s) }

func RedactSecrets(s string) string { return redactSecrets(s) }

func Digest(pane string, maxLines, maxCols int) string {
	if pane == "" || maxLines <= 0 {
		return ""
	}
	s := ansiRE.ReplaceAllString(pane, "")
	s = redactSecrets(s)
	for _, mb := range markerBreaks {
		s = strings.ReplaceAll(s, mb[0], mb[1])
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	if maxCols > 0 {
		for i, ln := range lines {
			lines[i] = truncateRunes(ln, maxCols)
		}
	}
	return strings.Join(lines, "\n")
}

func truncateRunes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
