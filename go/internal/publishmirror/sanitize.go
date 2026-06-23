// Package publishmirror builds and (optionally) pushes the public open-source
// mirror (github.com/mickeyyaya/evolveloop) from the private source-of-truth
// tree, applying the residual publish transform and a deterministic PII/secret
// sanitizer gate. See docs/operations/public-release.md for the process this
// automates.
package publishmirror

import (
	"regexp"
	"sort"
	"strings"
)

// Violation is a single deterministic sanitizer finding: a line in the staged
// public tree that matched a structural PII/secret rule or an operator-supplied
// denylist term. A non-empty []Violation is a hard publish stop.
type Violation struct {
	File  string // path relative to the staged tree root
	Line  int    // 1-based line number
	Rule  string // which rule fired ("macos-home-path", "denylist", a secret rule, ...)
	Match string // the offending substring (a denylist term is reported as the term)
}

type structRule struct {
	name string
	re   *regexp.Regexp
}

// structuralRules are format-anchored patterns that should never reach the
// public mirror after convergence. They are intentionally high-confidence
// (specific key formats, an absolute macOS home path) to avoid false positives
// on documentation — the canonical scrubbed forms (~, user@example.com,
// user@host) match none of them.
var structuralRules = []structRule{
	// A real macOS home path leaks the operator's username. Post-scrub these are
	// all "~"; the literal placeholder "/Users/<user>" does not match (the char
	// class requires an alphanumeric immediately after the slash).
	{"macos-home-path", regexp.MustCompile(`/Users/[A-Za-z0-9][A-Za-z0-9._-]*`)},
	{"openai-anthropic-key", regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{10,}`)},
	{"aws-access-key", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{"github-token", regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs)_[A-Za-z0-9]{20,}\b`)},
	{"github-pat", regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}\b`)},
	{"pem-private-key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
}

// Scan deterministically inspects every line of every staged file for PII/secret
// leaks. It applies the built-in structural rules plus a case-insensitive
// substring match for each denylist term (typically the operator's own username
// and git email, auto-derived by the caller). Results are sorted by file, then
// line, then rule, so the output is stable across runs.
func Scan(files map[string]string, denylist []string) []Violation {
	lowered := make([]string, 0, len(denylist))
	for _, d := range denylist {
		if t := strings.ToLower(strings.TrimSpace(d)); t != "" {
			lowered = append(lowered, t)
		}
	}
	var out []Violation
	for name, content := range files {
		for i, line := range strings.Split(content, "\n") {
			lineNo := i + 1
			for _, r := range structuralRules {
				if m := r.re.FindString(line); m != "" {
					out = append(out, Violation{File: name, Line: lineNo, Rule: r.name, Match: m})
				}
			}
			if len(lowered) > 0 {
				low := strings.ToLower(line)
				for _, term := range lowered {
					if strings.Contains(low, term) {
						out = append(out, Violation{File: name, Line: lineNo, Rule: "denylist", Match: term})
					}
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Rule < out[j].Rule
	})
	return out
}
