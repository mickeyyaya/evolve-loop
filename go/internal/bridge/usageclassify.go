package bridge

// usageclassify.go — the manifest-backed cap classifier the usage probe injects
// as its Classify seam. Given a family + a captured /usage|/status pane, it
// reports whether the pane shows the family is CURRENTLY out of quota. It is
// deliberately conservative: only an unambiguous exhaustion phrase benches, so a
// healthy report (which also mentions reset times and remaining percentages) is
// never a false positive. The pattern is config-driven — the family's
// controls.usage.exhausted_regex, falling back to the manifest's maintained
// rate_limit interactive-prompt regex.

import "regexp"

// ClassifyExhausted reports whether pane (the captured usage/status output for
// family) indicates the family is currently capped. Fail-open: an unloadable
// manifest or an empty/uncompilable pattern returns false (the probe must never
// invent a cap).
func ClassifyExhausted(family, pane string) bool {
	m, err := LoadManifest(family + "-tmux")
	if err != nil {
		return false
	}
	return matchExhausted(manifestExhaustedPattern(m), pane)
}

// manifestExhaustedPattern resolves the exhaustion regex for m: the usage
// control's exhausted_regex when set, else the rate_limit interactive-prompt
// regex (the single maintained source for "what a wall looks like"), else "".
func manifestExhaustedPattern(m Manifest) string {
	if spec, ok := m.Control("usage"); ok && spec.ExhaustedRegex != "" {
		return spec.ExhaustedRegex
	}
	for _, p := range m.InteractivePrompts {
		if p.Name == "rate_limit" {
			return p.Regex
		}
	}
	return ""
}

// matchExhausted compiles pattern and tests it against pane. An empty or
// invalid pattern matches nothing (fail-open).
func matchExhausted(pattern, pane string) bool {
	if pattern == "" {
		return false
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(pane)
}
