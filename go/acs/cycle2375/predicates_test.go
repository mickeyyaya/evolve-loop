//go:build acs

package cycle2375

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	designDoc        = "docs/architecture/composed-gate-scope.md"
	adrDir           = "docs/architecture/adr"
	scopeFile        = "go/internal/changedpkgs/gatescope.go"
	wiringFile       = "go/cmd/evolve/cmd_composition_wiring.go"
	scopeTestFile    = "go/internal/changedpkgs/gatescope_test.go"
	wiringTestFile   = "go/cmd/evolve/cmd_composition_scope_test.go"
	changedpkgsPath  = "github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	cmdEvolvePath    = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	firstNewADR      = 166
	requiredCoverage = 100.0
)

var (
	cpuSeconds   = regexp.MustCompile(`\b\d+(\.\d+)?\s*(s|sec|seconds)\b`)
	wholeModule  = regexp.MustCompile(`(?i)whole[- ]module`)
	changedOnly  = regexp.MustCompile(`(?i)changed[- ]only|cycle-affected`)
	closureScope = regexp.MustCompile(`(?i)importer closure|interaction`)
	adrName      = regexp.MustCompile(`^(\d{4})-.*compos.*\.md$`)
	hunkHeader   = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)
)

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), rel))
	if err != nil {
		t.Fatalf("RED: %s: %v", rel, err)
	}
	return string(raw)
}

func requireShippable(t *testing.T, rel string) {
	t.Helper()
	if !acsassert.FileExists(t, filepath.Join(acsassert.RepoRoot(t), rel)) {
		t.Fatalf("RED: %s missing on disk", rel)
	}
	_, _, code, _ := acsassert.SubprocessOutput("git", "-C", acsassert.RepoRoot(t), "check-ignore", "-q", rel)
	if code != 1 {
		t.Fatalf("RED: %s is gitignored or unreadable by git (check-ignore exit %d): it would be dropped at ship", rel, code)
	}
}

func newADR(t *testing.T) string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(acsassert.RepoRoot(t), adrDir))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		m := adrName.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		if n, _ := strconv.Atoi(m[1]); n >= firstNewADR {
			return adrDir + "/" + e.Name()
		}
	}
	t.Fatalf("RED: no ADR numbered %04d or higher, with 'compos' in its name, in %s", firstNewADR, adrDir)
	return ""
}

func tableRows(text string) []string {
	var rows []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "|") {
			rows = append(rows, line)
		}
	}
	return rows
}

func cells(row string) []string {
	var out []string
	for _, c := range strings.Split(strings.Trim(strings.TrimSpace(row), "|"), "|") {
		if c = strings.TrimSpace(c); c != "" {
			out = append(out, c)
		}
	}
	return out
}

func scopeRow(t *testing.T, rows []string, pattern *regexp.Regexp, label string) []string {
	t.Helper()
	for _, row := range rows {
		c := cells(row)
		if len(c) > 0 && pattern.MatchString(c[0]) {
			return c
		}
	}
	t.Errorf("RED: %s has no table row whose first cell names the %s scope", designDoc, label)
	return nil
}

func TestC2375_001_TheDesignDocComparesThreeScopesWithMeasuredCPUAndEscapeRisk(t *testing.T) {
	requireShippable(t, designDoc)
	text := readRepoFile(t, designDoc)
	rows := tableRows(text)
	header := ""
	for _, row := range rows {
		if strings.Contains(strings.ToLower(row), "cpu") && strings.Contains(strings.ToLower(row), "escape") {
			header = row
		}
	}
	if header == "" {
		t.Fatalf("RED: %s has no table header with both a CPU column and an escape-risk column", designDoc)
	}
	seen := map[string]bool{}
	for label, pattern := range map[string]*regexp.Regexp{
		"whole module":    wholeModule,
		"cycle-affected":  changedOnly,
		"interaction set": closureScope,
	} {
		c := scopeRow(t, rows, pattern, label)
		if c == nil {
			continue
		}
		key := strings.Join(c, "|")
		if seen[key] {
			t.Errorf("RED: the %s scope reuses a row of another scope: %q", label, key)
		}
		seen[key] = true
		if !cpuSeconds.MatchString(strings.Join(c[1:], " ")) {
			t.Errorf("RED: the %s row has no measured CPU figure in seconds: %q", label, key)
		}
		if risk := c[len(c)-1]; len(strings.Fields(risk)) < 3 {
			t.Errorf("RED: the %s row ends with %q, which does not describe an escape risk", label, risk)
		}
	}
	if !regexp.MustCompile(`(?i)main delta`).MatchString(text) {
		t.Errorf("RED: %s never discusses the main delta", designDoc)
	}
}

func TestC2375_002_TheDesignDocAndTheADRPassTheSteLintWithZeroFindings(t *testing.T) {
	standard, err := stelint.LoadStandard(acsassert.RepoRoot(t))
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	adr := newADR(t)
	requireShippable(t, adr)
	for _, rel := range []string{designDoc, adr} {
		requireShippable(t, rel)
		for _, f := range stelint.Check([]byte(readRepoFile(t, rel)), stelint.Options{Words: standard}) {
			if !f.IsSkipNotice() {
				t.Error(fmt.Sprintf("%s:%d [%s] %s", rel, f.Line, f.Rule, f.Message))
			}
		}
	}
	text := readRepoFile(t, adr)
	for _, heading := range []string{"Context", "Decision", "Consequences", "Rejected"} {
		if !regexp.MustCompile(`(?m)^## ` + heading).MatchString(text) {
			t.Errorf("RED: %s has no '## %s' section", adr, heading)
		}
	}
}

func goTests(t *testing.T, pkg, pattern string, names ...string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: pkg,
		Pattern: pattern,
		Names:   names,
	})
}

func TestC2375_003_AOnePackageChangeScopesToThatPackageAndItsImporters(t *testing.T) {
	goTests(t, "./internal/changedpkgs",
		"^(TestGateScope_OnePackageChangeRunsThatPackageAndItsImporters|TestGateScope_UncommittedChangeCountsLikeACommittedOne)$",
		"TestGateScope_OnePackageChangeRunsThatPackageAndItsImporters",
		"TestGateScope_UncommittedChangeCountsLikeACommittedOne")
}

func TestC2375_004_EveryUnreadableOrRootLevelChangeFallsBackToTheFullSet(t *testing.T) {
	goTests(t, "./internal/changedpkgs",
		"^(TestGateScope_ModuleRootChangeFallsBackToTheFullSet|TestGateScope_UnderivableChangeFallsBackToTheFullSet|TestGateScope_UnreadableImportGraphFallsBackToTheFullSet)$",
		"TestGateScope_ModuleRootChangeFallsBackToTheFullSet",
		"TestGateScope_UnderivableChangeFallsBackToTheFullSet",
		"TestGateScope_UnreadableImportGraphFallsBackToTheFullSet")
}

func TestC2375_005_NoGoChangeAndAcsOnlyChangeRunNothingAndAcsStaysOutOfTheScope(t *testing.T) {
	goTests(t, "./internal/changedpkgs",
		"^(TestGateScope_ChangeOutsideGoRunsNothing|TestGateScope_AcsOnlyChangeRunsNothing|TestGateScope_ACSPackagesNeverEnterTheScope)$",
		"TestGateScope_ChangeOutsideGoRunsNothing",
		"TestGateScope_AcsOnlyChangeRunsNothing",
		"TestGateScope_ACSPackagesNeverEnterTheScope")
}

func TestC2375_006_TheApicoverEnforceListIsFilteredToTheScope(t *testing.T) {
	goTests(t, "./internal/changedpkgs",
		"^(TestScopeEnforced_KeepsEntriesInsideTheScope|TestScopeEnforced_FullScopeKeepsEveryEntry|TestScopeEnforced_EmptyScopeKeepsNothing)$",
		"TestScopeEnforced_KeepsEntriesInsideTheScope",
		"TestScopeEnforced_FullScopeKeepsEveryEntry",
		"TestScopeEnforced_EmptyScopeKeepsNothing")
}

func TestC2375_007_TheComposedGateRunnerPassesTheScopeToMakeForTestAndApicoverOnly(t *testing.T) {
	goTests(t, "./cmd/evolve",
		"^(TestComposedGateScope_TestGateRunsOnlyChangedAndImporters|TestComposedGateScope_ApicoverGateRunsOnlyEnforcedEntriesInTheClosure|TestComposedGateScope_UnderivableChangeRunsTheFullSet|TestComposedGateScope_ModuleRootChangeRunsTheFullSet|TestComposedGateScope_ChangeOutsideGoSkipsTheScopedGatesAndSaysSo|TestComposedGateScope_CompileAndAcsKeepTheWholeModuleTargets|TestComposedGateScope_MissingEnforceListRunsTheFullApicoverSet)$",
		"TestComposedGateScope_TestGateRunsOnlyChangedAndImporters",
		"TestComposedGateScope_ApicoverGateRunsOnlyEnforcedEntriesInTheClosure",
		"TestComposedGateScope_UnderivableChangeRunsTheFullSet",
		"TestComposedGateScope_ModuleRootChangeRunsTheFullSet",
		"TestComposedGateScope_ChangeOutsideGoSkipsTheScopedGatesAndSaysSo",
		"TestComposedGateScope_CompileAndAcsKeepTheWholeModuleTargets",
		"TestComposedGateScope_MissingEnforceListRunsTheFullApicoverSet")
}

func coverFunc(t *testing.T, importPath, run string) string {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "cover.out")
	if _, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", "-run", run, "-coverprofile", profile, importPath); code != 0 {
		t.Fatalf("RED: go test -cover %s exited %d (%v)\n%s", importPath, code, err, stderr)
	}
	out, stderr, code, err := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+profile)
	if code != 0 {
		t.Fatalf("go tool cover exited %d (%v)\n%s", code, err, stderr)
	}
	return out
}

func coverOfFile(coverOutput, rel string) string {
	var kept []string
	for _, line := range strings.Split(coverOutput, "\n") {
		if strings.HasPrefix(line, changedpkgsPath+"/"+strings.TrimPrefix(rel, "go/internal/changedpkgs/")+":") ||
			strings.HasPrefix(line, cmdEvolvePath+"/"+strings.TrimPrefix(rel, "go/cmd/evolve/")+":") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

func funcsIn(t *testing.T, rel string) map[string][2]int {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join(acsassert.RepoRoot(t), rel), nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("RED: parse %s: %v", rel, err)
	}
	out := map[string][2]int{}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			out[fn.Name.Name] = [2]int{fset.Position(fn.Pos()).Line, fset.Position(fn.End()).Line}
		}
	}
	return out
}

func changedLines(t *testing.T, rel string) [][2]int {
	t.Helper()
	diff, _, code, err := acsassert.SubprocessOutput("git", "-C", acsassert.RepoRoot(t), "diff", "-U0", "main", "--", rel)
	if code != 0 {
		t.Fatalf("git diff main -- %s exited %d: %v", rel, code, err)
	}
	var ranges [][2]int
	for _, line := range strings.Split(diff, "\n") {
		m := hunkHeader.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		if count > 0 {
			ranges = append(ranges, [2]int{start, start + count - 1})
		}
	}
	return ranges
}

func TestC2375_008_EveryChangedFunctionHasFullCoverage(t *testing.T) {
	scopeCover := coverOfFile(coverFunc(t, changedpkgsPath, "^(TestGateScope_|TestScopeEnforced_)"), scopeFile)
	scopeFuncs := funcsIn(t, scopeFile)
	if len(scopeFuncs) == 0 {
		t.Fatalf("RED: %s declares no function", scopeFile)
	}
	for name := range scopeFuncs {
		pct, err := acsassert.FuncCoverage(scopeCover, name)
		if err != nil || pct < requiredCoverage {
			t.Errorf("RED: %s %s is covered %.1f%% by the gate-scope tests (%v), want 100%%", scopeFile, name, pct, err)
		}
	}

	wiringCover := coverOfFile(coverFunc(t, cmdEvolvePath, "^TestComposedGate"), wiringFile)
	changed := changedLines(t, wiringFile)
	touched := 0
	for name, span := range funcsIn(t, wiringFile) {
		for _, r := range changed {
			if r[0] <= span[1] && r[1] >= span[0] {
				touched++
				pct, err := acsassert.FuncCoverage(wiringCover, name)
				if err != nil || pct < requiredCoverage {
					t.Errorf("RED: %s %s is covered %.1f%% by the composed-gate tests (%v), want 100%%", wiringFile, name, pct, err)
				}
				break
			}
		}
	}
	if touched == 0 {
		t.Errorf("RED: %s has no changed function against main: the scope is not wired", wiringFile)
	}
}

func TestC2375_009_TheChangedGoFilesCarryNoComments(t *testing.T) {
	for _, rel := range []string{scopeFile, scopeTestFile, wiringFile, wiringTestFile} {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filepath.Join(acsassert.RepoRoot(t), rel), nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("RED: parse %s: %v", rel, err)
		}
		for _, group := range file.Comments {
			if group == file.Doc {
				continue
			}
			for _, c := range group.List {
				if strings.HasPrefix(c.Text, "//go:") {
					continue
				}
				t.Errorf("RED: %s:%d carries a comment: %q", rel, fset.Position(c.Pos()).Line, c.Text)
			}
		}
	}
}
