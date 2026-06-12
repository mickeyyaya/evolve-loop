package triagecap

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// deferred.go — R9.3: the deferred/dropped floor vocabulary. Triage's
// ## deferred and ## dropped sections are where the capacity clamp pushes
// overpacked floors; a TDD floor predicate that binds one of THOSE packages
// gates work the cycle never committed to (the cycle-280 failure mode).

// deferredHeadingRE matches the section headings whose floor items are out
// of this cycle's committed scope. The headings carry free-form suffixes
// ("## deferred (carry to NEXT cycle's carryoverTodos)") and models
// occasionally capitalise them, so match by word, case-insensitively.
var deferredHeadingRE = regexp.MustCompile(`(?mi)^## (?:deferred|dropped)\b`)

// DeferredFloorPackages returns the distinct candidate packages (sorted)
// mentioned by floor-bearing items in the artifact's ## deferred and
// ## dropped sections. candidatePkgs is the vocabulary to match — typically
// the floor-predicate target packages the caller extracted, so the result
// is exactly "which of these targets did triage push out of this cycle".
func DeferredFloorPackages(artifact string, candidatePkgs []string) []string {
	seen := map[string]bool{}
	for _, loc := range deferredHeadingRE.FindAllStringIndex(artifact, -1) {
		body := artifact[loc[1]:]
		if next := nextHeadingRE.FindStringIndex(body); next != nil {
			body = body[:next[0]]
		}
		for _, m := range listItemRE.FindAllStringSubmatch(body, -1) {
			item := m[1]
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

// ReadDeferredFloors reads deferred_floors from a triage-decision.json
// companion. Missing files or missing fields are not errors: callers should
// fall back to the prose scanner for backward compatibility.
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

// DeferredFloorPackagesDecl is declaration-primary: if the companion declares
// deferred_floors, the declared packages filtered to candidatePkgs are
// authoritative. Otherwise the legacy prose scanner remains the fail-open
// fallback for older triage artifacts.
func DeferredFloorPackagesDecl(artifact, companionPath string, candidatePkgs []string) []string {
	if declared, ok, err := ReadDeferredFloors(companionPath); err == nil && ok {
		return filterDeclaredPackages(declared, candidatePkgs)
	}
	return DeferredFloorPackages(artifact, candidatePkgs)
}

// DeferredFloorDivergence cross-checks prose deferred package mentions against
// deferred_floors. It returns an actionable correction string, not a reject.
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
