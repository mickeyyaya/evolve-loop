package reportdoc

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

type SeverityCounts struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
}

func (c SeverityCounts) Blocking() int { return c.Critical + c.High }

const taggedSeverities = severities + `|MAJOR|MINOR`

const MaxSourceLineRunes = 200

const bulletPrefix = `(?:[-*+]|\d+[.)])?\s*`

var foldedSeverity = map[string]string{"MAJOR": "HIGH", "MINOR": "LOW"}

var taggedFinding = regexp.MustCompile(`^\s*` + bulletPrefix + `\**\[(` + taggedSeverities + `)\]`)

var summaryRow = regexp.MustCompile(`^\s*\|\s*\**(` + taggedSeverities + `)\**\s*\|\s*(\d{1,6})\s*\|`)

var countsLine = regexp.MustCompile(`^\s*` + bulletPrefix + `(?:\**[A-Za-z][\w-]{0,30}\**:\**\s*){0,2}\**\d{1,6}\s+(?:` + taggedSeverities + `)\b`)

var countPair = regexp.MustCompile(`\b(\d{1,6})\s+(` + taggedSeverities + `)\b`)

const minCountsLinePairs = 2

type BlockingSource struct {
	CriticalDeclared int
	HighListed       int
	HighDeclared     int
	DeclaredLine     string
}

type severityScan struct {
	listed, declared map[string]int
	declaredLine     map[string]string
}

func fold(severity string) string {
	if folded, ok := foldedSeverity[severity]; ok {
		return folded
	}
	return severity
}

func (s severityScan) declare(severity, digits, line string) {
	n, _ := strconv.Atoi(digits)
	key := fold(severity)
	if n > s.declared[key] {
		s.declared[key], s.declaredLine[key] = n, line
	}
}

func (s severityScan) scanLine(line string) {
	if m := taggedFinding.FindStringSubmatch(line); m != nil {
		s.listed[fold(m[1])]++
	}
	if m := summaryRow.FindStringSubmatch(line); m != nil {
		s.declare(m[1], m[2], line)
	}
	if !countsLine.MatchString(line) {
		return
	}
	if pairs := countPair.FindAllStringSubmatch(line, -1); len(pairs) >= minCountsLinePairs {
		for _, p := range pairs {
			s.declare(p[2], p[1], line)
		}
	}
}

func scanSeverities(markdown string) severityScan {
	s := severityScan{listed: map[string]int{}, declared: map[string]int{}, declaredLine: map[string]string{}}
	for _, f := range Findings(markdown) {
		s.listed[f.Severity]++
	}
	for _, line := range visibleLines(markdown) {
		s.scanLine(line)
	}
	return s
}

func (s severityScan) larger(severity string) int {
	return max(s.listed[severity], s.declared[severity])
}

func CountSeverities(markdown string) SeverityCounts {
	s := scanSeverities(markdown)
	return SeverityCounts{Critical: s.larger("CRITICAL"), High: s.larger("HIGH"), Medium: s.larger("MEDIUM"), Low: s.larger("LOW")}
}

func cleanSourceLine(line string) string {
	printable := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(line))
	if runes := []rune(printable); len(runes) > MaxSourceLineRunes {
		return string(runes[:MaxSourceLineRunes])
	}
	return printable
}

func TraceBlocking(markdown string) BlockingSource {
	s := scanSeverities(markdown)
	line := s.declaredLine["HIGH"]
	if s.declared["CRITICAL"] > 0 {
		line = s.declaredLine["CRITICAL"]
	}
	return BlockingSource{CriticalDeclared: s.declared["CRITICAL"], HighListed: s.listed["HIGH"], HighDeclared: s.declared["HIGH"], DeclaredLine: cleanSourceLine(line)}
}
