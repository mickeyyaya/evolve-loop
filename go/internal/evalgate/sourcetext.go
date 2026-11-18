package evalgate

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
)

func sourceTextShapeGate() predicateLintGate {
	return predicateLintGate{
		gateName: "source-text-predicate-shape",
		label:    "source-text lint",
		subject:  "predicate",
		headline: "source-text predicate(s)",
		advice:   "A predicate that only greps source text proves nothing about behaviour; rewrite it to run the code before build dispatch.",
		lint:     lintSourceTextShapes,
	}
}

func lintSourceTextShapes(dir string) (predicateLintOutcome, error) {
	report, err := evalqualitycheck.LintSourceTextPredicates(dir)
	findings := make([]predicateLintFinding, 0, len(report.Findings))
	for _, f := range report.Findings {
		findings = append(findings, predicateLintFinding{text: fmt.Sprintf("%s:%s %s", f.File, f.Func, f.Reason), blocking: true})
	}
	return predicateLintOutcome{files: report.Files, findings: findings}, err
}
