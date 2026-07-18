// Package secretleakscan is the native, in-process Go implementation of the
// secret-leak-scan phase. It replaces the former full-LLM boot (tmux CLI,
// ~12 turns, thousands of output tokens) with a deterministic regex scan over a
// unified git diff — Rule 5: deterministic work belongs in code, not LLM cycles.
//
// The phase is pure judgment-free detection: it inspects the ADDED lines of a
// unified diff for well-known credential shapes (private-key headers, cloud
// access-key ids, provider tokens) and emits the canonical PASS/FAIL verdict
// vocabulary the LLM variant produced, so downstream gates that pattern-match
// the deliverable artifact keep working unchanged.
package secretleakscan

import "regexp"

// Finding is a single detected secret. Rule names the detector that fired and
// Match is the offending substring (already narrowed to the credential token,
// not the whole line) so a report can cite it without dumping surrounding code.
type Finding struct {
	Rule  string
	Match string
}

// rule pairs a detector name with its compiled pattern. The set is intentionally
// conservative — each pattern targets a structurally unambiguous credential
// shape so a clean diff never trips a false positive (which would make the
// native phase noisier than the LLM one it replaces).
type rule struct {
	name string
	re   *regexp.Regexp
}

// rules is the ordered detector set. Order is stable so findings are emitted
// deterministically for identical input.
var rules = []rule{
	{"pem-private-key", regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY-----`)},
	{"aws-access-key-id", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{"github-token", regexp.MustCompile(`\bgh[pousr]_[0-9A-Za-z]{36}\b`)},
	{"slack-token", regexp.MustCompile(`\bxox[baprs]-[0-9A-Za-z-]{10,}\b`)},
	{"generic-private-key-assign", regexp.MustCompile(`(?i)(?:private_?key|secret_?key|api_?key)\s*[:=]\s*["'][0-9A-Za-z/+_-]{24,}["']`)},
}

// ScanDiff scans the ADDED lines of a unified git diff (lines prefixed with a
// single '+', excluding the '+++' file header) and returns one Finding per
// detector match. Removed and context lines are ignored — only newly introduced
// content can leak a secret. The result is deterministic for identical input.
func ScanDiff(diff string) []Finding {
	var findings []Finding
	for _, line := range splitLines(diff) {
		if !isAddedLine(line) {
			continue
		}
		added := line[1:] // strip the leading '+'
		for _, r := range rules {
			if m := r.re.FindString(added); m != "" {
				findings = append(findings, Finding{Rule: r.name, Match: m})
			}
		}
	}
	return findings
}

// Verdict maps a finding set to the canonical phase verdict vocabulary:
// "PASS" when no secrets were found, "FAIL" when at least one was.
func Verdict(findings []Finding) string {
	if len(findings) == 0 {
		return "PASS"
	}
	return "FAIL"
}

// isAddedLine reports whether a unified-diff line is an added content line
// (single leading '+', not the '+++' target-file header).
func isAddedLine(line string) bool {
	return len(line) > 0 && line[0] == '+' && !hasPrefix(line, "+++")
}

// splitLines splits on '\n' without allocating on the common no-CR case; a
// trailing '\r' is trimmed so CRLF diffs scan identically to LF diffs.
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

// trimCR drops a single trailing carriage return.
func trimCR(s string) string {
	if n := len(s); n > 0 && s[n-1] == '\r' {
		return s[:n-1]
	}
	return s
}

// hasPrefix is a dependency-free strings.HasPrefix to keep this leaf import-light.
func hasPrefix(s, p string) bool {
	return len(s) >= len(p) && s[:len(p)] == p
}
