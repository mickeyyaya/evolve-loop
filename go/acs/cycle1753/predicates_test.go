//go:build acs

package cycle1753

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func funcLineSpan(t *testing.T, path, funcName string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, raw, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name.Name != funcName {
			continue
		}
		start := fset.Position(fd.Pos()).Line
		end := fset.Position(fd.End()).Line
		return end - start + 1
	}
	t.Fatalf("function %q not found in %s", funcName, path)
	return -1
}

func TestC1753_001_ApplyCarryoverDecisionsWithinSizeRatchetLimit(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "cmd", "evolve", "cmd_carryover.go")
	if lines := funcLineSpan(t, path, "applyCarryoverDecisions"); lines > 50 {
		t.Errorf("applyCarryoverDecisions is %d lines, want <=50 (go/internal/sizeratchet/offenders.json ceiling)", lines)
	}
}

func TestC1753_002_GCWorkspaceSweepWithinSizeRatchetLimit(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "cmd", "evolve", "cmd_gc.go")
	if lines := funcLineSpan(t, path, "gcWorkspaceSweep"); lines > 50 {
		t.Errorf("gcWorkspaceSweep is %d lines, want <=50 (go/internal/sizeratchet/offenders.json ceiling)", lines)
	}
}

func TestC1753_003_ApplyCarryoverDecisionsCallsExtractedFastPathHelper(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "cmd", "evolve", "cmd_carryover.go")
	n, err := acsassert.CountInGoFunc(path, "applyCarryoverDecisions", "carryoverNoOpFastPath(")
	if err != nil {
		t.Fatalf("CountInGoFunc: %v", err)
	}
	if n == 0 {
		t.Errorf("applyCarryoverDecisions does not call carryoverNoOpFastPath — the pre-read fast path must be extracted into that helper, not merely deleted or inlined elsewhere")
	}
}

func TestC1753_004_GCWorkspaceSweepCallsExtractedProjectRootResolver(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "cmd", "evolve", "cmd_gc.go")
	n, err := acsassert.CountInGoFunc(path, "gcWorkspaceSweep", "resolveGCProjectRoot(")
	if err != nil {
		t.Fatalf("CountInGoFunc: %v", err)
	}
	if n == 0 {
		t.Errorf("gcWorkspaceSweep does not call resolveGCProjectRoot — the project-root resolution block must be extracted into that helper, not merely deleted or inlined elsewhere")
	}
}

func TestC1753_005_OffendersJSONNoLongerListsApplyCarryoverDecisions(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "sizeratchet", "offenders.json")
	acsassert.FileNotContains(t, path, `"cmd/evolve.applyCarryoverDecisions"`)
}

func TestC1753_006_OffendersJSONNoLongerListsGCWorkspaceSweep(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "sizeratchet", "offenders.json")
	acsassert.FileNotContains(t, path, `"cmd/evolve.gcWorkspaceSweep"`)
}

func TestC1753_007_TouchedFilesAreGofmtClean(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{
		filepath.Join("go", "cmd", "evolve", "cmd_carryover.go"),
		filepath.Join("go", "cmd", "evolve", "cmd_gc.go"),
	} {
		path := filepath.Join(root, rel)
		stdout, stderr, code, err := acsassert.SubprocessOutput("gofmt", "-l", path)
		if err != nil {
			t.Fatalf("gofmt -l %s: %v (stderr=%s)", rel, err, stderr)
		}
		if code != 0 {
			t.Fatalf("gofmt -l %s exited %d: %s", rel, code, stderr)
		}
		if stdout != "" {
			t.Errorf("gofmt -l reports %s as unformatted:\n%s", rel, stdout)
		}
	}
}

func TestC1753_008_TouchedPackageVetsClean(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	_, stderr, code, err := acsassert.SubprocessOutput("go", "vet", "-C", goDir, "./cmd/evolve/...")
	if err != nil {
		t.Fatalf("go vet -C %s ./cmd/evolve/...: %v (stderr=%s)", goDir, err, stderr)
	}
	if code != 0 {
		t.Errorf("go vet ./cmd/evolve/... exited %d: %s", code, stderr)
	}
}

const explainDocGlob = "docs/explain/builds/cycle-1753-*.md"

var (
	newTestFiles = []struct{ subject, rel string }{
		{"carryoverNoOpFastPath", "go/cmd/evolve/cmd_carryover_fastpath_test.go"},
		{"resolveGCProjectRoot", "go/cmd/evolve/cmd_gc_projectroot_test.go"},
	}
	evalFiles = []string{
		".evolve/evals/shrink-carryover-apply.md",
		".evolve/evals/shrink-gc-workspace-sweep.md",
	}

	tableClaimRE   = regexp.MustCompile(`(?i)\btable[- ]?driven\b|\b(?:case|cases|test)[- ]tables?\b|\btables? of (?:test )?cases\b|\btabular\b`)
	claimNegatedRE = regexp.MustCompile(`(?i)\b(?:no|not|non|without|never|rather than|instead of)(?:[\s-]+\w+){0,2}?[\s-]*$`)
	testCountRE    = regexp.MustCompile("(?i)\\b(one|two|three|four|five|six|seven|eight|nine|ten|\\d+)\\s+(?:[\\w`+-]+\\s+){0,2}?(?:tests|`?test`?\\s+functions)\\b")
	blockStartRE   = regexp.MustCompile(`^(?:[-*|#]|\d+\.\s)`)
	countWords     = map[string]int{"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10}
)

type testFileShape struct {
	subject, rel string
	testFuncs    int
	tableDriven  bool
}

func observeTestFileShape(t *testing.T, root, subject, rel string) testFileShape {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, filepath.FromSlash(rel)), nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
	shape := testFileShape{subject: subject, rel: rel}
	for _, decl := range f.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Recv == nil && strings.HasPrefix(fd.Name.Name, "Test") {
			shape.testFuncs++
		}
	}
	structTypes := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok {
			if _, isStruct := ts.Type.(*ast.StructType); isStruct {
				structTypes[ts.Name.Name] = true
			}
		}
		return true
	})
	ast.Inspect(f, func(n ast.Node) bool {
		if lit, ok := n.(*ast.CompositeLit); ok && isCaseTableType(lit.Type, structTypes) {
			shape.tableDriven = true
		}
		return true
	})
	if shape.testFuncs == 0 {
		t.Fatalf("%s declares no Test functions, so its shape cannot be observed", rel)
	}
	return shape
}

func isCaseTableType(expr ast.Expr, structTypes map[string]bool) bool {
	var elem ast.Expr
	switch typ := expr.(type) {
	case *ast.ArrayType:
		elem = typ.Elt
	case *ast.MapType:
		elem = typ.Value
	default:
		return false
	}
	if star, ok := elem.(*ast.StarExpr); ok {
		elem = star.X
	}
	switch e := elem.(type) {
	case *ast.StructType:
		return true
	case *ast.Ident:
		return structTypes[e.Name]
	}
	return false
}

func docBlocks(text string) []string {
	var blocks, cur []string
	flush := func() {
		if len(cur) > 0 {
			blocks = append(blocks, strings.Join(cur, " "))
			cur = nil
		}
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), ">"))
		switch {
		case line == "":
			flush()
		case blockStartRE.MatchString(line):
			flush()
			cur = append(cur, line)
		default:
			cur = append(cur, line)
		}
	}
	flush()
	return blocks
}

func mentionsTestFile(block string, s testFileShape) bool {
	return strings.Contains(block, s.subject) || strings.Contains(block, path.Base(s.rel))
}

func assertedTableClaim(block string) string {
	for _, loc := range tableClaimRE.FindAllStringIndex(block, -1) {
		if !claimNegatedRE.MatchString(block[max(0, loc[0]-30):loc[0]]) {
			return block[loc[0]:loc[1]]
		}
	}
	return ""
}

func claimedTestCounts(block string) []int {
	var counts []int
	for _, m := range testCountRE.FindAllStringSubmatch(block, -1) {
		if n, ok := countWords[strings.ToLower(m[1])]; ok {
			counts = append(counts, n)
		} else if n, err := strconv.Atoi(m[1]); err == nil {
			counts = append(counts, n)
		}
	}
	return counts
}

func testShapeClaimFindings(rel, text string, shapes []testFileShape) []string {
	var findings []string
	for _, block := range docBlocks(text) {
		var named []testFileShape
		for _, s := range shapes {
			if mentionsTestFile(block, s) {
				named = append(named, s)
			}
		}
		if claim := assertedTableClaim(block); claim != "" {
			for _, s := range named {
				if !s.tableDriven {
					findings = append(findings, fmt.Sprintf("%s calls the tests of %s %q, but %s declares %d standalone Test functions and no case table: %q",
						rel, s.subject, claim, s.rel, s.testFuncs, block))
				}
			}
		}
		if len(named) != 1 {
			continue
		}
		for _, n := range claimedTestCounts(block) {
			if n != named[0].testFuncs {
				findings = append(findings, fmt.Sprintf("%s claims %d tests for %s, but %s declares %d Test functions: %q",
					rel, n, named[0].subject, named[0].rel, named[0].testFuncs, block))
			}
		}
	}
	return findings
}

func testFileOmissionFindings(rel, doc string, shapes []testFileShape) []string {
	var findings []string
	for _, s := range shapes {
		described := false
		for _, block := range docBlocks(doc) {
			if strings.Contains(block, path.Base(s.rel)) {
				described = true
			}
		}
		if !described {
			findings = append(findings, fmt.Sprintf("%s no longer describes %s; its inaccurate wording must be corrected, not deleted", rel, s.rel))
		}
	}
	return findings
}

func readTrackedExplanationDoc(t *testing.T, root string) (string, string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(explainDocGlob)))
	if err != nil || len(matches) != 1 {
		t.Fatalf("want exactly one %s, got %v (err=%v)", explainDocGlob, matches, err)
	}
	rel, err := filepath.Rel(root, matches[0])
	if err != nil {
		t.Fatalf("rel path of %s: %v", matches[0], err)
	}
	rel = filepath.ToSlash(rel)
	if _, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("RED: %s is untracked, so it would not ship (exit=%d): %v\n%s", rel, code, err, stderr)
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return rel, string(raw)
}

func TestC1753_009_ExplanationDocAndEvalsDescribeNewTestFilesAsDeclared(t *testing.T) {
	root := acsassert.RepoRoot(t)
	var shapes []testFileShape
	for _, f := range newTestFiles {
		s := observeTestFileShape(t, root, f.subject, f.rel)
		t.Logf("%s: %d Test functions, case table=%v", s.rel, s.testFuncs, s.tableDriven)
		shapes = append(shapes, s)
	}
	docRel, doc := readTrackedExplanationDoc(t, root)
	findings := testShapeClaimFindings(docRel, doc, shapes)
	findings = append(findings, testFileOmissionFindings(docRel, doc, shapes)...)
	for _, rel := range evalFiles {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		findings = append(findings, testShapeClaimFindings(rel, string(raw), shapes)...)
	}
	for _, f := range findings {
		t.Errorf("RED: %s", f)
	}
}

var (
	verificationHeadingRE = regexp.MustCompile(`(?m)^##\s+Verification\s*$`)
	predicateCountRE      = regexp.MustCompile("(?i)\\b(one|two|three|four|five|six|seven|eight|nine|ten|\\d+)\\s+(?:ACS\\s+)?predicates\\b")
	suiteFieldRE          = regexp.MustCompile(`\b(verdict|green|red|skip|total)=(\w+)`)
	suiteScopesRE         = regexp.MustCompile(`\(cycle=(\d+)[,;]?\s+regression=(\d+)[,;]?\s+red-team=(\d+)\)`)
)

func declaredPredicates(t *testing.T, root, glob string) int {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(glob)))
	if err != nil || len(files) == 0 {
		t.Fatalf("no predicate files match %s (err=%v)", glob, err)
	}
	n := 0
	for _, file := range files {
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || fd.Name.Name == "TestMain" || !strings.HasPrefix(fd.Name.Name, "Test") {
				continue
			}
			suffix := strings.TrimPrefix(fd.Name.Name, "Test")
			if first, _ := utf8.DecodeRuneInString(suffix); suffix != "" && unicode.IsLower(first) {
				continue
			}
			n++
		}
	}
	return n
}

func verificationSection(doc string) (string, bool) {
	loc := verificationHeadingRE.FindStringIndex(doc)
	if loc == nil {
		return "", false
	}
	section := doc[loc[1]:]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	return section, true
}

func suiteClaimFindings(rel, line string, cyclePredicates int) []string {
	fields := map[string]string{}
	for _, m := range suiteFieldRE.FindAllStringSubmatch(line, -1) {
		fields[m[1]] = m[2]
	}
	nums := map[string]int{}
	for _, k := range []string{"green", "red", "skip", "total"} {
		n, err := strconv.Atoi(fields[k])
		if err != nil {
			return []string{fmt.Sprintf("%s states the evolve acs suite result without a numeric %s= figure: %q", rel, k, line)}
		}
		nums[k] = n
	}
	scopes := suiteScopesRE.FindStringSubmatch(line)
	if scopes == nil {
		return []string{fmt.Sprintf("%s states suite total=%d without the (cycle= regression= red-team=) breakdown that evolve acs suite prints, so the total is not tied to the %d predicates go/acs/cycle1753 declares: %q",
			rel, nums["total"], cyclePredicates, line)}
	}
	cycle, _ := strconv.Atoi(scopes[1])
	regression, _ := strconv.Atoi(scopes[2])
	redTeam, _ := strconv.Atoi(scopes[3])
	var findings []string
	if cycle != cyclePredicates {
		findings = append(findings, fmt.Sprintf("%s claims a suite run with cycle=%d, but go/acs/cycle1753 declares %d predicates: %q", rel, cycle, cyclePredicates, line))
	}
	if sum := nums["green"] + nums["red"] + nums["skip"]; sum != nums["total"] {
		findings = append(findings, fmt.Sprintf("%s claims green+red+skip=%d but total=%d: %q", rel, sum, nums["total"], line))
	}
	if sum := cycle + regression + redTeam; sum != nums["total"] {
		findings = append(findings, fmt.Sprintf("%s claims cycle+regression+red-team=%d but total=%d: %q", rel, sum, nums["total"], line))
	}
	if verdict, ok := fields["verdict"]; ok && (verdict == "PASS") != (nums["red"] == 0) {
		findings = append(findings, fmt.Sprintf("%s claims verdict=%s with red=%d: %q", rel, verdict, nums["red"], line))
	}
	return findings
}

func verificationCountFindings(rel, doc string, cyclePredicates int) []string {
	section, ok := verificationSection(doc)
	if !ok {
		return []string{rel + " has no ## Verification section"}
	}
	var findings []string
	var cycleClaims, suiteClaims int
	for _, line := range strings.Split(section, "\n") {
		if strings.Contains(line, "acs/cycle1753") {
			for _, m := range predicateCountRE.FindAllStringSubmatch(line, -1) {
				n, known := countWords[strings.ToLower(m[1])]
				if !known {
					n, _ = strconv.Atoi(m[1])
				}
				cycleClaims++
				if n != cyclePredicates {
					findings = append(findings, fmt.Sprintf("%s claims %d cycle predicates, but go/acs/cycle1753 declares %d: %q", rel, n, cyclePredicates, line))
				}
			}
		}
		if strings.Contains(line, "acs suite") {
			suiteClaims++
			findings = append(findings, suiteClaimFindings(rel, line, cyclePredicates)...)
		}
	}
	if cycleClaims == 0 {
		findings = append(findings, rel+" Verification no longer states how many go/acs/cycle1753 predicates passed")
	}
	if suiteClaims == 0 {
		findings = append(findings, rel+" Verification no longer states the evolve acs suite result")
	}
	return findings
}

func TestC1753_010_ExplanationDocVerificationCountsMatchDeclaredPredicates(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cyclePredicates := declaredPredicates(t, root, "go/acs/cycle1753/*_test.go")
	t.Logf("go/acs/cycle1753 declares %d predicates", cyclePredicates)
	docRel, doc := readTrackedExplanationDoc(t, root)
	for _, f := range verificationCountFindings(docRel, doc, cyclePredicates) {
		t.Errorf("RED: %s", f)
	}
}
