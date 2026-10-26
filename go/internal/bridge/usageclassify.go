package bridge

import "regexp"

func ClassifyExhausted(family, pane string) bool {
	r, err := Resolve(DefaultDriverSet(), family)
	if err != nil {
		return false
	}
	return matchExhausted(manifestExhaustedPattern(r.Manifest), pane)
}

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

func classifyTransientPane(driver, pane string) bool {
	m, err := LoadManifest(driver)
	if err != nil {
		return false
	}
	return matchExhausted(m.TransientRegex, pane)
}
