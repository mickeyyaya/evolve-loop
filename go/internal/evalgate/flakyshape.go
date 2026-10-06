package evalgate

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
)

func flakyShapeGate() predicateLintGate {
	return predicateLintGate{
		gateName: "flaky-predicate-shape",
		label:    "flaky-shape lint",
		subject:  "predicate shape",
		headline: "flaky-shaped predicate(s)",
		advice:   "These shapes flake under fleet load (Luo FSE'14 async-wait/concurrency classes); rewrite before they enter the ACS corpus.",
		lint:     lintFlakyShapes,
	}
}

func lintFlakyShapes(dir string) (predicateLintOutcome, error) {
	report, err := evalqualitycheck.LintFlakyPredicates(dir)
	findings := make([]predicateLintFinding, 0, len(report.Findings))
	for _, f := range report.Findings {
		findings = append(findings, predicateLintFinding{text: fmt.Sprintf("%s:%s [%s] %s", f.File, f.Func, f.Class, f.Reason)})
	}
	return predicateLintOutcome{files: report.Files, findings: findings}, err
}
