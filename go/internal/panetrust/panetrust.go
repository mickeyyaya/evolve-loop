// Package panetrust is the single trust boundary for text derived from a tmux pane.
// See docs/architecture/packages/internal-panetrust.md.
package panetrust

import (
	"regexp"
	"strings"
)

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]|\x1b\\][^\x07]*\x07")

const defangTag = "[untrusted]"

var markerBreaks = [...][2]string{
	{"evolve-verdict:", "evolve-verdict" + defangTag + ":"},
	{`"evolve_channel"`, `"evolve_channel` + defangTag + `"`},
}

const redactedToken = "[REDACTED]"

var secretREs = []*regexp.Regexp{
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{10,}`),
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
	regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs)_[A-Za-z0-9]{20,}\b`),
	regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}\b`),
	regexp.MustCompile(`\bxox[a-z]-[A-Za-z0-9-]{10,}`),
	regexp.MustCompile(`\bxapp-[A-Za-z0-9-]{10,}`),
	regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]*`),
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
}

var kvSecretRE = regexp.MustCompile(`(?i)\b(access[_-]?token|refresh[_-]?token|id[_-]?token|client[_-]?secret|private[_-]?key|api[_-]?key|credential|secret|token|password|passwd|authorization)(\s*[:=]\s*)\S+`)

func redactSecrets(s string) string {
	for _, re := range secretREs {
		s = re.ReplaceAllString(s, redactedToken)
	}
	return kvSecretRE.ReplaceAllString(s, "${1}${2}"+redactedToken)
}

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
