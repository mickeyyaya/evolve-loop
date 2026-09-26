package deliverable

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// VerifyCatalogAware verifies through the project's merged catalog, located from roots.EvolveDir (<projectRoot>/.evolve).
func VerifyCatalogAware(phase string, roots phasecontract.Roots) (Result, error) {
	return VerifyCatalogAwareStage(phase, roots, config.StageOff)
}

// VerifyCatalogAwareStage is VerifyCatalogAware with the EVOLVE_PHASE_IO stage, so the reconcile rung reaches the gate's verdict.
func VerifyCatalogAwareStage(phase string, roots phasecontract.Roots, phaseIO config.Stage) (Result, error) {
	if roots.EvolveDir == "" {
		return VerifyWithStage(phase, roots, phasecontract.BuiltinResolver{}, phaseIO)
	}
	cat, _, _, err := phasespec.MergedCatalog(filepath.Dir(roots.EvolveDir))
	if err != nil {
		// Degrade loudly: a catalog glitch must not fail the check, but it can flip a user phase's outcome.
		fmt.Fprintf(os.Stderr, "[deliverable] WARN catalog load failed (%v) — contract resolution degraded to built-in-only; user/minted phases will not resolve\n", err)
		return VerifyWithStage(phase, roots, phasecontract.BuiltinResolver{}, phaseIO)
	}
	return VerifyWithStage(phase, roots, phasecontract.NewCatalogResolver(cat.Get), phaseIO)
}
