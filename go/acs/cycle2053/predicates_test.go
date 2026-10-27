//go:build acs

package cycle2053

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/regressiontia"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	fixtureModulePath = "example.com/m"
	pureRegression    = "./acs/regression/pure"
	externalRegress   = "./acs/regression/external"
	isolatedRegress   = "./acs/regression/isolated"
	directHatch       = "./acs/regression/directhatch"
	viaHarness        = "./acs/regression/viaharness"
	viaChain          = "./acs/regression/viachain"
	viaExec           = "./acs/regression/viaexec"
	unrelatedChange   = "./internal/other"
	dependencyChange  = "./internal/leaf"
	packageUnderTest  = "./internal/regressiontia"
	apicoverRegress   = "./acs/regression/apicover"
	packageDocPath    = "docs/architecture/packages/internal-regressiontia.md"
	adrPath           = "docs/architecture/adr/0082-regression-test-impact-selection-shadow.md"
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
		default:
			body += "\t_ = " + filepath.Base(imp) + ".Value()\n"
		}
	}
	return body + "}\n"
}

func fixtureModule(t *testing.T) (repoRoot, moduleDir string) {
	t.Helper()
	repoRoot = t.TempDir()
	moduleDir = filepath.Join(repoRoot, "go")
	internal := fixtureModulePath + "/internal/"
	writeFile(t, moduleDir, "go.mod", "module "+fixtureModulePath+"\n\ngo 1.21\n")
	writeFile(t, moduleDir, "internal/leaf/leaf.go", "package leaf\n\nfunc Value() int { return 1 }\n")
	writeFile(t, moduleDir, "internal/other/other.go", "package other\n\nfunc Value() int { return 2 }\n")
	writeFile(t, moduleDir, "internal/harness/harness.go", "package harness\n\nimport \"os\"\n\nfunc Value() int { return os.Getpagesize() }\n")
	writeFile(t, moduleDir, "internal/runner/runner.go", "package runner\n\nimport \"os/exec\"\n\nfunc Value() int { return len(exec.Command(\"true\").Args) }\n")
	writeFile(t, moduleDir, "internal/helper/helper.go", "package helper\n\nimport \"os\"\n\nfunc Value() int { return os.Getpagesize() }\n")
	writeFile(t, moduleDir, "internal/chain/chain.go", "package chain\n\nimport \""+internal+"helper\"\n\nfunc Value() int { return helper.Value() }\n")
	writeFile(t, moduleDir, "acs/regression/pure/pure_test.go", regressionTest("pure", internal+"leaf"))
	writeFile(t, moduleDir, "acs/regression/external/external_test.go", regressionTest("external_test", internal+"leaf"))
	writeFile(t, moduleDir, "acs/regression/isolated/isolated_test.go", regressionTest("isolated"))
	writeFile(t, moduleDir, "acs/regression/directhatch/directhatch_test.go", regressionTest("directhatch", "os", internal+"leaf"))
	writeFile(t, moduleDir, "acs/regression/viaharness/viaharness_test.go", regressionTest("viaharness", internal+"harness"))
	writeFile(t, moduleDir, "acs/regression/viachain/viachain_test.go", regressionTest("viachain", internal+"chain"))
	writeFile(t, moduleDir, "acs/regression/viaexec/viaexec_test.go", regressionTest("viaexec", internal+"runner"))
	return repoRoot, moduleDir
}

func TestC2053_001_PackageWithNoEscapeHatchInItsModuleLocalGraphIsSkippable(t *testing.T) {
	repoRoot, moduleDir := fixtureModule(t)

	d := regressiontia.Compute("shadow", repoRoot, moduleDir, []string{unrelatedChange})

	if len(d.ChangedPackages) == 0 {
		t.Fatalf("ChangedPackages is empty: Select keeps every package, so the dependency walk is never exercised")
	}
	for _, skippable := range []string{pureRegression, externalRegress} {
		if !slices.Contains(d.WouldSkip, skippable) {
			t.Errorf("WouldSkip = %v, Selected = %v: %s imports only testing and ./internal/leaf, so its module-local graph holds no escape hatch and it must be skippable", d.WouldSkip, d.Selected, skippable)
		}
	}
	if !slices.Contains(d.Selected, directHatch) {
		t.Errorf("Selected = %v: %s imports os directly and must stay selected", d.Selected, directHatch)
	}
}

func TestC2053_002_ReachingOsThroughAModulePackageStaysAnEscapeHatch(t *testing.T) {
	repoRoot, moduleDir := fixtureModule(t)

	d := regressiontia.Compute("shadow", repoRoot, moduleDir, []string{unrelatedChange})

	if len(d.ChangedPackages) == 0 {
		t.Fatalf("ChangedPackages is empty: the result would hold trivially")
	}
	for _, hatch := range []string{viaHarness, viaChain, viaExec} {
		if slices.Contains(d.WouldSkip, hatch) {
			t.Errorf("WouldSkip = %v: %s reaches an escape hatch through a module package and must stay one", d.WouldSkip, hatch)
		}
		if !slices.Contains(d.Selected, hatch) {
			t.Errorf("Selected = %v: %s must stay selected", d.Selected, hatch)
		}
	}
}

func TestC2053_003_ShadowSoakRecordsWouldSkipForAChangeOutsideSomePackagesDependencies(t *testing.T) {
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

func TestC2053_004_ChangeToADependencyStillSelectsTheDependingPackages(t *testing.T) {
	repoRoot, moduleDir := fixtureModule(t)

	d := regressiontia.Compute("shadow", repoRoot, moduleDir, []string{dependencyChange})

	if len(d.ChangedPackages) == 0 {
		t.Fatalf("ChangedPackages is empty: the result would hold trivially")
	}
	for _, dependent := range []string{pureRegression, externalRegress} {
		if slices.Contains(d.WouldSkip, dependent) {
			t.Errorf("WouldSkip = %v: %s imports ./internal/leaf, which changed, so it must stay selected", d.WouldSkip, dependent)
		}
		if !slices.Contains(d.Selected, dependent) {
			t.Errorf("Selected = %v: %s depends on the changed package", d.Selected, dependent)
		}
	}
}

func TestC2053_005_PackageWithNoModuleDependencyStaysSelected(t *testing.T) {
	repoRoot, moduleDir := fixtureModule(t)

	d := regressiontia.Compute("shadow", repoRoot, moduleDir, []string{unrelatedChange})

	if len(d.ChangedPackages) == 0 {
		t.Fatalf("ChangedPackages is empty: the result would hold trivially")
	}
	if slices.Contains(d.WouldSkip, isolatedRegress) {
		t.Errorf("WouldSkip = %v: %s declares no module dependency, so nothing proves what it observes and it must run", d.WouldSkip, isolatedRegress)
	}
	if !slices.Contains(d.Selected, isolatedRegress) {
		t.Errorf("Selected = %v: %s must stay selected", d.Selected, isolatedRegress)
	}
}

func TestC2053_006_RealTreeKeepsApicoverSelectedForAChangeOutsideItsImports(t *testing.T) {
	root := acsassert.RepoRoot(t)

	d := regressiontia.Compute("shadow", root, filepath.Join(root, "go"), []string{packageUnderTest})

	if len(d.ChangedPackages) == 0 {
		t.Fatalf("ChangedPackages is empty: the real-tree walk never ran")
	}
	if slices.Contains(d.WouldSkip, apicoverRegress) {
		t.Errorf("WouldSkip holds %s: apicover reads go/.apicover-enforce and shells out to go list through acsassert, so it must stay an escape hatch", apicoverRegress)
	}
	if !slices.Contains(d.Selected, apicoverRegress) {
		t.Errorf("Selected = %v: %s must stay selected", d.Selected, apicoverRegress)
	}
}

var coverageLine = regexp.MustCompile(`coverage: ([0-9.]+)% of statements`)

func TestC2053_007_RegressionTIAPackageIsFullyCovered(t *testing.T) {
	root := acsassert.RepoRoot(t)

	out, stderr, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-cover", packageUnderTest)
	if err != nil || code != 0 {
		t.Fatalf("go test %s exit=%d: %v\n%s\n%s", packageUnderTest, code, err, out, stderr)
	}
	match := coverageLine.FindStringSubmatch(out)
	if match == nil {
		t.Fatalf("no coverage line in the go test output: %s", out)
	}
	percent, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		t.Fatal(err)
	}
	if percent < 100.0 {
		t.Errorf("coverage of %s = %.1f%%, want 100.0%%", packageUnderTest, percent)
	}
}

var testReference = regexp.MustCompile("`(Test[A-Za-z0-9_]+)`")

func testFuncExists(t *testing.T, goDir, name string) bool {
	t.Helper()
	needle := "func " + name + "("
	found := false
	walkErr := filepath.WalkDir(goDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || found {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		found = strings.Contains(string(raw), needle)
		return nil
	})
	if walkErr != nil {
		t.Fatal(walkErr)
	}
	return found
}

func TestC2053_008_PackageDocNamesOnlyTestsThatExist(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, packageDocPath))
	if err != nil {
		t.Fatal(err)
	}

	references := testReference.FindAllStringSubmatch(string(raw), -1)

	if len(references) == 0 {
		t.Fatalf("%s names no test: the pinning claims were removed", packageDocPath)
	}
	for _, ref := range references {
		if !testFuncExists(t, filepath.Join(root, "go"), ref[1]) {
			t.Errorf("%s pins %s, but no *_test.go file under go/ defines it", packageDocPath, ref[1])
		}
	}
}

// acs-predicate: config-check
func TestC2053_009_ADRRecordsTheModuleLocalEscapeHatchWalk(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adr := filepath.Join(root, adrPath)

	if !acsassert.FileContains(t, adr, "module-local") {
		t.Errorf("%s does not record the module-local escape-hatch rule (direct imports of module packages only)", adrPath)
	}
	if !acsassert.FileContainsAny(adr, "TestC2053_", "TestCompute_SkipsAPredicateThatReachesOsOnlyThroughTesting") {
		t.Errorf("%s names no test that pins the module-local rule", adrPath)
	}
}
