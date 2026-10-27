//go:build acs

package cycle2044

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const acsassertPackage = "github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"

var newHelpers = []string{"ModuleDir", "OutputTail", "FuncCoverage", "WalkModule"}

var coverageRun struct {
	once      sync.Once
	coverFunc string
	failure   string
}

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func acsassertCoverFunc(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	coverageRun.once.Do(func() { coverageRun.coverFunc, coverageRun.failure = measureAcsassert(dir) })
	if coverageRun.failure != "" {
		t.Fatalf("coverage of pkg/acsassert: %s", coverageRun.failure)
	}
	return coverageRun.coverFunc
}

func measureAcsassert(dir string) (string, string) {
	scratch, err := os.MkdirTemp("", "cycle2044-cover-")
	if err != nil {
		return "", err.Error()
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	profile := filepath.Join(scratch, "cover.out")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	for _, args := range [][]string{
		{"test", "-count=1", "-coverprofile=" + profile, "./pkg/acsassert"},
		{"tool", "cover", "-func=" + profile},
	} {
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = dir
		cmd.Env = ipcenv.GoTestEnv()
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		if err := cmd.Run(); err != nil {
			return "", "go " + strings.Join(args, " ") + ": " + err.Error() + "\n" + out.String() + errOut.String()
		}
		if args[0] == "tool" {
			return out.String(), ""
		}
	}
	return "", "no coverage output"
}

func requireHelperTests(t *testing.T, names ...string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     goDir(t),
		Package: acsassertPackage,
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2044_001_ModuleDirHasATableTestAndAFailureCase(t *testing.T) {
	requireHelperTests(t,
		"TestModuleDir_FindsTheNearestGoModAboveTheStart",
		"TestModuleDir_RefusesAStartWithNoGoModAboveIt",
		"TestModuleDir_ResolvesTheRealModuleOfThisPackage",
	)
}

func TestC2044_002_OutputTailHasATableTestAndAFailureCase(t *testing.T) {
	requireHelperTests(t,
		"TestOutputTail_KeepsTheLastLines",
		"TestOutputTail_ReturnsNothingForANonPositiveCount",
	)
}

func TestC2044_003_FuncCoverageHasATableTestAndAFailureCase(t *testing.T) {
	requireHelperTests(t,
		"TestFuncCoverage_ReadsTheCoverageOfOneFunction",
		"TestFuncCoverage_RefusesAnOutputThatDoesNotAnswer",
		"TestFuncCoverage_SkipsAMalformedLineAndStillFindsTheFunction",
	)
}

func TestC2044_004_WalkModuleHasATableTestAndAFailureCase(t *testing.T) {
	requireHelperTests(t,
		"TestWalkModule_ListsTheFilesTheFilterKeeps",
		"TestWalkModule_ReturnsNothingForAnEmptyDirectory",
		"TestWalkModule_RefusesARootItCannotWalk",
	)
}

func TestC2044_005_ApicoverFindsEachNewHelperNamedByATestThatRunsIt(t *testing.T) {
	pkgDir := filepath.Join(goDir(t), "pkg", "acsassert")
	symbols, err := apicover.Enumerate(context.Background(), pkgDir)
	if err != nil {
		t.Fatalf("enumerate %s: %v", pkgDir, err)
	}
	exportedFuncs := map[string]bool{}
	for _, s := range symbols {
		if s.Kind == apicover.KindFunc {
			exportedFuncs[s.Name] = true
		}
	}
	for _, name := range newHelpers {
		if !exportedFuncs[name] {
			t.Errorf("pkg/acsassert exports no func %s", name)
		}
	}
	coverFunc := filepath.Join(t.TempDir(), "cover.func")
	if err := os.WriteFile(coverFunc, []byte(acsassertCoverFunc(t)), 0o644); err != nil {
		t.Fatal(err)
	}
	var report bytes.Buffer
	code, err := apicover.Run(context.Background(), apicover.Config{Dirs: []string{pkgDir}, CoverPath: coverFunc, Enforce: true}, &report)
	if err != nil || code != 0 {
		t.Errorf("apicover on pkg/acsassert exited %d (err=%v):\n%s", code, err, report.String())
	}
	for _, name := range newHelpers {
		if strings.Contains(report.String(), name) {
			t.Errorf("apicover reports %s as a problem:\n%s", name, report.String())
		}
	}
}

// acs-predicate: config-check
func TestC2044_006_PersonaAndReadmeNameEveryNewHelper(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{"agents/evolve-tdd-engineer.md", "go/acs/README.md"} {
		path := filepath.Join(root, rel)
		if !acsassert.FileExists(t, path) {
			continue
		}
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
			t.Errorf("%s is untracked: it may be gitignored and dropped at ship", rel)
		}
		for _, name := range newHelpers {
			acsassert.FileContains(t, path, "`"+name+"`")
		}
	}
}

func TestC2044_007_EveryFunctionTheKitAddsIsFullyCovered(t *testing.T) {
	baselineFiles := []string{"assertions.go", "go_tests.go"}
	coverFunc := acsassertCoverFunc(t)
	covered := map[string]float64{}
	var addedFuncs, total int
	for _, line := range strings.Split(coverFunc, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pct, err := strconv.ParseFloat(strings.TrimSuffix(fields[len(fields)-1], "%"), 64)
		if err != nil {
			continue
		}
		if fields[0] == "total:" {
			total++
			if pct < 90.0 {
				t.Errorf("total coverage of pkg/acsassert %.1f%% fell under the 90.0%% baseline of the default tags", pct)
			}
			continue
		}
		covered[fields[1]] = pct
		file := strings.SplitN(fields[0], ":", 2)[0]
		if slices.Contains(baselineFiles, filepath.Base(file)) {
			continue
		}
		addedFuncs++
		if pct != 100 {
			t.Errorf("%s in %s has %.1f%% coverage, want 100.0%%", fields[1], filepath.Base(file), pct)
		}
	}
	if total != 1 {
		t.Errorf("cover output holds %d total lines, want 1", total)
	}
	for _, name := range newHelpers {
		pct, ok := covered[name]
		if !ok {
			t.Errorf("cover output lists no func %s", name)
			continue
		}
		if pct != 100 {
			t.Errorf("%s has %.1f%% coverage, want 100.0%%", name, pct)
		}
	}
	if addedFuncs < len(newHelpers) {
		t.Errorf("cover output holds %d funcs outside %v, want at least %d", addedFuncs, baselineFiles, len(newHelpers))
	}
}

func TestC2044_008_ThePredicateKitServesAConsumerEndToEnd(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	moduleDir, err := acsassert.ModuleDir(wd)
	if err != nil {
		t.Fatalf("ModuleDir(%s): %v", wd, err)
	}
	gotReal, gotErr := filepath.EvalSymlinks(moduleDir)
	wantReal, wantErr := filepath.EvalSymlinks(goDir(t))
	if gotErr != nil || wantErr != nil || gotReal != wantReal {
		t.Errorf("ModuleDir(%s) = %s, want %s (%v, %v)", wd, moduleDir, goDir(t), gotErr, wantErr)
	}
	files, err := acsassert.WalkModule(moduleDir, func(rel string) bool {
		return strings.HasPrefix(rel, "pkg/acsassert/") && strings.HasSuffix(rel, ".go")
	})
	if err != nil {
		t.Fatalf("WalkModule(%s): %v", moduleDir, err)
	}
	for _, want := range []string{"pkg/acsassert/assertions.go", "pkg/acsassert/predicate_kit_test.go"} {
		if !slices.Contains(files, want) {
			t.Errorf("WalkModule misses %s in %v", want, files)
		}
	}
	coverFunc := acsassertCoverFunc(t)
	if pct, err := acsassert.FuncCoverage(coverFunc, "FileExists"); err != nil || pct != 100 {
		t.Errorf("FuncCoverage(FileExists) = (%v, %v), want (100, nil)", pct, err)
	}
	if _, err := acsassert.FuncCoverage(coverFunc, "NoSuchFunction"); err == nil {
		t.Errorf("FuncCoverage(NoSuchFunction) returned no error")
	}
	tail := acsassert.OutputTail(coverFunc, 1)
	if !strings.HasPrefix(tail, "total:") || strings.Contains(tail, "\n") {
		t.Errorf("OutputTail(coverFunc, 1) = %q, want the single total: line", tail)
	}
}
