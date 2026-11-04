//go:build acs

package cycle2150

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const auditPkg = "./internal/phases/audit"

const minimumSkillsDriftTests = 19

var skillsDriftTestName = regexp.MustCompile(`^Test(WorktreeSkillsDrift|SkillsDriftGate)_`)

var alreadyCoveredByWiderRun = map[string]bool{
	"worktreeFailureOffenders": true,
	"skillsCheckOffenders":     true,
}

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func auditDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(goDir(t), "internal", "phases", "audit")
}

type auditFile struct {
	file        *ast.File
	integration bool
}

func parseAuditFiles(t *testing.T, pattern string) []auditFile {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(auditDir(t), pattern))
	if err != nil || len(paths) == 0 {
		t.Fatalf("glob %s in %s: %v (%d files)", pattern, auditDir(t), err, len(paths))
	}
	fset := token.NewFileSet()
	var files []auditFile
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		f, err := parser.ParseFile(fset, p, raw, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		files = append(files, auditFile{file: f, integration: strings.Contains(string(raw), "//go:build integration")})
	}
	return files
}

func skillsDriftTests(t *testing.T) map[string]*ast.FuncDecl {
	t.Helper()
	found, _ := skillsDriftTestsByTier(t)
	return found
}

func skillsDriftTestsByTier(t *testing.T) (all map[string]*ast.FuncDecl, untagged []string) {
	t.Helper()
	found := map[string]*ast.FuncDecl{}
	for _, af := range parseAuditFiles(t, "*_test.go") {
		for _, decl := range af.file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if ok && fd.Recv == nil && fd.Body != nil && skillsDriftTestName.MatchString(fd.Name.Name) {
				found[fd.Name.Name] = fd
				if !af.integration {
					untagged = append(untagged, fd.Name.Name)
				}
			}
		}
	}
	sort.Strings(untagged)
	if len(found) < minimumSkillsDriftTests {
		t.Fatalf("found %d skills-drift tests in %s, want at least %d: the name pattern %s matched too little to be a proof", len(found), auditDir(t), minimumSkillsDriftTests, skillsDriftTestName)
	}
	return found, untagged
}

func sortedNames(set map[string]*ast.FuncDecl) []string {
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func startsWithParallel(fd *ast.FuncDecl) bool {
	if len(fd.Body.List) == 0 {
		return false
	}
	stmt, ok := fd.Body.List[0].(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := stmt.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "Parallel"
}

func serialTestKeys(t *testing.T) []string {
	t.Helper()
	var keys []string
	for _, af := range parseAuditFiles(t, "parallel_serial_tests_test.go") {
		ast.Inspect(af.file, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if key, ok := kv.Key.(*ast.BasicLit); ok && key.Kind == token.STRING {
					if name, err := strconv.Unquote(key.Value); err == nil {
						keys = append(keys, name)
					}
				}
			}
			return true
		})
	}
	return keys
}

func TestC2150_001_EverySkillsDriftTestCallsTParallelFirst(t *testing.T) {
	tests := skillsDriftTests(t)
	var serial []string
	for _, name := range sortedNames(tests) {
		if !startsWithParallel(tests[name]) {
			serial = append(serial, name)
		}
	}
	if len(serial) > 0 {
		t.Errorf("%d of %d skills-drift tests do not call t.Parallel as their first statement:\n  %s", len(serial), len(tests), strings.Join(serial, "\n  "))
	}
}

func TestC2150_002_TheSerialListNamesNoSkillsDriftTest(t *testing.T) {
	keys := serialTestKeys(t)
	if len(keys) == 0 {
		t.Fatalf("serialTests lists no test at all: the list moved or the parse missed it, so an empty answer proves nothing")
	}
	var offenders []string
	for _, key := range keys {
		if strings.Contains(key, "SkillsDrift") {
			offenders = append(offenders, key)
		}
	}
	if len(offenders) > 0 {
		t.Errorf("serialTests still names %d skills-drift tests:\n  %s", len(offenders), strings.Join(offenders, "\n  "))
	}
}

type testEvent struct {
	Action string
	Test   string
}

func TestC2150_003_SkillsDriftTestsRunInParallelAndPassUnderRaceCountShuffle(t *testing.T) {
	_, untagged := skillsDriftTestsByTier(t)
	if len(untagged) < 9 {
		t.Fatalf("found %d skills-drift tests in the default build, want at least 9", len(untagged))
	}
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", goDir(t), "test", "-race", "-count=3", "-shuffle=on", "-timeout=600s", "-json",
		"-run", "SkillsDrift|ParallelDefault", auditPkg)
	if err != nil || code != 0 {
		t.Fatalf("go test -race -count=3 -shuffle=on %s exit=%d: %v\n%s", auditPkg, code, err, acsassert.OutputTail(out+errOut, 40))
	}
	pauses := map[string]int{}
	passes := map[string]int{}
	scanner := bufio.NewScanner(strings.NewReader(out))
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		ev, ok := decodeEvent(scanner.Text())
		if !ok || strings.Contains(ev.Test, "/") {
			continue
		}
		switch ev.Action {
		case "pause":
			pauses[ev.Test]++
		case "pass":
			passes[ev.Test]++
		}
	}
	for _, name := range untagged {
		if passes[name] != 3 {
			t.Errorf("%s passed %d times in the -count=3 stream, want 3", name, passes[name])
		}
		if pauses[name] != 3 {
			t.Errorf("%s paused %d times in the -count=3 stream, want 3: it did not run as a parallel test", name, pauses[name])
		}
	}
}

var eventPattern = regexp.MustCompile(`"Action":"([a-z]+)"(?:,"Package":"[^"]*")?(?:,"Test":"([^"]*)")?`)

func decodeEvent(line string) (testEvent, bool) {
	m := eventPattern.FindStringSubmatch(line)
	if m == nil || m[2] == "" {
		return testEvent{}, false
	}
	return testEvent{Action: m[1], Test: m[2]}, true
}

var coverRow = regexp.MustCompile(`skillsdrift\.go:\d+:\s+(\w+)\s+(\d+\.\d)%`)

func TestC2150_004_EveryChangedSkillsDriftFunctionStaysFullyCovered(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "cover.out")
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", goDir(t), "test", "-tags", "integration", "-count=1", "-timeout=600s",
		"-run", "SkillsDrift", "-coverprofile="+profile, auditPkg)
	if err != nil || code != 0 {
		t.Fatalf("go test -tags integration -run SkillsDrift exit=%d: %v\n%s", code, err, acsassert.OutputTail(out+errOut, 40))
	}
	cover, coverErr, code, err := acsassert.SubprocessOutput("go", "-C", goDir(t), "tool", "cover", "-func="+profile)
	if err != nil || code != 0 {
		t.Fatalf("go tool cover exit=%d: %v\n%s", code, err, coverErr)
	}
	rows := coverRow.FindAllStringSubmatch(cover, -1)
	if len(rows) < 5 {
		t.Fatalf("skillsdrift.go lists %d functions in the cover profile, want at least 5\n%s", len(rows), acsassert.OutputTail(cover, 20))
	}
	for _, name := range []string{"worktreeSkillsDrift", "skillsDriftCheckDefault"} {
		pct, err := acsassert.FuncCoverage(cover, name)
		if err != nil {
			t.Errorf("%v", err)
			continue
		}
		if pct != 100.0 {
			t.Errorf("%s is covered %.1f%%, want 100.0%%", name, pct)
		}
	}
	for _, r := range rows {
		if !alreadyCoveredByWiderRun[r[1]] && r[2] != "100.0" {
			t.Errorf("%s is covered %s%% in skillsdrift.go, want 100.0%%", r[1], r[2])
		}
	}
}
