//go:build acs

package cycle2638

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitstarts"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	headCoreRecorded    = 730
	coreRecordedRelPath = "go/internal/core/testdata/gitstarts.json"
	guardTestName       = "TestEveryGitMethodHasCost"
	repoKeyPrefix       = "Repo."
)

var requiredCosts = map[string]int{
	"Fixture":          5,
	"Bare":             5,
	"At":               5,
	"Clone":            1,
	"Repo.CloneBare":   1,
	"BranchExists":     1,
	"Repo.Git":         1,
	"FromTemplate":     0,
	"BareFromTemplate": 0,
}

func sampleSource(body string) string {
	return "package sample\n\nfunc sample() {\n" + body + "\n}\n"
}

func writeFile(t *testing.T, dir, name, src string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

func chargedFor(t *testing.T, body string) int {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "sample_test.go", sampleSource(body))
	n, err := gitstarts.Count(dir)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	return n
}

func callFor(key string) string {
	if method, ok := strings.CutPrefix(key, repoKeyPrefix); ok {
		return "r." + method + "(t)"
	}
	return "gittest." + key + "(t)"
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

type gitCounter struct{ log string }

func installGitCounter(t *testing.T) *gitCounter {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("git not on PATH: %v", err)
	}
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "starts.log")
	shim := "#!/bin/sh\necho start >> " + shellQuote(log) + "\nexec " + shellQuote(realGit) + " \"$@\"\n"
	writeFile(t, bin, "git", shim)
	if err := os.Chmod(filepath.Join(bin, "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return &gitCounter{log: log}
}

func (c *gitCounter) reset(t *testing.T) {
	t.Helper()
	if err := os.Remove(c.log); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func (c *gitCounter) starts(t *testing.T) int {
	t.Helper()
	body, err := os.ReadFile(c.log)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(body), "start\n")
}

type measuredHelper struct {
	name   string
	source string
	start  func(t *testing.T, c *gitCounter)
}

func measuredHelpers() []measuredHelper {
	return []measuredHelper{
		{"Fixture", `r := gittest.Fixture(t)`, func(t *testing.T, c *gitCounter) {
			c.reset(t)
			gittest.Fixture(t)
		}},
		{"Bare", `r := gittest.Bare(t)`, func(t *testing.T, c *gitCounter) {
			c.reset(t)
			gittest.Bare(t)
		}},
		{"At", `r := gittest.At(t, dir)`, func(t *testing.T, c *gitCounter) {
			dir := t.TempDir()
			c.reset(t)
			gittest.At(t, dir)
		}},
		{"Clone", `r := gittest.Clone(t, src)`, func(t *testing.T, c *gitCounter) {
			src := gittest.Fixture(t)
			c.reset(t)
			gittest.Clone(t, src.Dir)
		}},
		{"Repo.CloneBare", `r.CloneBare(src, dst)`, func(t *testing.T, c *gitCounter) {
			src := gittest.Fixture(t)
			dst := filepath.Join(t.TempDir(), "bare.git")
			c.reset(t)
			src.CloneBare(src.Dir, dst)
		}},
		{"BranchExists", `gittest.BranchExists(t, dir, branch)`, func(t *testing.T, c *gitCounter) {
			src := gittest.Fixture(t)
			c.reset(t)
			gittest.BranchExists(t, src.Dir, "main")
		}},
		{"Repo.Git", `r.Git("status")`, func(t *testing.T, c *gitCounter) {
			src := gittest.Fixture(t)
			c.reset(t)
			src.Git("status")
		}},
	}
}

func TestC2638_001_EveryGitRunningHelperIsChargedItsMeasuredProcessCount(t *testing.T) {
	counter := installGitCounter(t)
	for _, tc := range measuredHelpers() {
		t.Run(tc.name, func(t *testing.T) {
			tc.start(t, counter)
			measured := counter.starts(t)
			if measured == 0 {
				t.Fatalf("%s: the git counter on PATH saw no start, so the measurement proves nothing", tc.name)
			}
			charged := chargedFor(t, tc.source)
			if charged != measured {
				t.Errorf("%s is charged %d git starts but runs %d real git processes", tc.name, charged, measured)
			}
		})
	}
}

func productionFiles(t *testing.T, dir string) map[string]*ast.File {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files[name] = f
	}
	return files
}

func callsSelector(f *ast.File, pkg, name string) bool {
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != name {
			return true
		}
		if pkg == "" {
			found = true
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == pkg {
			found = true
		}
		return true
	})
	return found
}

func stringCaseClauses(f *ast.File) []string {
	var cases []string
	ast.Inspect(f, func(n ast.Node) bool {
		clause, ok := n.(*ast.CaseClause)
		if !ok {
			return true
		}
		for _, expr := range clause.List {
			if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				cases = append(cases, lit.Value)
			}
		}
		return true
	})
	return cases
}

func TestC2638_002_EveryCostTheCounterChargesIsReadFromTheGittestTable(t *testing.T) {
	costs := gittest.StartCosts()
	for key, want := range requiredCosts {
		got, ok := costs[key]
		if !ok {
			t.Errorf("the gittest cost table has no entry for %q", key)
			continue
		}
		if got != want {
			t.Errorf("the gittest cost table charges %q %d, want %d", key, got, want)
		}
	}
	for key, cost := range costs {
		if charged := chargedFor(t, callFor(key)); charged != cost {
			t.Errorf("table entry %q costs %d but gitstarts.Count charges %d for a call of it", key, cost, charged)
		}
	}
	for name, body := range map[string]string{
		"a gittest name outside the table":   `gittest.NotInTheTable(t)`,
		"a method name outside the table":    `r.NotInTheTable(t)`,
		"slices.Clone is not gittest.Clone":  `c := slices.Clone(names)`,
		"maps.Clone is not gittest.Clone":    `c := maps.Clone(byName)`,
		"strings.Clone is not gittest.Clone": `c := strings.Clone(text)`,
	} {
		if charged := chargedFor(t, body); charged != 0 {
			t.Errorf("%s: charged %d, want 0", name, charged)
		}
	}
	costs["Fixture"] = 99
	if again := gittest.StartCosts()["Fixture"]; again != requiredCosts["Fixture"] {
		t.Errorf("StartCosts hands out its own map: a caller changed Fixture to 99 and the next call returns %d", again)
	}
}

func TestC2638_002b_TheCounterHoldsNoHelperNameSwitchAndCallsTheTable(t *testing.T) {
	files := productionFiles(t, filepath.Join(acsassert.GoModuleDir(t), "internal", "gitstarts"))
	readsTable := false
	for name, f := range files {
		if cases := stringCaseClauses(f); len(cases) > 0 {
			t.Errorf("%s still switches on helper names %v: every cost must come from the gittest table", name, cases)
		}
		readsTable = readsTable || callsSelector(f, "gittest", "StartCosts")
	}
	if !readsTable {
		t.Errorf("no production file of internal/gitstarts calls gittest.StartCosts, so no cost reaches the counter from the table")
	}
}

const guardFixtureSource = `package gittest

import "os/exec"

type Repo struct{ Dir string }

func Fixture() *Repo {
	r := &Repo{}
	r.Git("init")
	return r
}

func (r *Repo) Git(args ...string) string { return "" }

func (r *Repo) Write(rel string) {}

func (r *Repo) CommitAll(message string) {
	r.Git("add", "-A")
	r.Git("commit", "-m", message)
}

func (r *Repo) Rename(from, to string) {
	r.Git("mv", from, to)
}

func RawProbe(dir string) {
	exec.Command("git", "-C", dir, "status")
}

func (r *Repo) quiet() {
	r.Git("config", "k", "v")
}

func unexportedProbe() {
	exec.Command("git", "status")
}
`

const guardFixtureTestSource = `package gittest

import "os/exec"

func ExportedFromATestFile() {
	exec.Command("git", "status")
}
`

func TestC2638_003_TheGuardNamesAnExportedHelperThatStartsGitWithoutACostEntry(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "helper.go", guardFixtureSource)
	writeFile(t, dir, "helper_extra_test.go", guardFixtureTestSource)
	missing, err := gitstarts.MissingCosts(dir)
	if err != nil {
		t.Fatalf("MissingCosts: %v", err)
	}
	for _, want := range []string{"Repo.CommitAll", "Repo.Rename", "RawProbe"} {
		if !slices.Contains(missing, want) {
			t.Errorf("MissingCosts %v does not name %q, an exported helper that starts git with no cost entry", missing, want)
		}
	}
	for _, costed := range []string{"Fixture", "Repo.Git", "Repo.quiet", "quiet", "unexportedProbe", "ExportedFromATestFile"} {
		if slices.Contains(missing, costed) {
			t.Errorf("MissingCosts %v names %q, which has a cost entry, is unexported or lives in a test file", missing, costed)
		}
	}
	if _, err := gitstarts.MissingCosts(filepath.Join(dir, "absent")); err == nil {
		t.Errorf("MissingCosts of a directory that does not exist returned no error")
	}
}

func TestC2638_003b_EveryExportedGittestHelperThatStartsGitHasACostEntry(t *testing.T) {
	missing, err := gitstarts.MissingCosts(filepath.Join(acsassert.GoModuleDir(t), "internal", "gittest"))
	if err != nil {
		t.Fatalf("MissingCosts of internal/gittest: %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("exported gittest helpers that start git have no cost entry: %v", missing)
	}
}

func TestC2638_003c_TheGuardTestRunsFromTheGittestTestBinaryAndCallsTheGuard(t *testing.T) {
	mod := acsassert.GoModuleDir(t)
	gittestDir := filepath.Join(mod, "internal", "gittest")
	entries, err := os.ReadDir(gittestDir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	callsGuard := false
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(gittestDir, e.Name()), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Name.Name == guardTestName && fn.Body != nil {
				callsGuard = callsGuard || callsSelector(&ast.File{Decls: []ast.Decl{fn}}, "", "MissingCosts")
			}
		}
	}
	if !callsGuard {
		t.Errorf("no %s in internal/gittest calls MissingCosts", guardTestName)
	}
	out, errOut, code, _ := acsassert.SubprocessOutput("go", "-C", mod, "test", "-count=1", "-v", "-run", "^"+guardTestName+"$", "./internal/gittest")
	if code != 0 {
		t.Fatalf("go test -run %s ./internal/gittest exited %d:\n%s", guardTestName, code, acsassert.OutputTail(out+errOut, 30))
	}
	if !strings.Contains(out, "--- PASS: "+guardTestName) {
		t.Errorf("the gittest test binary has no passing %s:\n%s", guardTestName, acsassert.OutputTail(out, 20))
	}
}

func TestC2638_004_TheRecordedCeilingStaysAtOrBelowTodayAndTheCoreTestsFitIt(t *testing.T) {
	recordedPath := filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(coreRecordedRelPath))
	body, err := os.ReadFile(recordedPath)
	if err != nil {
		t.Fatalf("recorded count file: %v", err)
	}
	var doc struct {
		Recorded *int `json:"recorded"`
	}
	if err := json.Unmarshal(body, &doc); err != nil || doc.Recorded == nil {
		t.Fatalf("recorded count file %s is not {\"recorded\": N}: %v", coreRecordedRelPath, err)
	}
	recorded := *doc.Recorded
	if recorded < 0 || recorded > headCoreRecorded {
		t.Errorf("recorded %d is outside 0..%d: a raise of the ceiling is a separate reviewed change", recorded, headCoreRecorded)
	}
	coreDir := filepath.Join(acsassert.GoModuleDir(t), "internal", "core")
	real, err := gitstarts.Count(coreDir)
	if err != nil {
		t.Fatalf("Count of the core tests: %v", err)
	}
	if real > recorded {
		t.Errorf("the core tests hold %d git starts, above the recorded %d", real, recorded)
	}
	if err := gitstarts.Verify(coreDir, recordedPath); err != nil {
		t.Errorf("Verify of the real core tests: %v", err)
	}
}

func TestC2638_005_TheNewExportsAreNamedInTheApicoverGate(t *testing.T) {
	mod := acsassert.GoModuleDir(t)
	for _, pkg := range []string{"gitstarts", "gittest"} {
		profile := filepath.Join(t.TempDir(), pkg+"-cover.out")
		out, errOut, code, _ := acsassert.SubprocessOutput("go", "-C", mod, "test", "-count=1", "-coverprofile="+profile, "./internal/"+pkg)
		if code != 0 {
			t.Fatalf("go test ./internal/%s exited %d:\n%s", pkg, code, acsassert.OutputTail(out+errOut, 30))
		}
		funcs, errOut, code, _ := acsassert.SubprocessOutput("go", "-C", mod, "tool", "cover", "-func="+profile)
		if code != 0 {
			t.Fatalf("go tool cover -func exited %d:\n%s", code, errOut)
		}
		funcPath := filepath.Join(t.TempDir(), pkg+"-func.txt")
		writeFile(t, filepath.Dir(funcPath), filepath.Base(funcPath), funcs)
		var report, apicoverErr bytes.Buffer
		if rc := apicover.Main([]string{"-cover", funcPath, "-enforce", filepath.Join(mod, "internal", pkg)}, &report, &apicoverErr); rc != 0 {
			t.Errorf("apicover -enforce on internal/%s exited %d:\n%s%s", pkg, rc, report.String(), apicoverErr.String())
		}
	}
}
