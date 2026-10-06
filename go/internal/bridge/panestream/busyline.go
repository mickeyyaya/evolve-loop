package panestream

import (
	"regexp"
	"strings"
	"sync"
)

var compiledPatterns sync.Map

func compiledPattern(pattern string) *regexp.Regexp {
	if pattern == "" {
		return nil
	}
	if cached, ok := compiledPatterns.Load(pattern); ok {
		return cached.(*regexp.Regexp)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		re = nil
	}
	compiledPatterns.Store(pattern, re)
	return re
}

func withoutSpinnerFrame(line string, busyLine *regexp.Regexp) (string, bool) {
	if busyLine == nil {
		return line, false
	}
	loc := busyLine.FindStringSubmatchIndex(line)
	if loc == nil {
		return line, false
	}
	if len(loc) >= 4 && loc[2] >= 0 {
		return line[:loc[2]] + line[loc[3]:], true
	}
	return line, true
}

func normalizedLines(rendered string, p PaneProfile) []string {
	busyLine := compiledPattern(p.BusyLineRegex)
	lines := strings.Split(stripANSI(rendered), "\n")
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i], _ = withoutSpinnerFrame(line, busyLine)
	}
	return out
}

func hasBusyLine(lines []string, p PaneProfile) bool {
	busyLine := compiledPattern(p.BusyLineRegex)
	if busyLine == nil {
		return false
	}
	for _, line := range lines {
		if busyLine.MatchString(line) {
			return true
		}
	}
	return false
}
