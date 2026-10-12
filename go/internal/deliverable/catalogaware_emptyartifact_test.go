package deliverable

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func TestVerifyCatalogAwareStage_AnEmptyArtifactFailsForEachCorePhase(t *testing.T) {
	for _, phase := range []string{"scout", "triage", "build", "tdd", "audit", "intent"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			ws := t.TempDir()
			evolveDir := filepath.Join(t.TempDir(), ".evolve")
			if err := os.MkdirAll(evolveDir, 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			roots := phasecontract.Roots{Workspace: ws, EvolveDir: evolveDir}
			contract, ok := phasecontract.BuiltinResolver{}.Resolve(phase)
			if !ok {
				t.Fatalf("no builtin contract for %q", phase)
			}
			writeFile(t, ws, filepath.Base(contract.ArtifactPath(roots)), "  \n\t\n")
			res, err := VerifyCatalogAwareStage(phase, roots, config.StageOff)
			if err != nil {
				t.Fatalf("VerifyCatalogAwareStage: %v", err)
			}
			if res.OK || !hasCode(res, CodeEmptyArtifact) {
				t.Errorf("want empty_artifact for %s, got %+v", phase, res.Violations)
			}
		})
	}
}
