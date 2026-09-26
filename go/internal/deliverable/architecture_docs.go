package deliverable

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/docsfloor"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// CodeMissingArchitectureDocs: an architecture-class build diff carries no documentation delta.
const CodeMissingArchitectureDocs = "missing_architecture_docs"

// ArchitectureDocsViolations returns one violation iff changed is architecture-class with no docs delta; docsfloor owns the classification.
func ArchitectureDocsViolations(changed []string) []Violation {
	if !docsfloor.IsArchitectureClass(changed) || docsfloor.HasDocsDelta(changed) {
		return nil
	}
	return []Violation{{
		Code: CodeMissingArchitectureDocs,
		// The message is the correction directive, so it names where the doc belongs.
		Message: fmt.Sprintf(
			"this change is architecture-class but touches no documentation — record the decision under %s (an ADR, control-flags.md) or in %s",
			strings.TrimSuffix(docsfloor.DocsRoots[0], "/"), docsfloor.DocsRoots[1]),
	}}
}

// VerifyBuildWithChangedPaths is Verify("build") plus the docs floor over changed, additive to the well-formedness checks.
func VerifyBuildWithChangedPaths(roots phasecontract.Roots, changed []string) (Result, error) {
	return VerifyBuildWithChangedPathsStage(roots, changed, phasecontract.BuiltinResolver{}, config.StageOff)
}

// VerifyBuildWithChangedPathsStage threads the caller's resolver and stage, so adding the floor never weakens the build contract.
func VerifyBuildWithChangedPathsStage(roots phasecontract.Roots, changed []string, resolver phasecontract.Resolver, phaseIO config.Stage) (Result, error) {
	res, err := VerifyWithStage("build", roots, resolver, phaseIO)
	if err != nil {
		return Result{}, err
	}
	res.Violations = append(res.Violations, ArchitectureDocsViolations(changed)...)
	res.finish()
	return res, nil
}
