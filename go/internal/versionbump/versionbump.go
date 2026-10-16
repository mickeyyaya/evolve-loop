// Package versionbump updates the version markers of a release atomically.
// See docs/architecture/packages/internal-versionbump.md.
package versionbump

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/semvercheck"
)

type Result struct {
	Target   string   `json:"target"`
	Modified []string `json:"modified"`
}

type Paths struct {
	PluginJSON      string
	MarketplaceJSON string
	CodexPluginJSON string
	SkillMD         string
	ReadmeMD        string
}

func DefaultPaths(repoRoot string) Paths {
	return Paths{
		PluginJSON:      filepath.Join(repoRoot, ".claude-plugin", "plugin.json"),
		MarketplaceJSON: filepath.Join(repoRoot, ".claude-plugin", "marketplace.json"),
		CodexPluginJSON: filepath.Join(repoRoot, ".codex-plugin", "plugin.json"),
		SkillMD:         filepath.Join(repoRoot, "skills", "loop", "SKILL.md"),
		ReadmeMD:        filepath.Join(repoRoot, "README.md"),
	}
}

func (p Paths) Files() []string {
	out := make([]string, 0, 5)
	for _, f := range []string{p.PluginJSON, p.MarketplaceJSON, p.CodexPluginJSON, p.SkillMD, p.ReadmeMD} {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

func MajorMinor(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return version
	}
	return parts[0] + "." + parts[1]
}

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
	if label, err := bumpCodexPlugin(paths.CodexPluginJSON, target, dryRun); err != nil {
		return res, err
	} else if label != "" {
		res.Modified = append(res.Modified, label)
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
	if changed, err := BumpReadmeHistory(paths.ReadmeMD, target, now, dryRun); err != nil {
		return res, err
	} else if changed {
		res.Modified = append(res.Modified, "README.md (history)")
	}
	return res, nil
}

func bumpCodexPlugin(path, target string, dryRun bool) (string, error) {
	if path == "" {
		return "", nil
	}
	if _, statErr := os.Stat(path); statErr != nil {
		if !errors.Is(statErr, os.ErrNotExist) {
			return "", fmt.Errorf("versionbump: stat %s: %w", path, statErr)
		}
		return "", nil
	}
	changed, err := BumpJSONVersion(path, target, dryRun)
	if err != nil {
		return "", err
	}
	if changed {
		return ".codex-plugin/plugin.json", nil
	}
	return "", nil
}

var jsonVersionRE = regexp.MustCompile(`"version"\s*:\s*"([^"]+)"`)

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
	updated := jsonVersionRE.ReplaceAllString(string(data), fmt.Sprintf(`"version": "%s"`, target))
	if err := atomicwrite.Bytes(path, []byte(updated)); err != nil {
		return false, err
	}
	return true, nil
}

var skillHeadingRE = regexp.MustCompile(`(?m)^# Evolve Loop v([0-9]+\.[0-9]+)(?:\.[0-9]+)?`)

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

var readmeCurrentRE = regexp.MustCompile(`Current \(v([0-9]+\.[0-9]+)\)`)

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

var readmeHistoryRowRE = regexp.MustCompile(`(?m)^\| v[0-9]+\.[0-9]+ \|`)

func HasHistoryRow(path, majorMinor string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	pattern := regexp.MustCompile(fmt.Sprintf(`(?m)^\| v%s \|`, regexp.QuoteMeta(majorMinor)))
	return pattern.MatchString(string(data)), nil
}

var readFile = os.ReadFile

func BumpReadmeHistory(path, version string, now time.Time, dryRun bool) (bool, error) {
	majorMinor := MajorMinor(version)
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
	data, err := readFile(path)
	if err != nil {
		return false, err
	}
	lines := strings.Split(string(data), "\n")
	firstVRow := slices.IndexFunc(lines, readmeHistoryRowRE.MatchString)
	if firstVRow < 0 {
		return false, nil
	}
	newRow := fmt.Sprintf(
		"| v%s | %s | See the [%s] section of CHANGELOG.md. |",
		majorMinor, formatHistoryDate(now), version,
	)
	updated := slices.Insert(lines, firstVRow, newRow)
	return true, atomicwrite.Bytes(path, []byte(strings.Join(updated, "\n")))
}

func formatHistoryDate(t time.Time) string {
	return fmt.Sprintf("%s %d", t.Format("Jan"), t.Day())
}

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
		b.WriteString(m)
		b.WriteByte('"')
	}
	b.WriteString("]}\n")
	return b.String()
}
