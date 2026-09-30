package explanationdocs

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

func changedAreaFailures(section string, changed, material []string) []string {
	normalized := make([]string, 0, len(changed))
	for _, changedPath := range changed {
		normalized = append(normalized, normalize(changedPath))
	}
	entries, failures := changedAreaEntries(section, normalized)
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

const minExplanationBytes = 10

func changedAreaEntries(body string, changed []string) (map[string]string, []string) {
	entries := map[string]string{}
	var failures []string
	for _, item := range changedAreaItems(body, changed) {
		paths, explanation := splitChangedAreaItem(item, changed)
		for _, cited := range paths {
			path := normalize(cited)
			if !validRelative(path) {
				failures = append(failures, "Explanation Documentation: Changed Areas contains an invalid repo-relative path")
				continue
			}
			if len(explanation) < minExplanationBytes {
				failures = append(failures, fmt.Sprintf("Explanation Documentation: Changed Areas path %s needs a what/why explanation", path))
				continue
			}
			entries[path] = explanation
		}
	}
	return entries, failures
}

func changedAreaItems(body string, changed []string) []string {
	var items []string
	var lines []string
	indent, blank := 0, false
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		depth := len(raw) - len(strings.TrimLeft(raw, " \t"))
		nested := lines != nil && depth > indent
		switch {
		case strings.HasPrefix(line, "- `") && (!nested || citesNestedPath(firstSpan(line), changed)):
			items = appendItem(items, lines)
			lines, indent, blank = []string{line}, depth, false
		case line == "":
			blank = true
		case lines == nil:
		case nested || !blank && !strings.HasPrefix(line, "- "):
			lines = append(lines, line)
		default:
			items, lines = appendItem(items, lines), nil
		}
	}
	return appendItem(items, lines)
}

func firstSpan(line string) string {
	span, _, _ := leadingCodeSpan(strings.TrimPrefix(line, "- "))
	return span
}

func citesNestedPath(span string, changed []string) bool {
	return strings.Contains(span, "/") || namesBuildContent(span, changed)
}

func namesBuildContent(span string, changed []string) bool {
	return coversAnyChangedPath(normalize(span), changed)
}

func appendItem(items, lines []string) []string {
	if lines == nil {
		return items
	}
	return append(items, strings.Join(lines, " "))
}

func splitChangedAreaItem(item string, changed []string) ([]string, string) {
	first, rest, ok := leadingCodeSpan(strings.TrimPrefix(item, "- "))
	if !ok {
		return nil, ""
	}
	paths := []string{first}
	for {
		span, after, ok := leadingCodeSpan(afterGroupJoiner(rest))
		if !ok || !namesBuildContent(span, changed) {
			break
		}
		paths, rest = append(paths, span), after
	}
	return paths, explanationText(rest)
}

func leadingCodeSpan(s string) (span, rest string, ok bool) {
	if !strings.HasPrefix(s, "`") {
		return "", s, false
	}
	end := strings.Index(s[1:], "`")
	if end < 0 {
		return "", s, false
	}
	return s[1 : end+1], s[end+2:], true
}

func afterGroupJoiner(s string) string {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), ","))
	s = strings.TrimPrefix(s, "and ")
	s = strings.TrimPrefix(s, "& ")
	return strings.TrimSpace(s)
}

func explanationText(s string) string {
	return strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(s), "—-:"))
}
