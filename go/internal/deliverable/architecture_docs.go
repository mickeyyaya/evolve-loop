package deliverable

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/docsfloor"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// CodeMissingArchitectureDocs: the build's diff is architecture-class (a policy
// or config vocabulary surface, a new internal package, a phase spec, or a
// trust-kernel source file) and carries no documentation delta. Closed
// vocabulary, snake_case like its siblings above; consumed by the CLI, the
// reviewer gate and the auditor checklist. Design: ADR-0077.
const CodeMissingArchitectureDocs = "missing_architecture_docs"

// ArchitectureDocsViolations is the pure classifier over a build diff's
// changed-path set: one CodeMissingArchitectureDocs violation iff the set is
// architecture-class AND carries no docs delta, nil otherwise. Deterministic —
// no LLM judgement, no git shell-out, no filesystem access — so the agent's
// `evolve phase verify` self-check and the host-side gate reach the same verdict
// from the same input (the ADR-0034 no-drift invariant).
//
// The classification itself lives in internal/docsfloor, the single source of
// truth shared with the WARN-level build-handoff floor: this package owns the
// violation vocabulary, that one owns what "architecture" means.
//
// Fail-open: a diff that is not architecture-class never yields the violation,
// so bugfix, test-only and docs-only cycles are byte-identical to before.
func ArchitectureDocsViolations(changed []string) []Violation {
	if !docsfloor.IsArchitectureClass(changed) || docsfloor.HasDocsDelta(changed) {
		return nil
	}
	return []Violation{{
		Code: CodeMissingArchitectureDocs,
		// The message is re-dispatched verbatim to the builder as the correction
		// directive, so it must name WHERE the doc belongs — a bare "missing
		// docs" is not actionable.
		Message: fmt.Sprintf(
			"this change is architecture-class but touches no documentation — record the decision under %s (an ADR, control-flags.md) or in %s",
			strings.TrimSuffix(docsfloor.DocsRoots[0], "/"), docsfloor.DocsRoots[1]),
	}}
}

// VerifyBuildWithChangedPaths is Verify("build", roots) with the documentation
// floor applied to the build's changed-path set: the well-formedness violations
// and the docs-floor violations in one Result, OK recomputed over both.
//
// ADDITIVE by construction — the floor never replaces or masks a well-formedness
// check, so a missing build-report still reports CodeMissingArtifact alongside
// it. Callers with no diff source keep using Verify, which has nothing to
// inspect and therefore never emits the floor violation (the fail-open half of
// the ADR-0034 contract).
//
// Return contract is Verify's: err != nil ⇒ ambiguity, fail OPEN; err == nil
// with !OK ⇒ confirmed violation, fail CLOSED.
func VerifyBuildWithChangedPaths(roots phasecontract.Roots, changed []string) (Result, error) {
	return VerifyBuildWithChangedPathsStage(roots, changed, phasecontract.BuiltinResolver{}, config.StageOff)
}

// VerifyBuildWithChangedPathsStage is VerifyBuildWithChangedPaths threaded with
// the caller's resolver and EVOLVE_PHASE_IO rollout stage — the same two dials
// VerifyWithStage takes. The CLI self-check resolves through the merged phase
// catalog and the configured stage; dropping to the built-in resolver at
// StageOff to reach the docs floor would silently weaken the build contract it
// already enforces (the stage-gated failure-context check). Additive by
// construction: the floor is appended to the well-formedness violations, never
// substituted for them.
//
// VerifyBuildWithChangedPathsStage(roots, changed, BuiltinResolver{}, StageOff)
// == VerifyBuildWithChangedPaths, so the defaulted form stays byte-identical.
func VerifyBuildWithChangedPathsStage(roots phasecontract.Roots, changed []string, resolver phasecontract.Resolver, phaseIO config.Stage) (Result, error) {
	res, err := VerifyWithStage("build", roots, resolver, phaseIO)
	if err != nil {
		return Result{}, err
	}
	res.Violations = append(res.Violations, ArchitectureDocsViolations(changed)...)
	res.finish()
	return res, nil
}
