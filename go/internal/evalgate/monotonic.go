package evalgate

import (
	"fmt"
	"regexp"
	"strings"
)

// monotonicClass is the declared inbox class whose work is convergent —
// progress accumulates across cycles rather than completing in one.
const monotonicClass = "task-contract-design"

// binaryAbsoluteTargetRe matches an acceptance criterion that states where a
// count must END UP ("to <=25", "to at most 25", "to under 25") rather than how
// far it must MOVE. On a monotonic task that phrasing is all-or-nothing: a
// cycle that verifiably pruned 101 of 140 items scored zero and the work was
// discarded (cycle-992). Deliberately prose-tolerant — the defect is the
// phrasing pattern, not one operator spelling.
var binaryAbsoluteTargetRe = regexp.MustCompile(`(?i)\bto\s+(?:<=|≤|=<|at most|no more than|fewer than|less than|under|below)\s*\d+`)

// LintMonotonicBinaryTarget flags binary absolute-count acceptance criteria on
// monotonic-classed tasks, returning one finding per offending criterion (nil
// when nothing fires). Each finding names the direction+floor remedy: a lint
// that flags without saying what to write instead does not prevent the failure
// mode it exists for.
//
// Non-monotonic classes are exempt — an absolute target is the CORRECT contract
// for one-shot work, so firing there would block legitimate criteria across the
// repo. False-positive cost dominates false-negative cost here, hence the
// narrow pattern.
func LintMonotonicBinaryTarget(class string, criteria []string) []string {
	if !isMonotonicClass(class) {
		return nil
	}
	var findings []string
	for i, c := range criteria {
		if !binaryAbsoluteTargetRe.MatchString(c) {
			continue
		}
		findings = append(findings, fmt.Sprintf(
			"monotonic class %q, acceptance criterion %d: %q states a binary absolute target — a cycle that makes real, verified progress but misses the number scores zero and its work is discarded (cycle-992). Rewrite as direction+floor, e.g. \"reduce X by >=N, landing the verified delta and requeueing the remainder\".",
			class, i+1, c))
	}
	return findings
}

// isMonotonicClass recognises the declared monotonic class plus any future
// class carrying the word in its name, so the lint survives the class
// vocabulary being extended without re-authoring.
func isMonotonicClass(class string) bool {
	return class == monotonicClass || strings.Contains(strings.ToLower(class), "monotonic")
}
