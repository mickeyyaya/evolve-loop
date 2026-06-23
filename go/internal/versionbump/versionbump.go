// Package versionbump ports legacy/scripts/release/version-bump.sh.
//
// Atomic version updater across the canonical version markers:
//   - .claude-plugin/plugin.json         (always)
//   - .claude-plugin/marketplace.json    (always; updates every plugins[].version)
//   - skills/loop/SKILL.md        (only major.minor)
//   - README.md "Current (vX.Y)" cell    (only major.minor)
//   - README.md history table            (only major.minor; adds new row)
//
// Atomicity: every write goes through `<file>.tmp` + rename. Partial
// bump = mismatched JSON files; the next release.sh consistency check
// catches it.
//
// Idempotency: a marker already at the target is a silent no-op; the
// per-file Bump funcs return (didChange=false) without writing.
package versionbump

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolveloop/go/internal/semvercheck"
)

// Result is the JSON-compatible summary version-bump prints to stdout.
type Result struct {
	Target   string   `json:"target"`
	Modified []string `json:"modified"`
}

// Paths gathers the file paths version-bump touches. Defaults derived
// from a repoRoot — tests pin individual paths for isolation.
type Paths struct {
	PluginJSON      string // <repo>/.claude-plugin/plugin.json
	MarketplaceJSON string // <repo>/.claude-plugin/marketplace.json
	SkillMD         string // <repo>/skills/loop/SKILL.md
	ReadmeMD        string // <repo>/README.md
}

// DefaultPaths returns the canonical layout under repoRoot.
func DefaultPaths(repoRoot string) Paths {
	return Paths{
		PluginJSON:      filepath.Join(repoRoot, ".claude-plugin", "plugin.json"),
		MarketplaceJSON: filepath.Join(repoRoot, ".claude-plugin", "marketplace.json"),
		SkillMD:         filepath.Join(repoRoot, "skills", "loop", "SKILL.md"),
		ReadmeMD:        filepath.Join(repoRoot, "README.md"),
	}
}

// MajorMinor extracts "X.Y" from "X.Y.Z".
func MajorMinor(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return version
	}
	return parts[0] + "." + parts[1]
}

// Run applies the full bump pipeline. Returns the modified-list result.
// When dryRun is true, mutations are skipped but the would-modify list
// still populates (mirrors bash --dry-run semantics).
func Run(paths Paths, target string, dryRun bool, now time.Time) (Result, error) {
	if !semvercheck.IsSemver(target) {
		return Result{}, fmt.Errorf("versionbump: target version not semver: %s", target)
	}
	res := Result{Target: target, Modified: []string{}}
	mm := MajorMinor(target)

	if changed, err := BumpJSONVersion(paths.PluginJSON, target, dryRun); err != nil {
		return res, err
	} else if changed {
		res.Modified = append(res.Modified, ".claude-plugin/plugin.json")
	}
	if changed, err := BumpJSONVersion(paths.MarketplaceJSON, target, dryRun); err != nil {
		return res, err
	} else if changed {
		res.Modified = append(res.Modified, ".claude-plugin/marketplace.json")
	}
	if changed, err := BumpSkillHeading(paths.SkillMD, mm, dryRun); err != nil {
		return res, err
	} else if changed {
		res.Modified = append(res.Modified, "skills/loop/SKILL.md")
	}
	if changed, err := BumpReadmeCurrent(paths.ReadmeMD, mm, dryRun); err != nil {
		return res, err
	} else if changed {
		res.Modified = append(res.Modified, "README.md (Current)")
	}
	if changed, err := BumpReadmeHistory(paths.ReadmeMD, mm, now, dryRun); err != nil {
		return res, err
	} else if changed {
		res.Modified = append(res.Modified, "README.md (history)")
	}
	return res, nil
}

// --- JSON version bumps ---

// jsonVersionRE finds `"version": "<x>"` for current-version reads. The
// bash original uses sed; we keep a regex for the same lenient behavior.
var jsonVersionRE = regexp.MustCompile(`"version"\s*:\s*"([^"]+)"`)

// CurrentJSONVersion returns the first "version" string found in the
// file, or "" on miss. Matches bash current_json_version.
func CurrentJSONVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	m := jsonVersionRE.FindStringSubmatch(string(data))
	if len(m) < 2 {
		return "", nil
	}
	return m[1], nil
}

// BumpJSONVersion rewrites both top-level .version and every
// .plugins[].version to target. Idempotent: returns (false, nil) when
// already at target. Atomic via .tmp + rename.
func BumpJSONVersion(path, target string, dryRun bool) (bool, error) {
	current, err := CurrentJSONVersion(path)
	if err != nil {
		return false, fmt.Errorf("versionbump: read %s: %w", path, err)
	}
	if current == target {
		return false, nil
	}
	if dryRun {
		return true, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("versionbump: read %s: %w", path, err)
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return false, fmt.Errorf("versionbump: parse %s: %w", path, err)
	}
	if _, ok := obj["version"]; ok {
		obj["version"] = target
	}
	if plugins, ok := obj["plugins"].([]any); ok {
		for i, p := range plugins {
			if pm, ok := p.(map[string]any); ok {
				if _, has := pm["version"]; has {
					pm["version"] = target
					plugins[i] = pm
				}
			}
		}
		obj["plugins"] = plugins
	}
	// Re-encoding via json.MarshalIndent would reorder keys; the bash
	// version uses jq which preserves order. To stay close to bash output
	// (which the operator + diff tools care about), we do an in-place
	// string substitution of just the version values instead.
	updated := jsonVersionRE.ReplaceAllString(string(data), fmt.Sprintf(`"version": "%s"`, target))
	if err := atomicwrite.Bytes(path, []byte(updated)); err != nil {
		return false, err
	}
	return true, nil
}

// --- SKILL.md heading bump ---

var skillHeadingRE = regexp.MustCompile(`(?m)^# Evolve Loop v([0-9]+\.[0-9]+)(?:\.[0-9]+)?`)

// CurrentSkillHeading returns the "X.Y" from the SKILL.md "# Evolve Loop
// vX.Y[.Z]" heading, or "" on miss.
func CurrentSkillHeading(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	m := skillHeadingRE.FindStringSubmatch(string(data))
	if len(m) < 2 {
		return "", nil
	}
	return m[1], nil
}

// BumpSkillHeading rewrites the first SKILL.md "# Evolve Loop vX.Y" heading
// to majorMinor. Idempotent and missing-file tolerant: returns (false, nil)
// when already at majorMinor, the heading is absent, or the file is missing.
// Atomic via .tmp + rename.
func BumpSkillHeading(path, majorMinor string, dryRun bool) (bool, error) {
	current, err := CurrentSkillHeading(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("versionbump: read %s: %w", path, err)
	}
	if current == "" || current == majorMinor {
		return false, nil
	}
	if dryRun {
		return true, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	// Bash awk uses `sub(/v[0-9]+\.[0-9]+(\.[0-9]+)?/, "v" target)` on the
	// FIRST matching heading only. Mirror with a one-shot replacement.
	replaced := false
	out := skillHeadingRE.ReplaceAllStringFunc(string(data), func(match string) string {
		if replaced {
			return match
		}
		replaced = true
		return "# Evolve Loop v" + majorMinor
	})
	if err := atomicwrite.Bytes(path, []byte(out)); err != nil {
		return false, err
	}
	return true, nil
}

// --- README "Current (vX.Y)" bump ---

var readmeCurrentRE = regexp.MustCompile(`Current \(v([0-9]+\.[0-9]+)\)`)

// CurrentReadmeCurrent returns the "X.Y" from the README "Current (vX.Y)"
// cell, or "" on miss.
func CurrentReadmeCurrent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	m := readmeCurrentRE.FindStringSubmatch(string(data))
	if len(m) < 2 {
		return "", nil
	}
	return m[1], nil
}

// BumpReadmeCurrent rewrites the README "Current (vX.Y)" cell to majorMinor.
// Idempotent and missing-file tolerant: returns (false, nil) when already at
// majorMinor, the cell is absent, or the file is missing. Atomic via .tmp +
// rename.
func BumpReadmeCurrent(path, majorMinor string, dryRun bool) (bool, error) {
	current, err := CurrentReadmeCurrent(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("versionbump: read %s: %w", path, err)
	}
	if current == "" || current == majorMinor {
		return false, nil
	}
	if dryRun {
		return true, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	out := readmeCurrentRE.ReplaceAllString(string(data), fmt.Sprintf("Current (v%s)", majorMinor))
	return true, atomicwrite.Bytes(path, []byte(out))
}

// --- README history-table row bump ---

var readmeHistoryRowRE = regexp.MustCompile(`(?m)^\| v[0-9]+\.[0-9]+ \|`)

// HasHistoryRow reports whether the table already has a row for the given
// major.minor. Mirrors bash `grep -qE "^\| v${target} \|"`.
func HasHistoryRow(path, majorMinor string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	pattern := regexp.MustCompile(fmt.Sprintf(`(?m)^\| v%s \|`, regexp.QuoteMeta(majorMinor)))
	return pattern.MatchString(string(data)), nil
}

// BumpReadmeHistory inserts a new row "| v<mm> | <today> | TBD ... |"
// just AFTER the last contiguous v-row in the table. Bash uses awk to
// peek the next line and insert if that line is not also a v-row; we
// implement the same insert-after-last-vrow rule.
func BumpReadmeHistory(path, majorMinor string, now time.Time, dryRun bool) (bool, error) {
	has, err := HasHistoryRow(path, majorMinor)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("versionbump: read %s: %w", path, err)
	}
	if has {
		return false, nil
	}
	if dryRun {
		return true, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	// Find the last `| vX.Y |` line and insert the new row after it.
	lines := strings.Split(string(data), "\n")
	lastVRow := -1
	for i, line := range lines {
		if readmeHistoryRowRE.MatchString(line) {
			lastVRow = i
		}
	}
	if lastVRow < 0 {
		// No history table exists — leave the file alone (bash awk does
		// the same: it scans for v-rows; if none found, no insert).
		return false, nil
	}
	today := formatHistoryDate(now)
	newRow := fmt.Sprintf(
		"| v%s | %s | TBD — fill in via release-pipeline.sh + changelog-gen.sh |",
		majorMinor, today,
	)
	updated := append([]string{}, lines[:lastVRow+1]...)
	updated = append(updated, newRow)
	updated = append(updated, lines[lastVRow+1:]...)
	return true, atomicwrite.Bytes(path, []byte(strings.Join(updated, "\n")))
}

// formatHistoryDate mirrors bash `date +"%b %-d"` ("May 24", "May 4").
// Go's "Jan _2" pads single-digit days with a leading space — strip it.
func formatHistoryDate(t time.Time) string {
	return fmt.Sprintf("%s %d", t.Format("Jan"), t.Day())
}

// ResultJSON renders the bash-compatible JSON summary line. Returns it
// WITH a trailing newline, matching the bash `printf '...\n'` call.
func (r Result) ResultJSON() string {
	var b strings.Builder
	b.WriteString(`{"target":"`)
	b.WriteString(r.Target)
	b.WriteString(`","modified":[`)
	for i, m := range r.Modified {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		// Bash builds the list with raw string interpolation; we mirror
		// (no escape needed since the names are literals controlled by us).
		b.WriteString(m)
		b.WriteByte('"')
	}
	b.WriteString("]}\n")
	return b.String()
}
