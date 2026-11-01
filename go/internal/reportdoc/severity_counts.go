package reportdoc

import (
	"regexp"
	"strconv"
)

type SeverityCounts struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
}

func (c SeverityCounts) Blocking() int { return c.Critical + c.High }

const taggedSeverities = severities + `|MAJOR|MINOR`

var foldedSeverity = map[string]string{"MAJOR": "HIGH", "MINOR": "LOW"}

var taggedFinding = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])?\s*\**\[(` + taggedSeverities + `)\]`)

var summaryRow = regexp.MustCompile(`^\s*\|\s*\**(` + taggedSeverities + `)\**\s*\|\s*(\d{1,6})\s*\|`)

var countPair = regexp.MustCompile(`\b(\d{1,6})\s+(` + taggedSeverities + `)\b`)

const minCountsLinePairs = 2

func fold(severity string) string {
	if folded, ok := foldedSeverity[severity]; ok {
		return folded
	}
	return severity
}

func CountSeverities(markdown string) SeverityCounts {
	listed, declared := map[string]int{}, map[string]int{}
	for _, f := range Findings(markdown) {
		listed[f.Severity]++
	}
	declare := func(severity, digits string) {
		n, _ := strconv.Atoi(digits)
		declared[fold(severity)] = max(declared[fold(severity)], n)
	}
	for _, line := range visibleLines(markdown) {
		if m := taggedFinding.FindStringSubmatch(line); m != nil {
			listed[fold(m[1])]++
		}
		if m := summaryRow.FindStringSubmatch(line); m != nil {
			declare(m[1], m[2])
		}
		if pairs := countPair.FindAllStringSubmatch(line, -1); len(pairs) >= minCountsLinePairs {
			for _, p := range pairs {
				declare(p[2], p[1])
			}
		}
	}
	larger := func(severity string) int { return max(listed[severity], declared[severity]) }
	return SeverityCounts{Critical: larger("CRITICAL"), High: larger("HIGH"), Medium: larger("MEDIUM"), Low: larger("LOW")}
}
