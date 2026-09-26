package deliverable

import (
	"context"
	"fmt"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// Verifier re-checks without touching the breaker, so a multi-rung repair never counts as consecutive blocks.
// See ADR-0045.
type Verifier struct {
	resolver phasecontract.Resolver
	phaseIO  config.Stage // EVOLVE_PHASE_IO stage
}

// NewVerifier resolves built-in contracts only.
func NewVerifier() core.ContractVerifier {
	return &Verifier{resolver: phasecontract.BuiltinResolver{}}
}

// NewVerifierWithCatalog resolves like the catalog-aware Reviewer, so the rung and the gate agree on well-formed.
func NewVerifierWithCatalog(cat phasespec.Catalog) core.ContractVerifier {
	return &Verifier{resolver: phasecontract.NewCatalogResolver(cat.Get)}
}

// NewVerifierWithCatalogStage is NewVerifierWithCatalog with the EVOLVE_PHASE_IO stage the gate applies.
func NewVerifierWithCatalogStage(cat phasespec.Catalog, phaseIO config.Stage) core.ContractVerifier {
	return &Verifier{resolver: phasecontract.NewCatalogResolver(cat.Get), phaseIO: phaseIO}
}

// rootsFor is the one ReviewInput-to-Roots translation, shared by the gate, the rung re-check and HostEffects.
func rootsFor(in core.ReviewInput) phasecontract.Roots {
	return phasecontract.Roots{
		Workspace: in.Workspace,
		Worktree:  in.Worktree,
		// Spelled as the engine's rootsFor (verdict/settle.go); only a dispatch knows DispatchedArtifact.
		EvolveDir:                       paths.EvolveDirOf(in.ProjectRoot),
		ExplanationDocumentationVersion: in.ExplanationDocumentationVersion,
		Cycle:                           in.Cycle,
	}
}

// VerifyDeliverable implements core.ContractVerifier without the breaker; an error keeps Verify's fail-open contract.
func (v *Verifier) VerifyDeliverable(_ context.Context, in core.ReviewInput) (core.ContractVerification, error) {
	res, err := VerifyWithStage(in.Phase, rootsFor(in), v.resolver, v.phaseIO)
	if err != nil {
		return core.ContractVerification{}, err
	}
	out := core.ContractVerification{OK: res.OK, ArtifactPath: res.ArtifactPath}
	for _, vi := range res.Violations {
		out.Violations = append(out.Violations, fmt.Sprintf("[%s] %s", vi.Code, vi.Message))
	}
	return out, nil
}
