// Package releaseconsistency ports the version-consistency-check half of
// legacy/scripts/utility/release.sh.
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

var ErrInconsistent = errors.New("releaseconsistency: marker(s) inconsistent")

type Options struct {
	ProjectRoot string
	Target      string
	Stderr      io.Writer
}

type Result struct {
	Target     string
	MajorMinor string
	Canonical  string
	Checks     []Check
	Errors     int
}

type Check struct {
	File        string
	Description string
	Status      string
	Found       string
	Expected    string
}

var jsonVersionRE = regexp.MustCompile(`"version"[[:space:]]*:[[:space:]]*"([^"]*)"`)
var skillHeadingRE = regexp.MustCompile(`^# Evolve Loop v([0-9]+\.[0-9]+)`)
var readmeCurrentRE = regexp.MustCompile(`Current \(v([0-9]+\.[0-9]+)\)`)

func majorMinor(v string) string {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return v
	}
	return parts[0] + "." + parts[1]
}

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

	logChecklistHeader(logw, target, canonical)
	checks := buildChecks(opts, target, res.MajorMinor)
	results, errs := runChecks(logw, checks)
	res.Checks = results
	res.Errors = errs

	fmt.Fprintln(logw, "")
	if res.Errors > 0 {
		fmt.Fprintf(logw, "FAILED: %d issue(s) found. Fix before releasing.\n", res.Errors)
		return res, fmt.Errorf("%w: %d marker(s)", ErrInconsistent, res.Errors)
	}
	fmt.Fprintln(logw, "PASSED: All version references are consistent.")
	return res, nil
}

type checkSpec struct {
	file        string
	description string
	check       func() Check
}

func logChecklistHeader(logw io.Writer, target, canonical string) {
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
}

func buildChecks(opts Options, target, majorMinor string) []checkSpec {
	return []checkSpec{
		{".claude-plugin/plugin.json", "plugin.json version",
			func() Check {
				return checkJSONVersion(opts.ProjectRoot, ".claude-plugin/plugin.json", "plugin.json version", target)
			}},
		{".claude-plugin/marketplace.json", "marketplace.json version",
			func() Check {
				return checkJSONVersion(opts.ProjectRoot, ".claude-plugin/marketplace.json", "marketplace.json version", target)
			}},
		{".codex-plugin/plugin.json", "codex plugin.json version (generated mirror)",
			func() Check {
				return checkJSONVersionIfPresent(opts.ProjectRoot, ".codex-plugin/plugin.json", "codex plugin.json version (generated mirror)", target)
			}},
		{"skills/loop/SKILL.md", "SKILL.md heading (major.minor)",
			func() Check { return checkSkillHeading(opts.ProjectRoot, majorMinor) }},
		{"README.md", "README.md current version table",
			func() Check { return checkReadmeCurrent(opts.ProjectRoot, majorMinor) }},
		{"CHANGELOG.md", fmt.Sprintf("CHANGELOG.md entry for %s", target),
			func() Check {
				return checkContains(opts.ProjectRoot, "CHANGELOG.md", "["+target+"]", fmt.Sprintf("CHANGELOG.md entry for %s", target))
			}},
		{"README.md", fmt.Sprintf("README.md version history row for v%s", majorMinor),
			func() Check {
				return checkContains(opts.ProjectRoot, "README.md", "v"+majorMinor, fmt.Sprintf("README.md version history row for v%s", majorMinor))
			}},
	}
}

const contentMarkerStart = 5

func runChecks(logw io.Writer, checks []checkSpec) ([]Check, int) {
	var results []Check
	errs := 0
	for i, c := range checks {
		if i == contentMarkerStart {
			fmt.Fprintln(logw, "")
			fmt.Fprintln(logw, "--- Required content ---")
		}
		ch := c.check()
		results = append(results, ch)
		switch ch.Status {
		case "OK":
			fmt.Fprintf(logw, "OK       %s — %s (%s)\n", ch.File, ch.Description, ch.Found)
		case "MISSING":
			fmt.Fprintf(logw, "MISSING  %s — %s\n", ch.File, ch.Description)
			errs++
		case "NO_MATCH":
			fmt.Fprintf(logw, "NO MATCH %s — %s\n", ch.File, ch.Description)
			errs++
		case "MISMATCH":
			fmt.Fprintf(logw, "MISMATCH %s — found: %s, expected: %s\n", ch.File, ch.Found, ch.Expected)
			errs++
		}
	}
	return results, errs
}

func checkJSONVersionIfPresent(repoRoot, relPath, desc, target string) Check {
	if _, err := os.Stat(filepath.Join(repoRoot, relPath)); errors.Is(err, os.ErrNotExist) {
		return Check{File: relPath, Description: desc, Status: "OK", Found: "absent (skipped)"}
	}
	return checkJSONVersion(repoRoot, relPath, desc, target)
}

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
	relPath := "skills/loop/SKILL.md"
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
