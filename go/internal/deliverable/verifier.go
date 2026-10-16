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

type Verifier struct {
	resolver phasecontract.Resolver
	phaseIO  config.Stage
}

func NewVerifier() core.ContractVerifier {
	return &Verifier{resolver: phasecontract.BuiltinResolver{}}
}

func NewVerifierWithCatalog(cat phasespec.Catalog) core.ContractVerifier {
	return &Verifier{resolver: phasecontract.NewCatalogResolver(cat.Get)}
}

func NewVerifierWithCatalogStage(cat phasespec.Catalog, phaseIO config.Stage) core.ContractVerifier {
	return &Verifier{resolver: phasecontract.NewCatalogResolver(cat.Get), phaseIO: phaseIO}
}

func RootsFor(in core.ReviewInput) phasecontract.Roots {
	return phasecontract.Roots{
		Workspace:                       in.Workspace,
		Worktree:                        in.Worktree,
		EvolveDir:                       paths.EvolveDirOf(in.ProjectRoot),
		ExplanationDocumentationVersion: in.ExplanationDocumentationVersion,
		Cycle:                           in.Cycle,
	}
}

func (v *Verifier) VerifyDeliverable(_ context.Context, in core.ReviewInput) (core.ContractVerification, error) {
	res, err := VerifyWithStage(in.Phase, RootsFor(in), v.resolver, v.phaseIO)
	if err != nil {
		return core.ContractVerification{}, err
	}
	out := core.ContractVerification{OK: res.OK, ArtifactPath: res.ArtifactPath}
	for _, vi := range res.Violations {
		out.Violations = append(out.Violations, fmt.Sprintf("[%s] %s", vi.Code, vi.Message))
	}
	return out, nil
}
