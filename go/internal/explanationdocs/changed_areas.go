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
	for _, citedPath := range cited {
		if !coversAnyChangedPath(citedPath, changed) {
			failures = append(failures, fmt.Sprintf("Explanation Documentation: cited path %s is not in the Build diff", citedPath))
		}
	}
	return failures
}

func coversAnyChangedPath(cited string, changed []string) bool {
	for _, pattern := range expandBraces(cited) {
		for _, changedPath := range changed {
			if coversPath(pattern, normalize(changedPath)) {
				return true
			}
		}
	}
	return false
}

func coversPath(pattern, changedPath string) bool {
	if changedPath == pattern || strings.HasPrefix(changedPath, strings.TrimSuffix(pattern, "/")+"/") {
		return true
	}
	matched, err := path.Match(pattern, changedPath)
	return err == nil && matched
}

func expandBraces(s string) []string {
	open, end := strings.Index(s, "{"), strings.Index(s, "}")
	if open < 0 || end < open {
		return []string{s}
	}
	var out []string
	for _, alt := range strings.Split(s[open+1:end], ",") {
		out = append(out, expandBraces(s[:open]+alt+s[end+1:])...)
	}
	return out
}
