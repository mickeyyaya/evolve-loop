package deliverable

// verifier.go — the breaker-neutral core.ContractVerifier implementation
// (ADR-0045 I2 integrity rule). The correction ladder's intermediate rung
// re-checks (salvage's verify-after-move) run the SAME VerifyWith the gate
// runs, but never touch contract-gate-breaker.json — a multi-rung repair of
// one flaky deliverable must not count as three consecutive blocks and
// silently demote the gate batch-wide (cycle-265 forensics: two breakers,
// two scopes, do not conflate). Only the ladder's FINAL outcome goes through
// Reviewer.Review.

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// Verifier is the stateless breaker-neutral re-checker.
type Verifier struct {
	resolver phasecontract.Resolver
	phaseIO  config.Stage // EVOLVE_PHASE_IO rollout stage (ADR-0050 §3.8); default StageOff → byte-identical to pre-3.8.
}

// NewVerifier resolves built-in contracts only.
func NewVerifier() core.ContractVerifier {
	return &Verifier{resolver: phasecontract.BuiltinResolver{}}
}

// NewVerifierWithCatalog falls back to spec-derived contracts for user/minted
// phases — the same resolution the catalog-aware Reviewer uses, so the rung
// re-check and the gate can never disagree about what "well-formed" means.
// PhaseIO defaults to StageOff; production wires the dial via
// NewVerifierWithCatalogStage.
func NewVerifierWithCatalog(cat phasespec.Catalog) core.ContractVerifier {
	return &Verifier{resolver: phasecontract.NewCatalogResolver(cat.Get)}
}

// NewVerifierWithCatalogStage threads the EVOLVE_PHASE_IO rollout stage so the
// ladder re-check applies the same RequireFailureContextPhaseIO gating the host
// gate does — the rung re-check and the gate must never disagree about what
// "well-formed" means.
func NewVerifierWithCatalogStage(cat phasespec.Catalog, phaseIO config.Stage) core.ContractVerifier {
	return &Verifier{resolver: phasecontract.NewCatalogResolver(cat.Get), phaseIO: phaseIO}
}

// rootsFor maps a core.ReviewInput onto the contract roots — the ONE
// translation shared by the gate (Reviewer.Review) and the breaker-neutral
// re-check, so they can never resolve different paths for the same phase.
func rootsFor(in core.ReviewInput) phasecontract.Roots {
	return phasecontract.Roots{
		Workspace: in.Workspace,
		Worktree:  in.Worktree,
		EvolveDir: filepath.Join(in.ProjectRoot, ".evolve"),
	}
}

// VerifyDeliverable implements core.ContractVerifier: VerifyWithStage only, no
// breaker. It threads the verifier's PhaseIO stage so the rung re-check matches
// the host gate (default StageOff = the pre-3.8 VerifyWith). The error keeps
// deliverable.Verify's fail-open contract (ambiguity ⇒ error ⇒ the ladder skips
// the rung, never acts blind).
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
