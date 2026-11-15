//go:build acs

package cycle2406

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cmdPkg      = "./cmd/evolve"
	healthPkg   = "./internal/clihealth"
	cmdPattern  = `^(TestBootCLIUpdate_|TestUpdateCLIsAtBoundary_|TestBootGateHalts_|TestPrepareIteration_|TestRunLoop_.*(Smoke|CLIUpdate))`
	benchTest1  = "TestBootCLIUpdate_AFamilyThatFailsItsSmokeIsBenchedAndTheWaveStarts"
	benchTest2  = "TestBootCLIUpdate_TheRoutesSkipAFamilyBenchedBySmoke"
	benchTest3  = "TestUpdateCLIsAtBoundary_AFamilyThatFailsItsSmokeIsBenchedAndTheNextWaveRuns"
	benchTest4  = "TestBootGateHalts_ABenchedSmokeFailureLetsTheReadinessGateRun"
	haltTest1   = "TestBootCLIUpdate_AFamilyThatNoRouteCanDoWithoutHaltsTheLoop"
	haltTest2   = "TestBootCLIUpdate_ASmokeFailureWithNoRoutingTableStillHaltsTheLoop"
	haltTest3   = "TestUpdateCLIsAtBoundary_EveryRoutedFamilyFailingItsSmokeHaltsTheNextWave"
	haltTest4   = "TestRunLoop_ASmokeFailureAtBootHaltsBeforeAnyCycleNamingTheFamilyAndVersion"
	haltTest5   = "TestPrepareIteration_ASmokeFailureAtAWaveBoundaryHaltsTheNextWave"
	haltTest6   = "TestBootCLIUpdate_ASmokeFailureWithHealthyUsageStillHaltsTheLoop"
	benchedFile = "go/cmd/evolve/cmd_loop_cliupdate_bench_test.go"
)

func runGo(t *testing.T, args ...string) (string, error) {
	t.Helper()
	moduleRoot, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir, cmd.WaitDelay = moduleRoot, 5*time.Second
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func requirePasses(t *testing.T, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	out, err := runGo(t, "test", "-count=1", "-v", "-run", pattern, cmdPkg)
	if err != nil {
		t.Errorf("RED: go test -run %s exited with %v\n%s", pattern, err, acsassert.OutputTail(out, 40))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not pass", name)
		}
	}
}

func TestC2406_001_AFamilyThatFailsItsSmokeIsBenchedAndTheBootAndTheWaveBoundaryGoOnWhileTheRoutesSkipIt(t *testing.T) {
	requirePasses(t, benchTest1, benchTest2, benchTest3, benchTest4)
}

func TestC2406_002_WhenNoRouteHasAFamilyLeftToRunTheBootAndTheBoundaryHaltAsToday(t *testing.T) {
	requirePasses(t, haltTest1, haltTest2, haltTest3, haltTest4, haltTest5, haltTest6)
}

type changedFile struct {
	rel   string
	lines map[int]bool
}

func gitOut(t *testing.T, root string, args ...string) string {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("git", append([]string{"-C", root}, args...)...)
	if err != nil || code != 0 {
		t.Fatalf("git %s: code=%d err=%v\n%s", strings.Join(args, " "), code, err, stderr)
	}
	return stdout
}

func diffBase(t *testing.T, root string) string {
	t.Helper()
	for _, ref := range []string{"refs/heads/main", "refs/remotes/origin/main"} {
		if base, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", ref); code == 0 {
			return strings.TrimSpace(base)
		}
	}
	return "HEAD"
}

var hunkHeader = regexp.MustCompile(`^@@ -\S+ \+(\d+)(?:,(\d+))? @@`)

func changedFiles(t *testing.T, keep func(rel string) bool) []changedFile {
	t.Helper()
	root := acsassert.RepoRoot(t)
	var files []changedFile
	var current *changedFile
	next := 0
	for _, line := range strings.Split(gitOut(t, root, "diff", "-U0", "--no-color", diffBase(t, root), "--"), "\n") {
		switch {
		case strings.HasPrefix(line, "+++ b/"):
			rel := strings.TrimPrefix(line, "+++ b/")
			if !keep(rel) {
				current = nil
				continue
			}
			files = append(files, changedFile{rel: rel, lines: map[int]bool{}})
			current = &files[len(files)-1]
		case current == nil:
		case strings.HasPrefix(line, "@@"):
			m := hunkHeader.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			next, _ = strconv.Atoi(m[1])
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			current.lines[next] = true
			next++
		}
	}
	for _, rel := range strings.Fields(gitOut(t, root, "ls-files", "--others", "--exclude-standard")) {
		if !keep(rel) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		all := map[int]bool{}
		for i := range strings.Split(string(raw), "\n") {
			all[i+1] = true
		}
		files = append(files, changedFile{rel: rel, lines: all})
	}
	return files
}

func isProductionGo(rel string) bool {
	return strings.HasPrefix(rel, "go/") && strings.HasSuffix(rel, ".go") &&
		!strings.HasSuffix(rel, "_test.go") && !strings.HasPrefix(rel, "go/acs/")
}

func changedFuncs(t *testing.T, f changedFile) []string {
	t.Helper()
	path := filepath.Join(acsassert.RepoRoot(t), f.rel)
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", f.rel, err)
	}
	var names []string
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		for line := fset.Position(fn.Pos()).Line; line <= fset.Position(fn.End()).Line; line++ {
			if f.lines[line] {
				names = append(names, fn.Name.Name)
				break
			}
		}
	}
	return names
}

func TestC2406_003_EveryChangedProductionFunctionIsFullyCoveredByTheTestsThatDriveIt(t *testing.T) {
	files := changedFiles(t, isProductionGo)
	if len(files) == 0 {
		t.Fatalf("RED: no production Go file changed; the boot and the boundary updater must change")
	}
	byPackage := map[string][]string{}
	total := 0
	for _, f := range files {
		pkg := "./" + filepath.ToSlash(filepath.Dir(strings.TrimPrefix(f.rel, "go/")))
		for _, name := range changedFuncs(t, f) {
			byPackage[pkg] = append(byPackage[pkg], name)
			total++
		}
	}
	if total == 0 {
		t.Fatalf("RED: the changed files hold no changed function")
	}
	for pkg, names := range byPackage {
		profile := filepath.Join(t.TempDir(), "cover.out")
		args := []string{"test", "-count=1", "-coverprofile=" + profile}
		if pkg == cmdPkg {
			args = append(args, "-run", cmdPattern)
		}
		if out, err := runGo(t, append(args, pkg)...); err != nil {
			t.Fatalf("RED: go test %s exited with %v\n%s", pkg, err, acsassert.OutputTail(out, 40))
		}
		report, err := runGo(t, "tool", "cover", "-func="+profile)
		if err != nil {
			t.Fatalf("go tool cover: %v\n%s", err, report)
		}
		slices.Sort(names)
		for _, name := range slices.Compact(names) {
			pct, err := acsassert.FuncCoverage(report, name)
			if err != nil {
				t.Errorf("RED: %s %s: %v", pkg, name, err)
				continue
			}
			if pct < 100 {
				t.Errorf("RED: %s %s is covered %.1f%%, want 100%%", pkg, name, pct)
			}
		}
	}
}

func TestC2406_004_NoChangedGoFileAddsACodeComment(t *testing.T) {
	files := changedFiles(t, func(rel string) bool { return strings.HasPrefix(rel, "go/") && strings.HasSuffix(rel, ".go") })
	benchTestSeen := false
	for _, f := range files {
		benchTestSeen = benchTestSeen || strings.HasSuffix(benchedFile, f.rel)
		raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), f.rel))
		if err != nil {
			t.Fatal(err)
		}
		for i, text := range strings.Split(string(raw), "\n") {
			trimmed := strings.TrimSpace(text)
			if !f.lines[i+1] || !strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "//go:build") || strings.HasPrefix(trimmed, "// acs-predicate:") {
				continue
			}
			t.Errorf("RED: %s:%d adds a code comment: %s", f.rel, i+1, trimmed)
		}
	}
	if !benchTestSeen {
		t.Errorf("RED: %s is missing from the change", benchedFile)
	}
	if len(changedFiles(t, isProductionGo)) == 0 {
		t.Errorf("RED: no production Go file changed")
	}
}

func TestC2406_005_EveryAddedDocumentLineInTheChangeIsSteClean(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docs := changedFiles(t, func(rel string) bool { return strings.HasSuffix(rel, ".md") && !strings.HasPrefix(rel, ".evolve/") })
	if len(docs) == 0 {
		t.Fatalf("RED: no document changed; the package note of the changed behavior must say that a smoke failure benches the family")
	}
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	for _, doc := range docs {
		path := filepath.Join(root, doc.rel)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		findings, err := stelint.LintFile(path, stelint.Options{Words: words})
		if err != nil {
			t.Fatalf("lint %s: %v", doc.rel, err)
		}
		for _, finding := range stelint.KeepAdded(raw, false, doc.lines, findings) {
			t.Errorf("RED: %s", fmt.Sprintf("%s:%d %s", doc.rel, finding.Line, finding.Message))
		}
	}
}
