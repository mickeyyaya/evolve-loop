package triagecap

import (
	"regexp"
	"sort"
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
