package explanationdocs

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

func changedAreaFailures(section string, changed, material []string) []string {
	entries, failures := changedAreaEntries(section)
	for _, materialPath := range material {
		if entries[materialPath] == "" {
			failures = append(failures, fmt.Sprintf("Explanation Documentation: Changed Areas does not explain material path %s", materialPath))
		}
	}
	cited := make([]string, 0, len(entries))
	for citedPath := range entries {
		cited = append(cited, citedPath)
	}
	sort.Strings(cited)
	normalized := make([]string, 0, len(changed))
	for _, changedPath := range changed {
		normalized = append(normalized, normalize(changedPath))
	}
	for _, citedPath := range cited {
		if !coversAnyChangedPath(citedPath, normalized) {
			failures = append(failures, fmt.Sprintf("Explanation Documentation: cited path %s is not in the Build diff", citedPath))
		}
	}
	return failures
}

const maxCitationPatterns = 64

func coversAnyChangedPath(cited string, changed []string) bool {
	for _, pattern := range append([]string{cited}, expandBraces(cited)...) {
		for _, changedPath := range changed {
			if coversPath(pattern, changedPath) {
				return true
			}
		}
	}
	return false
}

func coversPath(pattern, changedPath string) bool {
	dir := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(pattern, "/..."), "/**"), "/")
	if changedPath == pattern || strings.HasPrefix(changedPath, dir+"/") {
		return true
	}
	matched, err := path.Match(pattern, changedPath)
	return err == nil && matched
}

func expandBraces(s string) []string {
	patterns := []string{s}
	for i := 0; i < len(patterns); {
		expanded, ok, grouped := expandFirstGroup(patterns[i])
		switch {
		case !ok:
			return nil
		case !grouped:
			i++
			continue
		}
		patterns = append(append(patterns[:i:i], expanded...), patterns[i+1:]...)
		if len(patterns) > maxCitationPatterns {
			return nil
		}
	}
	return patterns
}

func expandFirstGroup(s string) (expanded []string, ok, grouped bool) {
	open := strings.Index(s, "{")
	if open < 0 {
		return nil, true, false
	}
	end := strings.Index(s[open:], "}")
	if end < 0 {
		return nil, false, true
	}
	end += open
	body := s[open+1 : end]
	if strings.Contains(body, "{") {
		return nil, false, true
	}
	for _, alt := range strings.Split(body, ",") {
		if alt == "" {
			return nil, false, true
		}
		expanded = append(expanded, s[:open]+alt+s[end+1:])
	}
	return expanded, true, true
}

func changedAreaEntries(body string) (map[string]string, []string) {
	entries := map[string]string{}
	var failures []string
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "- `") {
			continue
		}
		rest := strings.TrimPrefix(line, "- `")
		end := strings.Index(rest, "`")
		if end < 0 {
			continue
		}
		path := normalize(rest[:end])
		explanation := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(rest[end+1:]), "—-:"))
		if !validRelative(path) {
			failures = append(failures, "Explanation Documentation: Changed Areas contains an invalid repo-relative path")
			continue
		}
		if len(explanation) < 10 {
			failures = append(failures, fmt.Sprintf("Explanation Documentation: Changed Areas path %s needs a what/why explanation", path))
			continue
		}
		entries[path] = explanation
	}
	return entries, failures
}
