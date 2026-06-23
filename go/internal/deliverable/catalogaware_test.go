package deliverable

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/phasecontract"
)

// catalogaware_test.go — RED contract for VerifyCatalogAware: ONE verify
// entry point that derives the merged phase catalog (built-in registry +
// .evolve/phases user specs) from the call-time roots, so every consumer —
// host gate, agent self-check, salvage rung, and the runner's
// timeout-reconcile — resolves contracts under the SAME policy. The
// reconcile path previously used Verify (BuiltinResolver only): a user
// phase whose artifact survived a bridge timeout could not be reconciled
// and synthesized FAIL.

// seedCatalogProject builds a project with a minimal built-in registry and
// one user phase, returning (projectRoot, evolveDir, workspace).
func seedCatalogProject(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	regDir := filepath.Join(root, "docs", "architecture")
	if err := os.MkdirAll(regDir, 0o755); err != nil {
		t.Fatal(err)
	}
	registry := `{"phases":[{"name":"build","archetype":"build","agent":"evolve-builder",
		"outputs":{"files":[".evolve/runs/cycle-{cycle}/build-report.md"]}}]}`
	if err := os.WriteFile(filepath.Join(regDir, "phase-registry.json"), []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}
	userDir := filepath.Join(root, ".evolve", "phases", "widget-scan")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	spec := `{"name":"widget-scan","archetype":"evaluate","agent":"evolve-widget-scan",
		"outputs":{"files":[".evolve/runs/cycle-{cycle}/widget-scan-report.md"]}}`
	if err := os.WriteFile(filepath.Join(userDir, "phase.json"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := filepath.Join(root, ".evolve", "runs", "cycle-7")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, filepath.Join(root, ".evolve"), ws
}

// TestVerifyCatalogAware_ResolvesUserPhase — a user phase must RESOLVE (the
// missing artifact is then a normal violation, not a resolution error),
// exactly as the host gate and `evolve phase verify` see it. Verify
// (builtin-only) cannot resolve it — pinned here as the contrast that keeps
// this test honest about what catalog-awareness adds.
func TestVerifyCatalogAware_ResolvesUserPhase(t *testing.T) {
	_, evolveDir, ws := seedCatalogProject(t)
	roots := phasecontract.Roots{Workspace: ws, EvolveDir: evolveDir}

	if _, err := Verify("widget-scan", roots); err == nil {
		t.Fatal("precondition: builtin-only Verify must NOT resolve a user phase")
	}

	res, err := VerifyCatalogAware("widget-scan", roots)
	if err != nil {
		t.Fatalf("VerifyCatalogAware must resolve the user phase via the catalog; got %v", err)
	}
	if res.OK {
		t.Error("artifact is absent — expected violations, got OK")
	}
}

// TestVerifyCatalogAware_WellFormedUserArtifact — with the user phase's
// artifact on disk, the same call returns OK: the reconcile-on-timeout
// upgrade path works for user phases.
func TestVerifyCatalogAware_WellFormedUserArtifact(t *testing.T) {
	_, evolveDir, ws := seedCatalogProject(t)
	report := "# widget-scan\n\n## Verdict\nPASS\n"
	if err := os.WriteFile(filepath.Join(ws, "widget-scan-report.md"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := VerifyCatalogAware("widget-scan", phasecontract.Roots{Workspace: ws, EvolveDir: evolveDir})
	if err != nil {
		t.Fatalf("VerifyCatalogAware: %v", err)
	}
	if !res.OK {
		t.Errorf("well-formed user-phase artifact must verify OK; violations=%+v", res.Violations)
	}
}

// TestVerifyCatalogAware_DegradesToBuiltin — no EvolveDir in roots (or no
// registry on disk) degrades to built-in-only resolution, mirroring the
// CLI's phaseVerifyResolver: built-in phases always verify, never hard-fail
// on a catalog glitch.
func TestVerifyCatalogAware_DegradesToBuiltin(t *testing.T) {
	ws := t.TempDir()
	res, err := VerifyCatalogAware("build", phasecontract.Roots{Workspace: ws})
	if err != nil {
		t.Fatalf("built-in phase must still resolve without a catalog: %v", err)
	}
	if res.OK {
		t.Error("no artifact on disk — expected violations")
	}
}
