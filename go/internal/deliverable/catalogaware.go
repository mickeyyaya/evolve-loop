package deliverable

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
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
func VerifyCatalogAware(phase string, roots phasecontract.Roots) (Result, error) {
	if roots.EvolveDir == "" {
		return VerifyWith(phase, roots, phasecontract.BuiltinResolver{})
	}
	cat, _, _, err := phasespec.MergedCatalog(filepath.Dir(roots.EvolveDir))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[deliverable] WARN catalog load failed (%v) — contract resolution degraded to built-in-only; user/minted phases will not resolve\n", err)
		return VerifyWith(phase, roots, phasecontract.BuiltinResolver{})
	}
	return VerifyWith(phase, roots, phasecontract.NewCatalogResolver(cat.Get))
}
