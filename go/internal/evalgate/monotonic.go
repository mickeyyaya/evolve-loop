package evalgate

import (
	"fmt"
	"regexp"
	"strings"
)

const monotonicClass = "task-contract-design"

var binaryAbsoluteTargetRe = regexp.MustCompile(`(?i)\bto\s+(?:<=|≤|=<|at most|no more than|fewer than|less than|under|below)\s*\d+`)

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

func isMonotonicClass(class string) bool {
	return class == monotonicClass || strings.Contains(strings.ToLower(class), "monotonic")
}
