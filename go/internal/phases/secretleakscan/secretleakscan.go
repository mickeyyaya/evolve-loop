// Package secretleakscan detects credential shapes in the added lines of a unified git diff.
// Its PASS/FAIL verdict matches the vocabulary of the LLM phase it replaces.
package secretleakscan

import (
	"regexp"
	"strconv"
	"strings"
)

type Finding struct {
	Rule  string
	Match string
	File  string
	Line  int
}

type rule struct {
	name string
	re   *regexp.Regexp
}

var rules = []rule{
	{"pem-private-key", regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY-----`)},
	{"aws-access-key-id", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{"github-token", regexp.MustCompile(`\bgh[pousr]_[0-9A-Za-z]{36}\b`)},
	{"slack-token", regexp.MustCompile(`\bxox[baprs]-[0-9A-Za-z-]{10,}\b`)},
	{"generic-private-key-assign", regexp.MustCompile(`(?i)(?:private_?key|secret_?key|api_?key)\s*[:=]\s*["'][0-9A-Za-z/+_-]{24,}["']`)},
}

func ScanDiff(diff string) []Finding {
	var findings []Finding
	var loc location
	for _, line := range splitLines(diff) {
		if !isAddedLine(line) {
			loc = loc.advance(line)
			continue
		}
		content := line[1:]
		for _, r := range rules {
			if m := r.re.FindString(content); m != "" {
				findings = append(findings, Finding{Rule: r.name, Match: m, File: loc.file, Line: loc.next})
			}
		}
		loc = loc.step()
	}
	return findings
}

type location struct {
	file string
	next int
}

func (l location) advance(line string) location {
	switch {
	case hasPrefix(line, "+++"):
		return location{file: headerPath(line)}
	case hasPrefix(line, "@@"):
		return location{file: l.file, next: hunkNewStart(line)}
	case hasPrefix(line, " "):
		return l.step()
	}
	return l
}

func (l location) step() location {
	if l.next > 0 {
		l.next++
	}
	return l
}

func headerPath(line string) string {
	path, _, _ := strings.Cut(strings.TrimPrefix(line, "+++ "), "\t")
	if len(path) >= 2 && path[0] == '"' && path[len(path)-1] == '"' {
		path = path[1 : len(path)-1]
	}
	return strings.TrimPrefix(path, "b/")
}

func hunkNewStart(line string) int {
	fields := strings.Fields(line)
	if len(fields) < 3 || !hasPrefix(fields[2], "+") {
		return 0
	}
	start, _, _ := strings.Cut(fields[2][1:], ",")
	n, err := strconv.Atoi(start)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func Verdict(findings []Finding) string {
	if len(findings) == 0 {
		return "PASS"
	}
	return "FAIL"
}

func isAddedLine(line string) bool {
	return len(line) > 0 && line[0] == '+' && !hasPrefix(line, "+++")
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, trimCR(s[start:i]))
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, trimCR(s[start:]))
	}
	return out
}

func trimCR(s string) string {
	if n := len(s); n > 0 && s[n-1] == '\r' {
		return s[:n-1]
	}
	return s
}

func hasPrefix(s, p string) bool {
	return len(s) >= len(p) && s[:len(p)] == p
}
