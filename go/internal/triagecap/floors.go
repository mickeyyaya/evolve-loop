// Package triagecap bounds per-cycle coverage-floor commitments by observed builder
// throughput, and parses the triage report into floors, the decision companion and fleet seeds.
package triagecap

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/triagedecision"
)

var nextHeadingRE = regexp.MustCompile(`(?m)^## `)

func TriageDecisionName() string { return "triage-decision.json" }

var listItemRE = regexp.MustCompile(`(?m)^[-*]\s+(\S.*)$`)

var (
	floorWordRE    = regexp.MustCompile(`(?i)coverage|floor`)
	floorPercentRE = regexp.MustCompile(`\d+(?:\.\d+)?\s*%`)
)

var targetPercentRE = regexp.MustCompile(`(?:(?:≥|>=)\s*|\bto\s+)\d+(?:\.\d+)?\s*%`)

func CountCommittedFloors(artifact string, knownPkgs []string) int {
	body, ok := topNSection(artifact)
	if !ok {
		return 0
	}
	total := 0
	for _, m := range listItemRE.FindAllStringSubmatch(body, -1) {
		item := floorItem(m[1])
		if !floorWordRE.MatchString(item) || !floorPercentRE.MatchString(item) {
			continue
		}
		n := len(floorTargetPackages(item, knownPkgs))
		if n < 1 {
			n = 1
		}
		total += n
	}
	return total
}

func floorTargetPackages(item string, candidatePkgs []string) []string {
	stripped := metadataFieldRE.ReplaceAllString(item, " ")
	targets := targetPercentRE.FindAllStringIndex(stripped, -1)
	if len(targets) == 0 {
		return nil
	}
	percents := floorPercentRE.FindAllStringIndex(stripped, -1)
	seen := map[string]bool{}
	var pkgs []string
	for _, tgt := range targets {
		start := 0
		for _, p := range percents {
			if p[1] > tgt[0] {
				break
			}
			start = p[1]
		}
		for _, pkg := range packagesInText(stripped[start:tgt[0]], candidatePkgs) {
			if !seen[pkg] {
				seen[pkg] = true
				pkgs = append(pkgs, pkg)
			}
		}
	}
	return pkgs
}

func ReadDeclaredFloors(companionPath string) ([]string, bool, error) {
	data, err := os.ReadFile(companionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, false, err
	}
	field, ok := raw["committed_floors"]
	if !ok {
		return nil, false, nil
	}
	var floors []string
	if err := json.Unmarshal(field, &floors); err != nil {
		return nil, false, fmt.Errorf("committed_floors: %w", err)
	}
	return floors, true, nil
}

func CommittedFloorCount(artifact, companionPath string, knownPkgs []string) int {
	if declared, ok, err := ReadDeclaredFloors(companionPath); err == nil && ok {
		return len(declared)
	}
	return CountCommittedFloors(artifact, knownPkgs)
}

func CommittedFloorPackages(artifact, companionPath string, candidatePkgs []string) []string {
	if declared, ok, err := ReadDeclaredFloors(companionPath); err == nil && ok {
		candidates := map[string]bool{}
		for _, pkg := range candidatePkgs {
			candidates[pkg] = true
		}
		var pkgs []string
		seen := map[string]bool{}
		for _, pkg := range declared {
			if candidates[pkg] && !seen[pkg] {
				seen[pkg] = true
				pkgs = append(pkgs, pkg)
			}
		}
		sort.Strings(pkgs)
		return pkgs
	}
	prose := proseFloorPackages(artifact, candidatePkgs)
	if len(prose) == 0 {
		return nil
	}
	pkgs := make([]string, 0, len(prose))
	for pkg := range prose {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)
	return pkgs
}

func MalformedCommittedFloorWarning(companionPath string) string {
	data, err := os.ReadFile(companionPath)
	if err != nil {
		return ""
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Sprintf("committed_floors companion malformed (%s): invalid JSON: %v", companionPath, err)
	}
	field, ok := raw["committed_floors"]
	if !ok {
		return ""
	}
	var floors []string
	if err := json.Unmarshal(field, &floors); err != nil {
		return fmt.Sprintf("committed_floors malformed (%s): %v", companionPath, err)
	}
	return ""
}

func FloorDivergenceCorrective(artifact, companionPath string, knownPkgs []string) string {
	declared, ok, err := ReadDeclaredFloors(companionPath)
	if err != nil || !ok {
		return ""
	}
	prose := proseFloorPackages(artifact, knownPkgs)
	declaredSet := map[string]bool{}
	for _, pkg := range declared {
		if pkg != "" {
			declaredSet[pkg] = true
		}
	}
	var proseOnly, declaredOnly []string
	for pkg := range prose {
		if !declaredSet[pkg] {
			proseOnly = append(proseOnly, pkg)
		}
	}
	for pkg := range declaredSet {
		if !prose[pkg] {
			declaredOnly = append(declaredOnly, pkg)
		}
	}
	if len(proseOnly) == 0 && len(declaredOnly) == 0 {
		return ""
	}
	sort.Strings(proseOnly)
	sort.Strings(declaredOnly)
	return fmt.Sprintf("Prose/declaration floor mismatch: prose-only=[%s], committed_floors-only=[%s]. Reconcile by adding the prose floor packages to committed_floors or removing the stale prose floor mention.",
		strings.Join(proseOnly, ", "), strings.Join(declaredOnly, ", "))
}

func proseFloorPackages(artifact string, knownPkgs []string) map[string]bool {
	seen := map[string]bool{}
	body, ok := topNSection(artifact)
	if !ok {
		return seen
	}
	for _, m := range listItemRE.FindAllStringSubmatch(body, -1) {
		item := floorItem(m[1])
		if !floorWordRE.MatchString(item) || !floorPercentRE.MatchString(item) {
			continue
		}
		for _, pkg := range floorTargetPackages(item, knownPkgs) {
			seen[pkg] = true
		}
	}
	return seen
}

func topNSection(artifact string) (string, bool) {
	return triagedecision.SectionBody(artifact, "top_n")
}

var tokenRE = regexp.MustCompile(`[A-Za-z0-9_-]+`)

var metadataFieldRE = regexp.MustCompile(`\bdefer_reason=[^\n]*|\b(?:source|priority)=\S+|\bevidence=`)

func floorItem(item string) string {
	_, stripped := triagedecision.SplitDeclaredFiles(item)
	return stripped
}

var pathOnlyPkgs = map[string]*regexp.Regexp{
	"paths": regexp.MustCompile(`/paths(?:[^A-Za-z0-9_-]|$)`),
}

func mentionedPackages(item string, candidatePkgs []string) []string {
	return packagesInText(metadataFieldRE.ReplaceAllString(item, " "), candidatePkgs)
}

func packagesInText(text string, candidatePkgs []string) []string {
	tokens := map[string]bool{}
	for _, tok := range tokenRE.FindAllString(text, -1) {
		tokens[tok] = true
	}
	var pkgs []string
	for _, pkg := range candidatePkgs {
		if pkg == "" {
			continue
		}
		if re, pathOnly := pathOnlyPkgs[pkg]; pathOnly {
			if re.MatchString(text) {
				pkgs = append(pkgs, pkg)
			}
			continue
		}
		if tokens[pkg] {
			pkgs = append(pkgs, pkg)
		}
	}
	return pkgs
}

func KnownPackages(projectRoot string) []string {
	seen := map[string]bool{}
	for _, base := range []string{"go/internal", "go/cmd"} {
		root := filepath.Join(projectRoot, base)
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "testdata" {
				return filepath.SkipDir
			}
			if dirHasGoFiles(path) {
				seen[name] = true
			}
			return nil
		})
	}
	pkgs := make([]string, 0, len(seen))
	for p := range seen {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	return pkgs
}

func dirHasGoFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			return true
		}
	}
	return false
}
