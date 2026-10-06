package evalgate

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
)

func unsatisfiableShapeGate() predicateLintGate {
	return predicateLintGate{
		gateName: "unsatisfiable-predicate-shape",
		label:    "unsatisfiable-lint",
		subject:  "predicate",
		headline: "unsatisfiable predicate(s)",
		advice:   "A predicate that is red on every tree burns the build (the cycle-1488 class); rewrite it before build dispatch.",
		lint:     lintUnsatisfiableShapes,
	}
}

var unsatisfiableProofKinds = map[string]bool{
	evalqualitycheck.UnsatisfiableKindInvertedIdiom: true,
	evalqualitycheck.UnsatisfiableKindGoRunExitCode: true,
}

func lintUnsatisfiableShapes(dir string) (predicateLintOutcome, error) {
	report, err := evalqualitycheck.LintUnsatisfiablePredicates(dir)
	findings := make([]predicateLintFinding, 0, len(report.Findings))
	for _, f := range report.Findings {
		findings = append(findings, predicateLintFinding{
			text:     fmt.Sprintf("%s:%s [%s] %s", f.File, f.Func, f.Kind, f.Reason),
			blocking: unsatisfiableProofKinds[f.Kind],
		})
	}
	return predicateLintOutcome{files: report.Files, findings: findings}, err
}
