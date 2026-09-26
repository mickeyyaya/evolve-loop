package phasespec

import (
	"os"
	"path/filepath"
	"testing"
)

// seedProject writes a project with one built-in registry phase and one user phase overlay.
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

func TestMergedCatalog_MissingRegistryErrors(t *testing.T) {
	if _, _, _, err := MergedCatalog(t.TempDir()); err == nil {
		t.Error("MergedCatalog(no registry) = nil error, want error")
	}
}
