package phasecmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func specPhaseProject(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "architecture", "phase-registry.json"))
	if err != nil {
		t.Fatalf("read phase registry: %v", err)
	}
	root := t.TempDir()
	regDir := filepath.Join(root, "docs", "architecture")
	if err := os.MkdirAll(regDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(regDir, "phase-registry.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestIsKnownPhase(t *testing.T) {
	root := specPhaseProject(t)
	cases := []struct {
		name string
		want bool
	}{
		{"tdd", true},
		{"TDD", true},
		{"plan-review", true},
		{"spec-verify", true},
		{"memo", false},
		{"secret-leak-scan", false},
		{"no-such-phase", false},
	}
	for _, c := range cases {
		if got := IsKnownPhase(c.name, root); got != c.want {
			t.Errorf("IsKnownPhase(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCatalogPhaseNamesExcludesNativeAndControlSpecs(t *testing.T) {
	_, specs, err := CatalogPhaseNames(specPhaseProject(t))
	if err != nil {
		t.Fatalf("CatalogPhaseNames: %v", err)
	}
	for _, unrunnable := range []string{"ship", "memo", "retrospective", "secret-leak-scan", "flake-rerun-scan"} {
		for _, s := range specs {
			if s == unrunnable {
				t.Errorf("specs %v list unrunnable phase %q", specs, unrunnable)
			}
		}
	}
}

func TestCatalogPhaseNamesReportsCatalogLoadFailure(t *testing.T) {
	builtins, specs, err := CatalogPhaseNames(t.TempDir())
	if err == nil {
		t.Fatal("want an error for a project without a phase registry")
	}
	if len(builtins) == 0 || len(specs) != 0 {
		t.Errorf("builtins=%v specs=%v, want builtins kept and no specs", builtins, specs)
	}
	msg := FormatUnknownPhaseError("evolve phase", "x", t.TempDir())
	if !strings.Contains(msg, "spec catalog unavailable") || !strings.Contains(msg, "known: ") {
		t.Errorf("message %q hides the catalog failure", msg)
	}
}

func TestResolveRunnerSkipsUnrunnableSpec(t *testing.T) {
	runner, found, err := ResolveRunner("memo", core.PhaseRequest{ProjectRoot: specPhaseProject(t)})
	if err != nil || found || runner != nil {
		t.Errorf("ResolveRunner(memo) = (%v, %v, %v), want (nil, false, nil)", runner, found, err)
	}
}
