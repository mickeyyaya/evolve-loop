package regressiontia

import (
	"os"
	"path/filepath"
	"testing"
)

func writeModuleFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func stdlibOnlyThroughTestingModule(t *testing.T) (repoRoot, moduleDir string) {
	t.Helper()
	repoRoot = t.TempDir()
	moduleDir = filepath.Join(repoRoot, "go")
	writeModuleFile(t, moduleDir, "go.mod", "module example.com/m\n\ngo 1.21\n")
	writeModuleFile(t, moduleDir, "internal/leaf/leaf.go", "package leaf\n\nfunc Value() int { return 1 }\n")
	writeModuleFile(t, moduleDir, "internal/other/other.go", "package other\n\nfunc Value() int { return 2 }\n")
	writeModuleFile(t, moduleDir, "acs/regression/pure/pure_test.go",
		"//go:build acs\n\npackage pure\n\nimport (\n\t\"testing\"\n\n\t\"example.com/m/internal/leaf\"\n)\n\nfunc TestPure(t *testing.T) {\n\tif leaf.Value() != 1 {\n\t\tt.Fatal(\"leaf\")\n\t}\n}\n")
	writeModuleFile(t, moduleDir, "acs/regression/hatch/hatch_test.go",
		"//go:build acs\n\npackage hatch\n\nimport (\n\t\"os\"\n\t\"testing\"\n\n\t\"example.com/m/internal/leaf\"\n)\n\nfunc TestHatch(t *testing.T) {\n\tif leaf.Value() != 1 || os.Getpagesize() == 0 {\n\t\tt.Fatal(\"leaf\")\n\t}\n}\n")
	return repoRoot, moduleDir
}

func TestCompute_SkipsAPredicateThatReachesOsOnlyThroughTesting(t *testing.T) {
	repoRoot, moduleDir := stdlibOnlyThroughTestingModule(t)

	d := Compute("shadow", repoRoot, moduleDir, []string{"./internal/other/..."})

	if len(d.ChangedPackages) == 0 {
		t.Fatalf("ChangedPackages is empty, so Select keeps every predicate before the dependency walk runs")
	}
	if !containsStr(d.WouldSkip, "./acs/regression/pure") {
		t.Errorf("WouldSkip = %v, Selected = %v: ./acs/regression/pure imports only testing and ./internal/leaf, yet the walk follows the stdlib closure of testing to os and trips the escape hatch", d.WouldSkip, d.Selected)
	}
	if !containsStr(d.Selected, "./acs/regression/hatch") {
		t.Errorf("Selected = %v: ./acs/regression/hatch imports os directly and must stay selected", d.Selected)
	}
}
