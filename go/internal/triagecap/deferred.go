package triagecap

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// deferredHeadingRE matches by word, case-insensitively: models add suffixes and sometimes capitalize the headings.
var deferredHeadingRE = regexp.MustCompile(`(?mi)^## (?:deferred|dropped)\b`)

// DeferredFloorPackages returns the sorted candidates mentioned by floor items in ## deferred and ## dropped.
func DeferredFloorPackages(artifact string, candidatePkgs []string) []string {
	seen := map[string]bool{}
	for _, loc := range deferredHeadingRE.FindAllStringIndex(artifact, -1) {
		body := artifact[loc[1]:]
		if next := nextHeadingRE.FindStringIndex(body); next != nil {
			body = body[:next[0]]
		}
		for _, m := range listItemRE.FindAllStringSubmatch(body, -1) {
			item := floorItem(m[1])
			if !floorWordRE.MatchString(item) || !floorPercentRE.MatchString(item) {
				continue
			}
			for _, pkg := range mentionedPackages(item, candidatePkgs) {
				seen[pkg] = true
			}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	pkgs := make([]string, 0, len(seen))
	for p := range seen {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	return pkgs
}

// ReadDeferredFloors reads deferred_floors from the companion; a missing file or field is (nil, false, nil).
func ReadDeferredFloors(companionPath string) ([]string, bool, error) {
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
	field, ok := raw["deferred_floors"]
	if !ok {
		return nil, false, nil
	}
	var floors []string
	if err := json.Unmarshal(field, &floors); err != nil {
		return nil, false, fmt.Errorf("deferred_floors: %w", err)
	}
	return floors, true, nil
}

// DeferredFloorPackagesDecl returns declared deferred_floors filtered to candidatePkgs, else the prose scan.
func DeferredFloorPackagesDecl(artifact, companionPath string, candidatePkgs []string) []string {
	if declared, ok, err := ReadDeferredFloors(companionPath); err == nil && ok {
		return filterDeclaredPackages(declared, candidatePkgs)
	}
	return DeferredFloorPackages(artifact, candidatePkgs)
}

// MalformedDeferredFloorWarning names the parse error of a present-but-malformed companion; otherwise "".
func MalformedDeferredFloorWarning(companionPath string) string {
	data, err := os.ReadFile(companionPath)
	if err != nil {
		return ""
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Sprintf("deferred_floors companion malformed (%s): invalid JSON: %v", companionPath, err)
	}
	field, ok := raw["deferred_floors"]
	if !ok {
		return ""
	}
	var floors []string
	if err := json.Unmarshal(field, &floors); err != nil {
		return fmt.Sprintf("deferred_floors malformed (%s): %v", companionPath, err)
	}
	return ""
}

// DeferredFloorDivergence returns a satisfiable correction when prose deferrals and deferred_floors disagree, else "".
func DeferredFloorDivergence(artifact, companionPath string, knownPkgs []string) string {
	declared, ok, err := ReadDeferredFloors(companionPath)
	if err != nil || !ok {
		return ""
	}
	prose := map[string]bool{}
	for _, pkg := range DeferredFloorPackages(artifact, knownPkgs) {
		prose[pkg] = true
	}
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
	return fmt.Sprintf("Prose/declaration deferred floor mismatch: prose-only=[%s], deferred_floors-only=[%s]. Reconcile by adding the prose deferred floor packages to deferred_floors or removing the stale prose deferred floor mention.",
		strings.Join(proseOnly, ", "), strings.Join(declaredOnly, ", "))
}

func filterDeclaredPackages(declared, candidatePkgs []string) []string {
	candidates := map[string]bool{}
	for _, pkg := range candidatePkgs {
		if pkg != "" {
			candidates[pkg] = true
		}
	}
	seen := map[string]bool{}
	for _, pkg := range declared {
		if candidates[pkg] {
			seen[pkg] = true
		}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for pkg := range seen {
		out = append(out, pkg)
	}
	sort.Strings(out)
	return out
}
