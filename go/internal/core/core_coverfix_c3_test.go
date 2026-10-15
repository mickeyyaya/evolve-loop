package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoverfixC3_DistinguisherReadsTheRegistryArtifactNameBeforeTheConventionalOne(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "test-report.md"), []byte("# TDD\n\n- D1 HIGH: registry-named defect\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "tdd-report.md"), []byte("- D1 HIGH: conventional-named defect\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := verdictFailDistinguisher("tdd", ws)
	if !strings.Contains(got, "registry-named defect") || strings.Contains(got, "conventional-named defect") {
		t.Fatalf("distinguisher = %q, want the registry-named report to win", got)
	}
}

func TestCoverfixC3_DistinguisherSkipsAMissingRegistryReportForTheConventionalOne(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "tdd-report.md"), []byte("- D2 LOW: fallback-file defect\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := verdictFailDistinguisher("tdd", ws); !strings.Contains(got, "fallback-file defect") {
		t.Fatalf("distinguisher = %q, want the conventional report consulted when the registry one is absent", got)
	}
}

func TestCoverfixC3_ReadableReportWithoutBulletsSettlesTheLayerAndLeavesNoDistinguisher(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "tdd-report.md"), []byte("- D9 CRITICAL: must not be reached\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "test-report.md"), []byte("# TDD\n\nno bullets here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := verdictFailDistinguisher("tdd", ws); got != "" {
		t.Fatalf("distinguisher = %q, want empty: a readable bulletless report settles the layer", got)
	}
}

func TestCoverfixC3_UnreadableMintSpecIsNotVerified(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "phases", "locked-mint")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	spec := filepath.Join(dir, "phase.json")
	if err := os.WriteFile(spec, []byte(`{"name":"locked-mint","optional":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(spec, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(spec); err == nil {
		t.Skip("file stays readable (running as root); the unreadable branch cannot be set up")
	}
	if got := verifiedActiveMints(root, map[string]bool{"locked-mint": true}); len(got) != 0 {
		t.Fatalf("verifiedActiveMints = %v, want an unreadable spec rejected", got)
	}
}
