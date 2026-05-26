// Package releaseconsistency ports the version-consistency-check half of
// legacy/scripts/utility/release.sh.
//
// Verifies all version markers are consistent with the target version:
//
//	.claude-plugin/plugin.json        json "version" field
//	.claude-plugin/marketplace.json   json "version" field
//	skills/evolve-loop/SKILL.md       `# Evolve Loop vX.Y` heading
//	README.md                          `Current (vX.Y)` cell
//	CHANGELOG.md                       contains `[<target>]` entry
//	README.md history                  contains `v<major.minor>` row
//
// The plugin-cache-refresh half of release.sh (~100 lines of marketplace
// pull + python registry update) is intentionally NOT ported — it's
// environment-specific and removed entirely in v12.0.0 when legacy/
// vanishes. The cache refresh is also covered by `evolve marketplace-poll`
// for the in-pipeline flow.
//
// Exit codes (cmd layer maps from ErrInconsistent):
//
//	0 = all markers consistent with target
//	1 = at least one inconsistency (ErrInconsistent)
package releaseconsistency

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrInconsistent is returned when one or more markers don't match.
var ErrInconsistent = errors.New("releaseconsistency: marker(s) inconsistent")

// Options drives a Check() invocation.
type Options struct {
	ProjectRoot string
	Target      string // optional; if empty, derived from plugin.json
	Stderr      io.Writer
}

// Result captures per-check outcomes.
type Result struct {
	Target     string
	MajorMinor string
	Canonical  string // version from plugin.json
	Checks     []Check
	Errors     int
}

// Check is one marker's verification outcome.
type Check struct {
	File        string
	Description string
	Status      string // "OK" | "MISSING" | "NO_MATCH" | "MISMATCH"
	Found       string
	Expected    string
}

var jsonVersionRE = regexp.MustCompile(`"version"[[:space:]]*:[[:space:]]*"([^"]*)"`)
var skillHeadingRE = regexp.MustCompile(`^# Evolve Loop v([0-9]+\.[0-9]+)`)
var readmeCurrentRE = regexp.MustCompile(`Current \(v([0-9]+\.[0-9]+)\)`)

// majorMinor extracts "X.Y" from "X.Y.Z".
func majorMinor(v string) string {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return v
	}
	return parts[0] + "." + parts[1]
}

// extractJSONVersion reads <file>'s first "version" field.
func extractJSONVersion(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	m := jsonVersionRE.FindStringSubmatch(string(body))
	if len(m) < 2 {
		return "", fmt.Errorf("no version field")
	}
	return m[1], nil
}

// Run executes all 6 marker verifications.
func Run(opts Options) (Result, error) {
	res := Result{}
	if opts.ProjectRoot == "" {
		return res, errors.New("releaseconsistency: ProjectRoot required")
	}
	logw := opts.Stderr
	if logw == nil {
		logw = io.Discard
	}

	pluginJSON := filepath.Join(opts.ProjectRoot, ".claude-plugin", "plugin.json")
	canonical, err := extractJSONVersion(pluginJSON)
	if err != nil {
		return res, fmt.Errorf("%w: cannot read plugin.json: %v", ErrInconsistent, err)
	}
	res.Canonical = canonical

	target := opts.Target
	if target == "" {
		target = canonical
	}
	res.Target = target
	res.MajorMinor = majorMinor(target)

	fmt.Fprintln(logw, "")
	fmt.Fprintln(logw, "=== evolve-loop release checklist ===")
	fmt.Fprintln(logw, "")
	fmt.Fprintf(logw, "Target version:    %s\n", target)
	fmt.Fprintf(logw, "Canonical version: %s (plugin.json)\n", canonical)
	if target != canonical {
		fmt.Fprintf(logw, "WARNING: target version differs from plugin.json\n")
	}
	fmt.Fprintln(logw, "")
	fmt.Fprintln(logw, "--- Version strings ---")

	checks := []struct {
		file        string
		description string
		check       func() Check
	}{
		{".claude-plugin/plugin.json", "plugin.json version",
			func() Check {
				return checkJSONVersion(opts.ProjectRoot, ".claude-plugin/plugin.json", "plugin.json version", target)
			}},
		{".claude-plugin/marketplace.json", "marketplace.json version",
			func() Check {
				return checkJSONVersion(opts.ProjectRoot, ".claude-plugin/marketplace.json", "marketplace.json version", target)
			}},
		{"skills/evolve-loop/SKILL.md", "SKILL.md heading (major.minor)",
			func() Check { return checkSkillHeading(opts.ProjectRoot, res.MajorMinor) }},
		{"README.md", "README.md current version table",
			func() Check { return checkReadmeCurrent(opts.ProjectRoot, res.MajorMinor) }},
		{"CHANGELOG.md", fmt.Sprintf("CHANGELOG.md entry for %s", target),
			func() Check {
				return checkContains(opts.ProjectRoot, "CHANGELOG.md", "["+target+"]", fmt.Sprintf("CHANGELOG.md entry for %s", target))
			}},
		{"README.md", fmt.Sprintf("README.md version history row for v%s", res.MajorMinor),
			func() Check {
				return checkContains(opts.ProjectRoot, "README.md", "v"+res.MajorMinor, fmt.Sprintf("README.md version history row for v%s", res.MajorMinor))
			}},
	}

	contentMarkerStart := 4 // index 4 = CHANGELOG; 5 = README history
	for i, c := range checks {
		if i == contentMarkerStart {
			fmt.Fprintln(logw, "")
			fmt.Fprintln(logw, "--- Required content ---")
		}
		ch := c.check()
		res.Checks = append(res.Checks, ch)
		switch ch.Status {
		case "OK":
			fmt.Fprintf(logw, "OK       %s — %s (%s)\n", ch.File, ch.Description, ch.Found)
		case "MISSING":
			fmt.Fprintf(logw, "MISSING  %s — %s\n", ch.File, ch.Description)
			res.Errors++
		case "NO_MATCH":
			fmt.Fprintf(logw, "NO MATCH %s — %s\n", ch.File, ch.Description)
			res.Errors++
		case "MISMATCH":
			fmt.Fprintf(logw, "MISMATCH %s — found: %s, expected: %s\n", ch.File, ch.Found, ch.Expected)
			res.Errors++
		}
	}

	fmt.Fprintln(logw, "")
	if res.Errors > 0 {
		fmt.Fprintf(logw, "FAILED: %d issue(s) found. Fix before releasing.\n", res.Errors)
		return res, fmt.Errorf("%w: %d marker(s)", ErrInconsistent, res.Errors)
	}
	fmt.Fprintln(logw, "PASSED: All version references are consistent.")
	return res, nil
}

// --- Per-check helpers -----------------------------------------------------

func checkJSONVersion(repoRoot, relPath, desc, target string) Check {
	c := Check{File: relPath, Description: desc, Expected: target}
	full := filepath.Join(repoRoot, relPath)
	if _, err := os.Stat(full); err != nil {
		c.Status = "MISSING"
		return c
	}
	v, err := extractJSONVersion(full)
	if err != nil || v == "" {
		c.Status = "NO_MATCH"
		return c
	}
	c.Found = v
	if target != "" && v != target {
		c.Status = "MISMATCH"
		return c
	}
	c.Status = "OK"
	return c
}

func checkSkillHeading(repoRoot, majorMinor string) Check {
	relPath := "skills/evolve-loop/SKILL.md"
	c := Check{File: relPath, Description: "SKILL.md heading (major.minor)", Expected: majorMinor}
	full := filepath.Join(repoRoot, relPath)
	body, err := os.ReadFile(full)
	if err != nil {
		c.Status = "MISSING"
		return c
	}
	for _, line := range strings.Split(string(body), "\n") {
		if m := skillHeadingRE.FindStringSubmatch(line); len(m) >= 2 {
			c.Found = m[1]
			if majorMinor != "" && m[1] != majorMinor {
				c.Status = "MISMATCH"
				return c
			}
			c.Status = "OK"
			return c
		}
	}
	c.Status = "NO_MATCH"
	return c
}

func checkReadmeCurrent(repoRoot, majorMinor string) Check {
	relPath := "README.md"
	c := Check{File: relPath, Description: "README.md current version table", Expected: majorMinor}
	full := filepath.Join(repoRoot, relPath)
	body, err := os.ReadFile(full)
	if err != nil {
		c.Status = "MISSING"
		return c
	}
	m := readmeCurrentRE.FindStringSubmatch(string(body))
	if len(m) < 2 {
		c.Status = "NO_MATCH"
		return c
	}
	c.Found = m[1]
	if majorMinor != "" && m[1] != majorMinor {
		c.Status = "MISMATCH"
		return c
	}
	c.Status = "OK"
	return c
}

func checkContains(repoRoot, relPath, pattern, desc string) Check {
	c := Check{File: relPath, Description: desc, Expected: pattern}
	full := filepath.Join(repoRoot, relPath)
	body, err := os.ReadFile(full)
	if err != nil {
		c.Status = "MISSING"
		return c
	}
	if strings.Contains(string(body), pattern) {
		c.Status = "OK"
		c.Found = pattern
		return c
	}
	c.Status = "MISSING"
	return c
}
