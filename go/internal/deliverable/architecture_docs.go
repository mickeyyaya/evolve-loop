package deliverable

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/docsfloor"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const CodeMissingArchitectureDocs = "missing_architecture_docs"

func ArchitectureDocsViolations(changed []string) []Violation {
	if !docsfloor.IsArchitectureClass(changed) || docsfloor.HasDocsDelta(changed) {
		return nil
	}
	return []Violation{{
		Code: CodeMissingArchitectureDocs,
		Message: fmt.Sprintf(
			"this change is architecture-class but touches no documentation — record the decision under %s (an ADR, control-flags.md) or in %s",
			strings.TrimSuffix(docsfloor.DocsRoots[0], "/"), docsfloor.DocsRoots[1]),
	}}
}

func VerifyBuildWithChangedPaths(roots phasecontract.Roots, changed []string) (Result, error) {
	return VerifyBuildWithChangedPathsStage(roots, changed, phasecontract.BuiltinResolver{}, config.StageOff)
}

func VerifyBuildWithChangedPathsStage(roots phasecontract.Roots, changed []string, resolver phasecontract.Resolver, phaseIO config.Stage) (Result, error) {
	res, err := VerifyWithStage("build", roots, resolver, phaseIO)
	if err != nil {
		return Result{}, err
	}
	res.Violations = append(res.Violations, ArchitectureDocsViolations(changed)...)
	res.finish()
	return res, nil
}
