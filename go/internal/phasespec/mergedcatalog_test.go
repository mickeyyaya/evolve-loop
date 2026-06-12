package phasespec

import (
	"os"
	"path/filepath"
	"testing"
)

// mergedcatalog_test.go — RED contract for moving the merged-catalog loader
// out of cmd/evolve into phasespec, so cmd, the agent self-check, AND the
// runner's reconcile default all resolve phases through ONE loader (the
// timeout-reconcile path resolved via BuiltinResolver only, a second policy).

// seedProject writes a minimal project: a built-in registry with one phase
// and a user phase under .evolve/phases/<name>/phase.json.
func seedProject(t *testing.T) string {
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
	return root
}

// TestMergedCatalog_ResolvesBuiltinAndUserPhases — the single-loader contract:
// built-in registry phases AND .evolve/phases user specs resolve through one
// call, with user provenance reported.
func TestMergedCatalog_ResolvesBuiltinAndUserPhases(t *testing.T) {
	root := seedProject(t)

	cat, sources, warns, err := MergedCatalog(root)
	if err != nil {
		t.Fatalf("MergedCatalog: %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("warns = %v, want none", warns)
	}
	if _, ok := cat.Get("build"); !ok {
		t.Error("built-in phase 'build' must resolve")
	}
	if _, ok := cat.Get("widget-scan"); !ok {
		t.Error("user phase 'widget-scan' must resolve through the merged catalog")
	}
	if !cat.IsUser("widget-scan") {
		t.Error("widget-scan must be flagged as a user phase")
	}
	if src := sources["widget-scan"]; src == "" {
		t.Errorf("sources[widget-scan] = %q, want its discovery root", src)
	}
}

// TestMergedCatalog_MissingRegistryErrors — a project without the built-in
// registry must error loudly (the caller decides how to degrade).
func TestMergedCatalog_MissingRegistryErrors(t *testing.T) {
	if _, _, _, err := MergedCatalog(t.TempDir()); err == nil {
		t.Error("MergedCatalog(no registry) = nil error, want error")
	}
}
