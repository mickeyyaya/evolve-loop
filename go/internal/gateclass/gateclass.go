// Package gateclass is the deterministic gate classifier of the self-healing loop (ADR-0155, component X15): it names the surface tier of a path and flags a cycle diff that changes a kernel surface or weakens a gate.
// See docs/architecture/packages/internal-gateclass.md.
package gateclass

import (
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type Tier string

const (
	T0 Tier = "T0"
	T1 Tier = "T1"
	T2 Tier = "T2"
)

type Pattern string

const (
	PatternProtectedPath       Pattern = "protected-path"
	PatternRemovedGuardTest    Pattern = "removed-guard-test"
	PatternAddedSkip           Pattern = "added-skip"
	PatternChangedThreshold    Pattern = "changed-threshold"
	PatternStageOffEnforce     Pattern = "stage-off-enforce"
	PatternFrozenCheck         Pattern = "frozen-check"
	PatternHealingConstitution Pattern = "healing-constitution"
)

var t1Prefixes = []string{
	"go/internal/core/",
	"go/internal/bridge/",
	"go/internal/cliroute/",
	"go/internal/gc/",
	"agents/",
}

type Flag struct {
	Pattern Pattern `json:"pattern"`
	Path    string  `json:"path"`
	Detail  string  `json:"detail,omitempty"`
}

type Input struct {
	Diff         string
	Untracked    []string
	Cycle        int
	Protected    func(string) bool
	Constitution func(string) bool
}

func hasAnyPrefix(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func holds(pred func(string) bool, path string) bool {
	return pred != nil && pred(path)
}

func TierOf(path string, protected func(string) bool) Tier {
	switch {
	case holds(protected, path):
		return T0
	case hasAnyPrefix(path, t1Prefixes):
		return T1
	}
	return T2
}

func Classify(in Input) []Flag {
	files := parseDiff(in.Diff)
	movedTests := addedTestNames(files)
	var out []Flag
	for _, f := range files {
		out = append(out, f.flags(in, movedTests)...)
	}
	for _, path := range in.Untracked {
		out = append(out, in.pathFlags(path)...)
	}
	return out
}

func (in Input) pathFlags(path string) []Flag {
	var out []Flag
	if holds(in.Protected, path) {
		out = append(out, Flag{Pattern: PatternProtectedPath, Path: path})
	}
	if holds(in.Constitution, path) {
		out = append(out, Flag{Pattern: PatternHealingConstitution, Path: path})
	}
	return out
}

type fileDiff struct {
	path    string
	created bool
	removed []string
	added   []string
}

func parseDiff(diff string) []fileDiff {
	var files []fileDiff
	inHunk := false
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			files, inHunk = append(files, fileDiff{path: diffPath(line)}), false
			continue
		}
		if len(files) == 0 {
			continue
		}
		f := &files[len(files)-1]
		switch {
		case strings.HasPrefix(line, "@@"):
			inHunk = true
		case !inHunk:
			f.created = f.created || strings.HasPrefix(line, "new file mode")
		case strings.HasPrefix(line, "-"):
			f.removed = append(f.removed, line[1:])
		case strings.HasPrefix(line, "+"):
			f.added = append(f.added, line[1:])
		}
	}
	return files
}

func diffPath(header string) string {
	name, _, _ := strings.Cut(strings.TrimPrefix(header, "diff --git a/"), " b/")
	return name
}

func (f fileDiff) flags(in Input, movedTests map[string][]string) []Flag {
	out := in.pathFlags(f.path)
	out = append(out, f.removedGuardTests(movedTests)...)
	out = append(out, f.addedSkips()...)
	if !f.isTest() {
		out = append(out, f.changedThresholds()...)
		out = append(out, f.stagesOffEnforce()...)
	}
	if f.changesFrozenCheck(in.Cycle) {
		out = append(out, Flag{Pattern: PatternFrozenCheck, Path: f.path})
	}
	return out
}

var testFunc = regexp.MustCompile(`^func (Test\w*)\(`)

var guardTestPath = regexp.MustCompile(`(^|/)(guards|repocontract|sizeratchet|rawgitratchet)/|guard[^/]*_test\.go$|^go/acs/regression/`)

func addedTestNames(files []fileDiff) map[string][]string {
	names := map[string][]string{}
	for _, f := range files {
		for _, line := range f.added {
			if m := testFunc.FindStringSubmatch(line); m != nil {
				names[m[1]] = append(names[m[1]], f.path)
			}
		}
	}
	return names
}

func movedTo(from string, targets []string) bool {
	return slices.ContainsFunc(targets, func(to string) bool {
		return guardTestPath.MatchString(to) || path.Dir(to) == path.Dir(from)
	})
}

func (f fileDiff) removedGuardTests(movedTests map[string][]string) []Flag {
	var out []Flag
	for _, line := range f.removed {
		m := testFunc.FindStringSubmatch(line)
		if m == nil || movedTo(f.path, movedTests[m[1]]) {
			continue
		}
		if guardTestPath.MatchString(f.path) || strings.Contains(m[1], "Contract") {
			out = append(out, Flag{Pattern: PatternRemovedGuardTest, Path: f.path, Detail: m[1]})
		}
	}
	return out
}

var skipCall = regexp.MustCompile(`\b\w+\.Skip(?:f|Now)?\(.*`)

func (f fileDiff) isTest() bool { return strings.HasSuffix(f.path, "_test.go") }

func (f fileDiff) addedSkips() []Flag {
	if f.created || !f.isTest() {
		return nil
	}
	kept := map[string]int{}
	for _, line := range f.removed {
		if m := skipCall.FindString(line); m != "" {
			kept[m]++
		}
	}
	var out []Flag
	for _, line := range f.added {
		m := skipCall.FindString(line)
		switch {
		case m == "":
		case kept[m] > 0:
			kept[m]--
		default:
			out = append(out, Flag{Pattern: PatternAddedSkip, Path: f.path, Detail: m})
		}
	}
	return out
}

var numericSetting = regexp.MustCompile(`^\s*(?:(?:const|var)\s+)?"?([A-Za-z_]\w*)"?\s*(?:[A-Za-z_][\w.]*\s*)?[:=]\s*(-?\d+(?:\.\d+)?)\b`)

var thresholdName = regexp.MustCompile(`(?i)threshold|budget|floor|halt|ratchet|coverage`)

func (f fileDiff) changedThresholds() []Flag {
	before := map[string]string{}
	for _, line := range f.removed {
		if m := numericSetting.FindStringSubmatch(line); m != nil && thresholdName.MatchString(m[1]) {
			before[m[1]] = m[2]
		}
	}
	var out []Flag
	for _, line := range f.added {
		m := numericSetting.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if old, ok := before[m[1]]; ok && old != m[2] {
			out = append(out, Flag{Pattern: PatternChangedThreshold, Path: f.path, Detail: m[1] + " " + old + " -> " + m[2]})
		}
	}
	return out
}

var stageSetting = regexp.MustCompile(`(?i)"?(\w*stage\w*)"?\s*[:=]\s*(?:"(enforce|shadow|off)"|[\w.]*Stage(Enforce|Shadow|Off)\b)`)

func stageOf(line string) (key, value string, ok bool) {
	m := stageSetting.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	return m[1], strings.ToLower(m[2] + m[3]), true
}

func (f fileDiff) stagesOffEnforce() []Flag {
	enforced := map[string]bool{}
	for _, line := range f.removed {
		if key, value, ok := stageOf(line); ok && value == "enforce" {
			enforced[key] = true
		}
	}
	var out []Flag
	for _, line := range f.added {
		if key, value, ok := stageOf(line); ok && enforced[key] && value != "enforce" {
			out = append(out, Flag{Pattern: PatternStageOffEnforce, Path: f.path, Detail: key + " enforce -> " + value})
		}
	}
	return out
}

func (f fileDiff) changesFrozenCheck(cycle int) bool {
	own := "go/acs/cycle" + strconv.Itoa(cycle) + "/"
	return !f.created && strings.HasPrefix(f.path, "go/acs/") && (cycle <= 0 || !strings.HasPrefix(f.path, own))
}

func diffArgs(base string) []string {
	return []string{"-c", "core.quotePath=false", "diff", "--no-color", "--no-ext-diff", "-U0", "--no-renames", base}
}

func untrackedArgs() []string {
	return []string{"-c", "core.quotePath=false", "ls-files", "--others", "--exclude-standard", "-z"}
}

func Read(git func(args ...string) (string, error), base string) (Input, error) {
	diff, err := git(diffArgs(base)...)
	if err != nil {
		return Input{}, err
	}
	untracked, err := git(untrackedArgs()...)
	if err != nil {
		return Input{}, err
	}
	var paths []string
	for _, p := range strings.Split(untracked, "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return Input{Diff: diff, Untracked: paths}, nil
}
