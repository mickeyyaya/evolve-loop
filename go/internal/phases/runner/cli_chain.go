package runner

import (
	"strings"
)

// DefaultDiscoverCLIsFn and DefaultUniversalFallback are the universal fallback's boot-time defaults, set once and
// read-only after, so the per-phase constructors need not thread discovery; Options fields override them.
var (
	DefaultDiscoverCLIsFn    func() []string
	DefaultUniversalFallback bool
)

func sameCandidates(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func joinAttempts(attempts []string) string {
	if len(attempts) == 0 {
		return ""
	}
	out := attempts[0]
	for _, a := range attempts[1:] {
		out += " -> " + a
	}
	return out
}

// FormatSkillOverlayLog renders an attempt's skill-overlay line; an empty set renders as `skill-overlays=[]`,
// so "no overlay resolved" differs from "the line never ran".
func FormatSkillOverlayLog(phase string, skills []string, tier string) string {
	return "[runner] phase=" + phase +
		" skill-overlays=[" + strings.Join(skills, ",") + "]" +
		" (tier=" + tier + ")"
}
