package deliverable

import (
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/codereview"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualityindex"
)

type grammarCheck func(content string, roots phasecontract.Roots) ([]string, error)

var grammars = map[string]grammarCheck{
	phasespec.GrammarCodeReviewReport: func(content string, roots phasecontract.Roots) ([]string, error) {
		thresholds, err := thresholdsFor(roots)
		if err != nil {
			return nil, err
		}
		return codereview.ValidateReport(content, thresholds), nil
	},
}

func verifyGrammars(res *Result, c phasecontract.Contract, content string, roots phasecontract.Roots) error {
	for _, name := range c.Grammars {
		check, bound := grammars[name]
		if !bound {
			res.add(CodeUnboundGrammar, fmt.Sprintf("declared grammar %q has no registered check — a phase-registry defect, not something this agent can correct", name))
			continue
		}
		problems, err := check(content, roots)
		if err != nil {
			return fmt.Errorf("deliverable: the %s grammar cannot be checked: %w", name, err)
		}
		for _, problem := range problems {
			res.add(CodeBadGrammar, fmt.Sprintf("the report breaks the %s grammar: %s", name, problem))
		}
	}
	return nil
}

func thresholdsFor(roots phasecontract.Roots) (qualityindex.Thresholds, error) {
	if roots.EvolveDir == "" {
		t, _ := qualityindex.ResolveThresholds(nil)
		return t, nil
	}
	return policy.QualityIndexThresholdsFor(filepath.Dir(roots.EvolveDir))
}
