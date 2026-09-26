package deliverable

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// stageBuildWorkspace writes a build-report.md with the given body.
func stageBuildWorkspace(t *testing.T, body string) string {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return ws
}

func TestVerifyBuildWithChangedPathsStage_AppliesFloorAndPreservesStage(t *testing.T) {
	roots := phasecontract.Roots{Workspace: stageBuildWorkspace(t, archFloorBuildReport)}
	archClass := []string{"go/internal/policy/policy.go"}

	t.Run("floor fires on an undocumented architecture-class diff", func(t *testing.T) {
		res, err := VerifyBuildWithChangedPathsStage(roots, archClass, phasecontract.BuiltinResolver{}, config.StageEnforce)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.OK {
			t.Fatalf("want !OK — the diff is architecture-class with no docs delta; got %+v", res)
		}
		if !hasCode(res, CodeMissingArchitectureDocs) {
			t.Errorf("violations = %+v, want %s", res.Violations, CodeMissingArchitectureDocs)
		}
	})

	t.Run("a docs delta satisfies the floor", func(t *testing.T) {
		res, err := VerifyBuildWithChangedPathsStage(roots, append(archClass, adrPath), phasecontract.BuiltinResolver{}, config.StageEnforce)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.OK {
			t.Errorf("want OK — the ADR documents the change; got %+v", res.Violations)
		}
	})

	t.Run("well-formedness violations are additive, never replaced", func(t *testing.T) {
		res, err := VerifyBuildWithChangedPathsStage(
			phasecontract.Roots{Workspace: t.TempDir()}, archClass, phasecontract.BuiltinResolver{}, config.StageOff)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !hasCode(res, CodeMissingArtifact) {
			t.Errorf("a missing build-report must still report %s; got %+v", CodeMissingArtifact, res.Violations)
		}
	})

	t.Run("defaulted form equals the built-in/StageOff pinning", func(t *testing.T) {
		want, err := VerifyBuildWithChangedPathsStage(roots, archClass, phasecontract.BuiltinResolver{}, config.StageOff)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, err := VerifyBuildWithChangedPaths(roots, archClass)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.OK != want.OK || len(got.Violations) != len(want.Violations) {
			t.Errorf("VerifyBuildWithChangedPaths = %+v, want %+v", got, want)
		}
	})
}
