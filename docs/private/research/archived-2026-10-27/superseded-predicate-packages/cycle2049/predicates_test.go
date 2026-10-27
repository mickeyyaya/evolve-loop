//go:build acs

package cycle2049

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/regressiontia"
)

const (
	pureRegression    = "./acs/regression/pure"
	directHatch       = "./acs/regression/directhatch"
	viaHarness        = "./acs/regression/viaharness"
	viaChain          = "./acs/regression/viachain"
	unrelatedChange   = "./internal/other/..."
	dependencyChange  = "./internal/leaf/..."
	fixtureModulePath = "example.com/m"
)

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func regressionTest(pkg string, imports ...string) string {
	body := "//go:build acs\n\npackage " + pkg + "\n\nimport (\n\t\"testing\"\n"
	for _, imp := range imports {
		body += "\t\"" + imp + "\"\n"
	}
	body += ")\n\nfunc TestFixture(t *testing.T) {\n\t_ = t\n"
	for _, imp := range imports {
		switch imp {
		case "os":
			body += "\t_ = os.Getpagesize()\n"
		case fixtureModulePath + "/internal/leaf":
			body += "\t_ = leaf.Value()\n"
		case fixtureModulePath + "/internal/harness":
			body += "\t_ = harness.Value()\n"
		case fixtureModulePath + "/internal/chain":
			body += "\t_ = chain.Value()\n"
		}
	}
	return body + "}\n"
}

func fixtureModule(t *testing.T) (repoRoot, moduleDir string) {
	t.Helper()
	repoRoot = t.TempDir()
	moduleDir = filepath.Join(repoRoot, "go")
	writeFile(t, moduleDir, "go.mod", "module "+fixtureModulePath+"\n\ngo 1.21\n")
	writeFile(t, moduleDir, "internal/leaf/leaf.go", "package leaf\n\nfunc Value() int { return 1 }\n")
	writeFile(t, moduleDir, "internal/other/other.go", "package other\n\nfunc Value() int { return 2 }\n")
	writeFile(t, moduleDir, "internal/harness/harness.go", "package harness\n\nimport \"os\"\n\nfunc Value() int { return os.Getpagesize() }\n")
	writeFile(t, moduleDir, "internal/helper/helper.go", "package helper\n\nimport \"os\"\n\nfunc Value() int { return os.Getpagesize() }\n")
	writeFile(t, moduleDir, "internal/chain/chain.go", "package chain\n\nimport \""+fixtureModulePath+"/internal/helper\"\n\nfunc Value() int { return helper.Value() }\n")
	writeFile(t, moduleDir, "acs/regression/pure/pure_test.go", regressionTest("pure", fixtureModulePath+"/internal/leaf"))
	writeFile(t, moduleDir, "acs/regression/directhatch/directhatch_test.go", regressionTest("directhatch", "os", fixtureModulePath+"/internal/leaf"))
	writeFile(t, moduleDir, "acs/regression/viaharness/viaharness_test.go", regressionTest("viaharness", fixtureModulePath+"/internal/harness"))
	writeFile(t, moduleDir, "acs/regression/viachain/viachain_test.go", regressionTest("viachain", fixtureModulePath+"/internal/chain"))
	return repoRoot, moduleDir
}

func TestC2049_001_PackageWithNoEscapeHatchInItsModuleLocalGraphIsSkippable(t *testing.T) {
	repoRoot, moduleDir := fixtureModule(t)

	d := regressiontia.Compute("shadow", repoRoot, moduleDir, []string{unrelatedChange})

	if len(d.ChangedPackages) == 0 {
		t.Fatalf("ChangedPackages is empty: Select keeps every package, so the dependency walk is never exercised")
	}
	if !slices.Contains(d.WouldSkip, pureRegression) {
		t.Errorf("WouldSkip = %v, Selected = %v: %s imports only testing and ./internal/leaf, so its module-local graph holds no escape hatch and it must be skippable", d.WouldSkip, d.Selected, pureRegression)
	}
	if !slices.Contains(d.Selected, directHatch) {
		t.Errorf("Selected = %v: %s imports os directly and must stay selected", d.Selected, directHatch)
	}
}

func TestC2049_002_ReachingOsThroughAModulePackageStaysAnEscapeHatch(t *testing.T) {
	repoRoot, moduleDir := fixtureModule(t)

	d := regressiontia.Compute("shadow", repoRoot, moduleDir, []string{unrelatedChange})

	if len(d.ChangedPackages) == 0 {
		t.Fatalf("ChangedPackages is empty: the result would hold trivially")
	}
	for _, hatch := range []string{viaHarness, viaChain} {
		if slices.Contains(d.WouldSkip, hatch) {
			t.Errorf("WouldSkip = %v: %s reaches os through a module package and must stay an escape hatch", d.WouldSkip, hatch)
		}
		if !slices.Contains(d.Selected, hatch) {
			t.Errorf("Selected = %v: %s must stay selected", d.Selected, hatch)
		}
	}
}

func TestC2049_003_ShadowSoakRecordsWouldSkipForAChangeOutsideSomePackagesDependencies(t *testing.T) {
	repoRoot, moduleDir := fixtureModule(t)
	runsDir := filepath.Join(t.TempDir(), "runs")
	workspace := filepath.Join(runsDir, "cycle-1")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	d := regressiontia.Compute("shadow", repoRoot, moduleDir, []string{unrelatedChange})
	if _, err := regressiontia.Emit(workspace, d); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	summary, err := regressiontia.Rollup(runsDir)
	if err != nil {
		t.Fatalf("Rollup: %v", err)
	}

	if summary.Cycles != 1 {
		t.Fatalf("Rollup.Cycles = %d, want 1 recorded shadow decision", summary.Cycles)
	}
	if summary.WouldSkip <= 0 {
		t.Errorf("Rollup.WouldSkip = %d, Selected = %d: the shadow soak must record would_skip > 0 for a change that touches no dependency of %s", summary.WouldSkip, summary.Selected, pureRegression)
	}
}

func TestC2049_004_ChangeToADependencyStillSelectsTheDependingPackage(t *testing.T) {
	repoRoot, moduleDir := fixtureModule(t)

	d := regressiontia.Compute("shadow", repoRoot, moduleDir, []string{dependencyChange})

	if len(d.ChangedPackages) == 0 {
		t.Fatalf("ChangedPackages is empty: the result would hold trivially")
	}
	if slices.Contains(d.WouldSkip, pureRegression) {
		t.Errorf("WouldSkip = %v: %s imports ./internal/leaf, which changed, so it must stay selected", d.WouldSkip, pureRegression)
	}
	if !slices.Contains(d.Selected, pureRegression) {
		t.Errorf("Selected = %v: %s depends on the changed package", d.Selected, pureRegression)
	}
}
