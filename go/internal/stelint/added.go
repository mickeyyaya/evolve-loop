package stelint

import (
	"regexp"
	"strings"
)

var addedListItem = regexp.MustCompile(`^\s*([-*+]|\d{1,9}[.)])\s+`)

func KeepAdded(raw []byte, exactLines bool, added map[int]bool, findings []Finding) []Finding {
	lines := strings.Split(string(raw), "\n")
	kept := make([]Finding, 0, len(findings))
	for _, f := range findings {
		from, to := f.Line, f.Line
		if !exactLines {
			from, to = blockSpan(lines, f.Line)
		}
		if f.IsSkipNotice() || f.Line < 1 || addedWithin(added, from, to) {
			kept = append(kept, f)
		}
	}
	return kept
}

func addedWithin(added map[int]bool, from, to int) bool {
	for n := from; n <= to; n++ {
		if added[n] {
			return true
		}
	}
	return false
}

func blockSpan(lines []string, line int) (int, int) {
	from, to := line, line
	if line < 1 || line > len(lines) || blockEdge(lines[line-1]) {
		return from, to
	}
	for from > 1 && !addedListItem.MatchString(lines[from-1]) && !blockEdge(lines[from-2]) {
		from--
	}
	for to < len(lines) && !addedListItem.MatchString(lines[to]) && !blockEdge(lines[to]) {
		to++
	}
	return from, to
}

func blockEdge(line string) bool {
	t := strings.TrimSpace(line)
	return t == "" || strings.HasPrefix(t, "|") || strings.HasPrefix(t, "#")
}
