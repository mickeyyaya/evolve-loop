//go:build acs

package cycle1869

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/paralleltest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const paralleltestImportPath = "github.com/mickeyyaya/evolve-loop/go/internal/paralleltest"

var guardPackages = []struct {
	dir, pkgName, guardFile, seamTest, seamHelper string
}{
	{"cmd/evolve", "main", "parallel_default_test.go", "TestBootBinaryRefresh_PolicyOffDisables", "swapBinaryCommit"},
	{"internal/core", "core", "parallel_default_test.go", "TestWriteBuildSelfCheck_HealthyWriteIsSilent", "captureStderr"},
	{"internal/phases/ship", "ship", "parallel_default_test.go", "TestRepoContractGate_OffSkips", "swapRepoContractTest"},
}

var duplicatedGuardFuncs = []string{
	"func testingTName(", "func isTopLevelTest(", "func isMethodCallOn(", "func startsWithParallel(",
	"func callsAnywhere(", "func declaredPackageVars(", "func localNames(", "func writtenPackageName(",
	"func renderNode(", "func processStateMutations(",
}

func goModuleDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runGo(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = goModuleDir(t)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func realPackageSources(t *testing.T, dir string) map[string]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(goModuleDir(t), dir, "*.go"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("glob %s/*.go: %v (%d files)", dir, err, len(paths))
	}
	sources := make(map[string]string, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		sources[filepath.Base(path)] = string(raw)
	}
	return sources
}

func withParallelAdded(t *testing.T, sources map[string]string, testName string) map[string]string {
	t.Helper()
	marker := "func " + testName + "(t *testing.T) {\n"
	mutated := make(map[string]string, len(sources))
	hits := 0
	for path, src := range sources {
		if strings.Contains(src, marker) {
			src = strings.Replace(src, marker, marker+"\tt.Parallel()\n", 1)
			hits++
		}
		mutated[path] = src
	}
	if hits != 1 {
		t.Fatalf("expected exactly one declaration of %s, found %d", testName, hits)
	}
	return mutated
}

func violationsFor(all []string, key string) []string {
	var mine []string
	for _, v := range all {
		if strings.HasPrefix(v, key+" ") || strings.HasPrefix(v, key+":") {
			mine = append(mine, v)
		}
	}
	return mine
}

func fixture(body string) map[string]string {
	return map[string]string{
		"fixture_test.go": "package fix\nimport (\n\t\"os\"\n\t\"testing\"\n)\nvar _ = os.Args\nvar sharedSeam = 1\n" + body,
	}
}

func TestC1869_001_SharedGuardJudgesParallelAndSerialTests(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		serial map[string]string
		want   string
	}{
		{"unlisted test without t.Parallel is flagged", "func TestF(t *testing.T) {\n\t_ = 1\n}\n", nil, "fix.TestF"},
		{"t.Parallel first passes", "func TestF(t *testing.T) {\n\tt.Parallel()\n}\n", nil, ""},
		{"listed serial test passes", "func TestF(t *testing.T) {\n\t_ = 1\n}\n", map[string]string{"fix.TestF": "reason"}, ""},
		{"t.Setenv test needs no entry", "func TestF(t *testing.T) {\n\tt.Setenv(\"A\", \"b\")\n}\n", nil, ""},
		{"t.Parallel only inside a subtest is flagged", "func TestF(t *testing.T) {\n\tt.Run(\"x\", func(t *testing.T) {\n\t\tt.Parallel()\n\t})\n}\n", nil, "fix.TestF"},
		{"t.Parallel not first is flagged", "func TestF(t *testing.T) {\n\t_ = 1\n\tt.Parallel()\n}\n", nil, "first statement"},
		{"renamed testing parameter is read", "func TestF(tt *testing.T) {\n\ttt.Parallel()\n}\n", nil, ""},
		{"listed test that calls t.Parallel is flagged", "func TestF(t *testing.T) {\n\tt.Parallel()\n}\n", map[string]string{"fix.TestF": "reason"}, "fix.TestF"},
		{"stale serial entry is flagged", "func TestF(t *testing.T) {\n\tt.Parallel()\n}\n", map[string]string{"fix.TestGone": "reason"}, "fix.TestGone"},
		{"direct package variable write is flagged", "func TestF(t *testing.T) {\n\tt.Parallel()\n\tsharedSeam = 2\n}\n", nil, "sharedSeam = 2"},
		{"os.Stderr swap is flagged", "func TestF(t *testing.T) {\n\tt.Parallel()\n\tos.Stderr = nil\n}\n", nil, "os.Stderr = nil"},
		{"os.NewFile on descriptor 2 is flagged", "func TestF(t *testing.T) {\n\tt.Parallel()\n\t_ = os.NewFile(2, \"x\")\n}\n", nil, "os.NewFile(2"},
		{"shadowing local is not a package write", "func TestF(t *testing.T) {\n\tt.Parallel()\n\tsharedSeam := 1\n\tsharedSeam = 2\n\t_ = sharedSeam\n}\n", nil, ""},
	}
	for _, tc := range cases {
		got := paralleltest.Violations(fixture(tc.body), paralleltest.Config{Serial: tc.serial})
		if tc.want == "" {
			if len(got) != 0 {
				t.Errorf("%s: want no violation, got %v", tc.name, got)
			}
			continue
		}
		if len(got) == 0 || !strings.Contains(strings.Join(got, "\n"), tc.want) {
			t.Errorf("%s: want a violation containing %q, got %v", tc.name, tc.want, got)
		}
	}
}

func TestC1869_002_SharedGuardFollowsHelpersTransitively(t *testing.T) {
	helpers := "func helperOuter() { helperInner() }\nfunc helperInner() { sharedSeam = 5 }\nfunc readOnly() int { return sharedSeam }\n"
	parallelCallsMutator := helpers + "func TestFixtureHelper(t *testing.T) {\n\tt.Parallel()\n\thelperOuter()\n}\n"
	got := paralleltest.Violations(fixture(parallelCallsMutator), paralleltest.Config{})
	if len(got) != 1 || !strings.Contains(got[0], "fix.TestFixtureHelper") || !strings.Contains(got[0], "helperInner") {
		t.Errorf("a parallel test reaching a package write two helpers deep must be flagged once, naming the test and helperInner; got %v", got)
	}
	parallelCallsReader := helpers + "func TestFixtureRead(t *testing.T) {\n\tt.Parallel()\n\t_ = readOnly()\n}\n"
	if got := paralleltest.Violations(fixture(parallelCallsReader), paralleltest.Config{}); len(got) != 0 {
		t.Errorf("a parallel test whose helper only reads a package variable must pass; got %v", got)
	}
	serialCallsMutator := helpers + "func TestFixtureSerial(t *testing.T) {\n\thelperOuter()\n}\n"
	if got := paralleltest.Violations(fixture(serialCallsMutator), paralleltest.Config{Serial: map[string]string{"fix.TestFixtureSerial": "swaps sharedSeam"}}); len(got) != 0 {
		t.Errorf("a listed serial test may reach a mutating helper; got %v", got)
	}
	helperSetsEnv := "func setsEnv(tb *testing.T) { tb.Setenv(\"A\", \"b\") }\nfunc TestFixtureEnvHelper(t *testing.T) {\n\tt.Parallel()\n\tsetsEnv(t)\n}\n"
	if got := paralleltest.Violations(fixture(helperSetsEnv), paralleltest.Config{}); len(got) != 1 || !strings.Contains(got[0], "setsEnv") {
		t.Errorf("a parallel test whose helper calls t.Setenv must be flagged naming setsEnv; got %v", got)
	}
}

func TestC1869_003_SharedGuardFollowsMethodsOnPackageValues(t *testing.T) {
	methods := "type seamBox struct{}\nvar box seamBox\nfunc (seamBox) swap() { sharedSeam = 7 }\nfunc (seamBox) peek() int { return sharedSeam }\n"
	mutating := methods + "func TestFixtureMethod(t *testing.T) {\n\tt.Parallel()\n\tbox.swap()\n}\n"
	got := paralleltest.Violations(fixture(mutating), paralleltest.Config{})
	if len(got) != 1 || !strings.Contains(got[0], "fix.TestFixtureMethod") || !strings.Contains(got[0], "sharedSeam = 7") {
		t.Errorf("a parallel test calling a mutating method on a package-level value must be flagged once; got %v", got)
	}
	reading := methods + "func TestFixtureMethodRead(t *testing.T) {\n\tt.Parallel()\n\t_ = box.peek()\n}\n"
	if got := paralleltest.Violations(fixture(reading), paralleltest.Config{}); len(got) != 0 {
		t.Errorf("a parallel test calling a read-only method on a package-level value must pass; got %v", got)
	}
}

func TestC1869_004_GlobalSettersAreConfiguredPerPackage(t *testing.T) {
	body := "func TestFixtureSetter(t *testing.T) {\n\tt.Parallel()\n\tbridge.SetModelCatalogDirFn(nil)\n}\n"
	sources := map[string]string{"fixture_test.go": "package fix\nimport (\n\t\"testing\"\n\t\"example.com/bridge\"\n)\n" + body}
	configured := paralleltest.Violations(sources, paralleltest.Config{GlobalSetters: map[string]bool{"SetModelCatalogDirFn": true}})
	if len(configured) != 1 || !strings.Contains(configured[0], "SetModelCatalogDirFn") {
		t.Errorf("a configured process-global setter in a parallel test must be flagged; got %v", configured)
	}
	if got := paralleltest.Violations(sources, paralleltest.Config{}); len(got) != 0 {
		t.Errorf("a setter the package did not configure must not be flagged; got %v", got)
	}
}

func TestC1869_005_ASeamTestMadeParallelIsCaughtThroughItsHelperInEveryPackage(t *testing.T) {
	for _, pkg := range guardPackages {
		key := pkg.pkgName + "." + pkg.seamTest
		sources := realPackageSources(t, pkg.dir)
		for _, v := range violationsFor(paralleltest.Violations(sources, paralleltest.Config{}), key) {
			if strings.Contains(v, pkg.seamHelper) {
				t.Errorf("%s: the serial seam test must not be judged through %s before it is made parallel; got %q", pkg.dir, pkg.seamHelper, v)
			}
		}
		mutated := violationsFor(paralleltest.Violations(withParallelAdded(t, sources, pkg.seamTest), paralleltest.Config{}), key)
		if len(mutated) == 0 || !strings.Contains(strings.Join(mutated, "\n"), pkg.seamHelper) {
			t.Errorf("%s: %s given t.Parallel and no serial entry must be flagged through helper %s; got %v", pkg.dir, key, pkg.seamHelper, mutated)
		}
	}
}

func TestC1869_006_EachPackageDelegatesToTheOneGuardAndItsRealTreePasses(t *testing.T) {
	for _, pkg := range guardPackages {
		deps, err := runGo(t, "list", "-test", "-deps", "./"+pkg.dir)
		if err != nil {
			t.Fatalf("go list -test -deps ./%s: %v\n%s", pkg.dir, err, deps)
		}
		if !strings.Contains(deps, paralleltestImportPath+"\n") {
			t.Errorf("%s: its test binary does not depend on %s", pkg.dir, paralleltestImportPath)
		}
		guard := filepath.Join(goModuleDir(t), pkg.dir, pkg.guardFile)
		for _, decl := range duplicatedGuardFuncs {
			acsassert.FileNotContains(t, guard, decl)
		}
		out, err := runGo(t, "test", "-count=1", "-run", "Parallel|SerialByDefault", "./"+pkg.dir)
		if err != nil {
			t.Errorf("%s: the real tree must pass the shared guard: %v\n%s", pkg.dir, err, out)
			continue
		}
		if strings.Contains(out, "no tests to run") {
			t.Errorf("%s: no guard test matched Parallel|SerialByDefault:\n%s", pkg.dir, out)
		}
	}
}

func TestC1869_007_NewPackageIsEnrolledInTheApicoverGate(t *testing.T) {
	moduleDir := goModuleDir(t)
	raw, err := os.ReadFile(filepath.Join(moduleDir, ".apicover-enforce"))
	if err != nil {
		t.Fatalf("read .apicover-enforce: %v", err)
	}
	enrolled := false
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "./internal/paralleltest" {
			enrolled = true
		}
	}
	if !enrolled {
		t.Errorf(".apicover-enforce does not list ./internal/paralleltest")
	}
	named := filepath.Join(moduleDir, "internal", "paralleltest", "apicover_named_test.go")
	if !acsassert.FileExists(t, named) {
		t.Fatalf("missing %s", named)
	}
	ignore := exec.Command("git", "-C", moduleDir, "check-ignore", "-q", named)
	if ignore.Run() == nil {
		t.Errorf("%s is gitignored and would be dropped at ship", named)
	}
	out, err := runGo(t, "test", "-count=1", "./internal/paralleltest")
	if err != nil || strings.Contains(out, "no test files") {
		t.Errorf("go test ./internal/paralleltest must pass with tests: %v\n%s", err, out)
	}
}

func TestC1869_008_TouchedGoFilesCarryNoComments(t *testing.T) {
	moduleDir := goModuleDir(t)
	files := []string{
		"internal/paralleltest/paralleltest.go",
		"internal/paralleltest/paralleltest_test.go",
		"internal/paralleltest/apicover_named_test.go",
	}
	for _, pkg := range guardPackages {
		files = append(files, filepath.Join(pkg.dir, pkg.guardFile))
	}
	for _, rel := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(moduleDir, rel), nil, parser.ParseComments)
		if err != nil {
			t.Errorf("%s: %v", rel, err)
			continue
		}
		for _, group := range parsed.Comments {
			for _, c := range group.List {
				if !strings.HasPrefix(c.Text, "//go:") {
					t.Errorf("%s: carries a comment: %s", rel, c.Text)
				}
			}
		}
	}
}
