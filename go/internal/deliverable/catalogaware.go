package deliverable

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolveloop/go/internal/config"
	"github.com/mickeyyaya/evolveloop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolveloop/go/internal/phasespec"
)

// catalogaware.go — VerifyCatalogAware: VerifyWith behind the SAME merged-
// catalog resolution the host gate (NewReviewerWithCatalog), the salvage rung
// (NewVerifierWithCatalog), and the agent self-check (`evolve phase verify`)
// use, derived lazily from the call-time roots. It exists for callers that
// have roots but no pre-loaded catalog — the runner's reconcile-on-timeout
// default — so no consumer is left resolving phases under a second,
// builtin-only policy.

// VerifyCatalogAware runs the well-formedness checks resolving the phase's
// contract through the project's merged catalog (built-in registry +
// .evolve/phases user specs), locating the project from roots.EvolveDir.
// PRECONDITION: roots.EvolveDir must be <projectRoot>/.evolve (the shape
// every production constructor builds — paths.go, verifier.go rootsFor, the
// runner reconcile site); any other shape silently resolves the wrong
// project and degrades. Missing EvolveDir or an unloadable catalog degrades
// to built-in-only resolution WITH a stderr WARN — built-in phases always
// verify, a catalog glitch never hard-fails the check, but a degrade on the
// reconcile path can flip a user phase's outcome and must be visible.
//
// PhaseIO (ADR-0050 §3.10): as of Slice 1 this reconcile-on-timeout rung is
// stage-aware via VerifyCatalogAwareStage, so it honors the SAME stage-gated
// failure-context requirement as the host gate (NewReviewerWithCatalogStage) and
// the salvage rung (NewVerifierWithCatalogStage). VerifyCatalogAware is retained
// as the byte-identical back-compat wrapper (StageOff) for callers that pass no
// stage.
func VerifyCatalogAware(phase string, roots phasecontract.Roots) (Result, error) {
	return VerifyCatalogAwareStage(phase, roots, config.StageOff)
}

// VerifyCatalogAwareStage is VerifyCatalogAware threaded with the EVOLVE_PHASE_IO
// rollout stage (ADR-0050 §3.10). The stage flows through to VerifyWithStage at
// every resolution branch (catalog-resolved, builtin fallback, and the degraded
// catalog-load path); at StageOff it is byte-identical to the pre-3.10 path, and
// at >=StageEnforce a build/scout/triage FAIL-without-block artifact reconciled on
// a timeout race is now caught here too (the reconcile rung reaches the same
// verdict as the host gate, not just deferring to it).
func VerifyCatalogAwareStage(phase string, roots phasecontract.Roots, phaseIO config.Stage) (Result, error) {
	if roots.EvolveDir == "" {
		return VerifyWithStage(phase, roots, phasecontract.BuiltinResolver{}, phaseIO)
	}
	cat, _, _, err := phasespec.MergedCatalog(filepath.Dir(roots.EvolveDir))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[deliverable] WARN catalog load failed (%v) — contract resolution degraded to built-in-only; user/minted phases will not resolve\n", err)
		return VerifyWithStage(phase, roots, phasecontract.BuiltinResolver{}, phaseIO)
	}
	return VerifyWithStage(phase, roots, phasecontract.NewCatalogResolver(cat.Get), phaseIO)
}
