package core

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

const (
	dataPkgSource = "package x\n\nimport (\n\t_ \"embed\"\n)\n\n//go:embed data.json\nvar Data string\n\nfunc Len() int { return len(Data) }\n"
	dataPkgTest   = "package x\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestFixture(t *testing.T) {\n\tb, err := os.ReadFile(\"testdata/f.json\")\n\tif err != nil || string(b) != \"ok\" {\n\t\tt.Fatalf(\"fixture = %q, %v\", b, err)\n\t}\n\t_ = Len()\n}\n"
)

func dataPkgBase() map[string]string {
	return map[string]string{
		"internal/x/x.go":            dataPkgSource,
		"internal/x/x_test.go":       dataPkgTest,
		"internal/x/data.json":       "{}",
		"internal/x/testdata/f.json": "ok",
	}
}

func TestFloorTestPackages_RunsThePackageOfAChangedTestdataFile(t *testing.T) {
	t.Parallel()
	var warn strings.Builder

	got := floorTestPackages([]string{"go/internal/x/testdata/f.json"}, &warn)

	if strings.Join(got, " ") != "./internal/x" {
		t.Fatalf("packages = %v, want ./internal/x for a lane that changed only a fixture", got)
	}
}

func TestFloorTestPackages_RunsThePackageOfAChangedPackageRootDataFile(t *testing.T) {
	t.Parallel()
	var warn strings.Builder

	got := floorTestPackages([]string{"go/internal/x/data.json", "go/internal/x/testdata/a/b/c.json"}, &warn)

	if strings.Join(got, " ") != "./internal/x" {
		t.Fatalf("packages = %v, want ./internal/x once for a root data file and a deep fixture", got)
	}
}

func TestFloorTestPackages_ADataFileAndAGoFileInOnePackageRunItOnce(t *testing.T) {
	t.Parallel()
	var warn strings.Builder

	got := floorTestPackages([]string{"go/internal/x/testdata/f.json", "go/internal/x/x.go", "go/q/q.go"}, &warn)

	if strings.Join(got, " ") != "./internal/x ./q" {
		t.Fatalf("packages = %v, want ./internal/x once and ./q", got)
	}
}

func TestFloorTestPackages_AnEnvExclusiveDataPackageIsNotedAndNotRun(t *testing.T) {
	t.Parallel()
	var warn strings.Builder

	got := floorTestPackages([]string{"go/internal/bridge/testdata/g.json"}, &warn)

	if len(got) != 0 || !strings.Contains(warn.String(), "./internal/bridge") {
		t.Fatalf("packages = %v, warn = %q, want the env-exclusive package noted and not run", got, warn.String())
	}
}

func TestFloorTestPackages_ATestdataGoFileKeepsItsOwnPackageBesideTheOwner(t *testing.T) {
	t.Parallel()
	var warn strings.Builder

	got := floorTestPackages([]string{"go/internal/x/testdata/y/y.go"}, &warn)

	if strings.Join(got, " ") != "./internal/x/testdata/y" {
		t.Fatalf("packages = %v, want today's behavior for a .go file under testdata kept", got)
	}
}

func TestFloorChangedPackages_IgnoresModuleRootAndForeignPaths(t *testing.T) {
	t.Parallel()
	paths := []string{"go/go.mod", "go/go.sum", "go/vendor/m/x.json", "go/README.md", "go/testdata/f.json", "docs/testdata/f.json", "README.md", "go/.apicover-enforce"}

	got := floorChangedPackages(paths)

	if len(got) != 0 {
		t.Fatalf("packages = %v, want none: module-root changes belong to the composed gates and CI", got)
	}
}

func TestCoverChangedPackages_AFixtureOrDataOnlyChangeSelectsNoPackage(t *testing.T) {
	t.Parallel()
	in, _ := scopeJourneyRepo(t, dataPkgBase(), map[string]string{"internal/x/data.json": "{ }"})

	got := coverChangedPackages(in.Worktree+"/go", []string{"go/internal/x/testdata/f.json", "go/internal/x/data.json"})

	if len(got) != 0 {
		t.Fatalf("cover-changed packages = %v, want none: a data file has no statements", got)
	}
}

func TestDefaultBuildFloorChecks_AFixtureOnlyChangeThatBreaksTheTestFailsTheFloor(t *testing.T) {
	t.Parallel()
	in, _ := scopeJourneyRepo(t, dataPkgBase(), map[string]string{"internal/x/testdata/f.json": "broken"})

	failures := strings.Join(DefaultBuildFloorChecks(context.Background(), in), "\n")

	if !strings.Contains(failures, "TestFixture") {
		t.Fatalf("failures = %q, want TestFixture named: a fixture-only diff must run its package", failures)
	}
}

func TestDefaultBuildFloorChecks_AnEmbeddedDataFileOnlyChangeRunsItsPackage(t *testing.T) {
	t.Parallel()
	base := dataPkgBase()
	base["internal/x/x_test.go"] = strings.Replace(dataPkgTest, "_ = Len()", "if Len() != 2 {\n\t\tt.Fatalf(\"embedded = %q\", Data)\n\t}", 1)
	in, _ := scopeJourneyRepo(t, base, map[string]string{"internal/x/data.json": "{ }"})

	failures := strings.Join(DefaultBuildFloorChecks(context.Background(), in), "\n")

	if !strings.Contains(failures, "TestFixture") {
		t.Fatalf("failures = %q, want the embedded-data test red", failures)
	}
}

func TestDefaultBuildFloorChecks_AnEmbedDirectoryWithoutGoFilesMakesNoFalseRed(t *testing.T) {
	t.Parallel()
	in, _ := scopeJourneyRepo(t, dataPkgBase(), map[string]string{"internal/x/templates/t.txt": "new"})
	resolved, err := filepath.EvalSymlinks(in.Worktree)
	if err != nil {
		t.Fatalf("resolve worktree: %v", err)
	}
	in.Worktree = resolved

	failures := DefaultBuildFloorChecks(context.Background(), in)

	if len(failures) != 0 {
		t.Fatalf("failures = %q, want none for a changed file in a directory with no Go files", failures)
	}
}

func TestDefaultBuildFloorChecks_EnforcedPackageFixtureBreakFailsTheFloor(t *testing.T) {
	t.Parallel()
	base := dataPkgBase()
	base[".apicover-enforce"] = "./internal/x\n"
	in, _ := scopeJourneyRepo(t, base, map[string]string{"internal/x/testdata/f.json": "broken"})

	failures := strings.Join(DefaultBuildFloorChecks(context.Background(), in), "\n")

	if !strings.Contains(failures, "enforced package tests FAIL") {
		t.Fatalf("failures = %q, want 'enforced package tests FAIL' for a broken fixture in an enforced package", failures)
	}
}

func TestDefaultBuildFloorChecks_EnforcedPackageGreenFixtureChangeDemandsNoNaming(t *testing.T) {
	t.Parallel()
	base := dataPkgBase()
	base[".apicover-enforce"] = "./internal/x\n"
	base["internal/x/testdata/f.json"] = "old"
	base["internal/x/x_test.go"] = strings.Replace(dataPkgTest, `string(b) != "ok"`, `len(b) == 0`, 1)
	in, _ := scopeJourneyRepo(t, base, map[string]string{"internal/x/testdata/f.json": "ok"})

	failures := strings.Join(DefaultBuildFloorChecks(context.Background(), in), "\n")

	if strings.Contains(failures, "apicover naming floor") {
		t.Fatalf("failures = %q, want no naming demand for pre-existing exports when only data changed", failures)
	}
}
