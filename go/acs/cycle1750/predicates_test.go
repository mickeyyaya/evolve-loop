//go:build acs

// Package cycle1750 pins the shrink of dag.Levels to the size ratchet, and keeps the already-shrunk cyclehealth.Check and naminguard.Fix as they are.
package cycle1750

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	baseCommit     = "d44676280fb88a3ac1aa5dbc8f17d53da64d53c6"
	offendersRel   = "go/internal/sizeratchet/offenders.json"
	dagDir         = "internal/dag"
	cyclehealthDir = "internal/cyclehealth"
	naminguardDir  = "pkg/naminguard"
	mutantWorkers  = 4
)

var packageDirs = []string{dagDir, cyclehealthDir, naminguardDir}

type target struct {
	dir, file, fn string
	allowance     int
}

func (tg target) key() string { return tg.dir + "." + tg.fn }

var (
	levelsTarget = target{dir: dagDir, file: "dag.go", fn: "Levels", allowance: 57}
	checkTarget  = target{dir: cyclehealthDir, file: "cyclehealth.go", fn: "Check", allowance: 51}
	fixTarget    = target{dir: naminguardDir, file: "naminguard.go", fn: "Fix", allowance: 51}
	targets      = []target{levelsTarget, checkTarget, fixTarget}
)

var protectedPrefixes = []string{
	"go/cmd/evolve",
	"go/internal/core",
	"go/internal/bridge",
	"go/internal/acssuite",
	"go/acs/regression",
	"go/internal/subagent",
	"go/internal/phases/audit",
	"go/internal/phases/ship",
	"go/internal/config",
	"go/internal/adapters/bridge",
	"go/internal/guards",
	"go/internal/binaryguard",
}

func goModuleDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func offendersPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(offendersRel))
}

func walkModule(t *testing.T) []sizeratchet.FuncSpan {
	t.Helper()
	spans, err := sizeratchet.Walk(goModuleDir(t))
	if err != nil {
		t.Fatalf("sizeratchet.Walk: %v", err)
	}
	return spans
}

func TestC1750_001_ThreeTargetFunctionsFitTheRatchetLimit(t *testing.T) {
	sizes := map[string]int{}
	for _, s := range walkModule(t) {
		if s.Lines > sizes[s.Key] {
			sizes[s.Key] = s.Lines
		}
	}
	var over []string
	for _, tg := range targets {
		n, found := sizes[tg.key()]
		if !found {
			t.Errorf("RED: %s not found by sizeratchet.Walk — renamed, removed, or moved out of %s", tg.key(), tg.dir)
			continue
		}
		if n > sizeratchet.MaxLines {
			over = append(over, tg.key()+" is "+strconv.Itoa(n)+" lines")
		}
	}
	if len(over) > 0 {
		t.Errorf("RED: functions still exceed the %d-line ratchet limit:\n  %s", sizeratchet.MaxLines, strings.Join(over, "\n  "))
	}
}

func TestC1750_002_OffendersJSONLeftUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	diff, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", baseCommit, "--", offendersRel)
	if code != 0 {
		t.Fatalf("git diff against %s failed: %v\n%s", baseCommit, err, stderr)
	}
	if strings.TrimSpace(diff) != "" {
		t.Errorf("RED: %s edited since %s — an allowance is a ceiling and a lane never edits the file; a shrunk function's entry is slack the boundary tighten removes:\n%s", offendersRel, baseCommit, diff)
	}
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	for _, tg := range targets {
		if got, listed := offenders[tg.key()]; !listed || got != tg.allowance {
			t.Errorf("RED: offenders.json %s = %d (listed=%v), want the untouched allowance %d", tg.key(), got, listed, tg.allowance)
		}
	}
}

func TestC1750_003_ModuleWideRatchetCheckPasses(t *testing.T) {
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	if err := sizeratchet.Check(walkModule(t), offenders); err != nil {
		t.Errorf("RED: %v", err)
	}
}

func TestC1750_004_BaselineTestFilesUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, dir := range packageDirs {
		names, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-status", "--no-renames", "--diff-filter=a", baseCommit, "--", "go/"+dir+"/*_test.go")
		if code != 0 {
			t.Fatalf("git diff against %s failed: %v\n%s", baseCommit, err, stderr)
		}
		if strings.TrimSpace(names) != "" {
			t.Errorf("RED: baseline %s test files modified or deleted since %s — put new characterization tests in new _test.go files:\n%s", dir, baseCommit, names)
		}
	}
}

func TestC1750_005_TargetPackageTestsPass(t *testing.T) {
	for _, dir := range packageDirs {
		if r := execGo(goModuleDir(t), "test", "-count=1", "./"+dir); r.err != nil || r.code != 0 {
			t.Errorf("RED: go test -count=1 ./%s failed (exit=%d, err=%v):\n%s", dir, r.code, r.err, r.out)
		}
	}
}

func TestC1750_006_TargetPackagesVetAndGofmtClean(t *testing.T) {
	goDir := goModuleDir(t)
	for _, dir := range packageDirs {
		if r := execGo(goDir, "vet", "./"+dir); r.err != nil || r.code != 0 {
			t.Errorf("RED: go vet ./%s failed (exit=%d, err=%v):\n%s", dir, r.code, r.err, r.out)
		}
		entries, err := os.ReadDir(filepath.Join(goDir, dir))
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			path := filepath.Join(goDir, dir, e.Name())
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			formatted, err := format.Source(src)
			if err != nil {
				t.Errorf("RED: %s/%s does not parse for gofmt: %v", dir, e.Name(), err)
				continue
			}
			if !bytes.Equal(formatted, src) {
				t.Errorf("RED: %s/%s is not gofmt-formatted", dir, e.Name())
			}
		}
	}
}

func TestC1750_007_NoCommentLinesAdded(t *testing.T) {
	git := worktreeGit{root: acsassert.RepoRoot(t)}
	changed, err := git.ChangedFiles(baseCommit)
	if err != nil {
		t.Fatalf("list changed files: %v", err)
	}
	args := []string{"comments", "-base", baseCommit}
	var scoped []string
	for _, dir := range packageDirs {
		for _, rel := range changed {
			if strings.HasPrefix(rel, "go/"+dir+"/") && strings.HasSuffix(rel, ".go") {
				scoped = append(scoped, dir)
				args = append(args, filepath.Join(goModuleDir(t), dir))
				break
			}
		}
	}
	if len(scoped) == 0 {
		return
	}
	var out, errOut strings.Builder
	if code := commentaudit.Main(args, &out, &errOut, git); code != 0 {
		t.Errorf("RED: commentaudit comments exit=%d — the diff adds comment lines under %s (names carry the intent):\n%s%s", code, strings.Join(scoped, ", "), out.String(), errOut.String())
	}
}

// acs-predicate: config-check — comment text IS the contract under test
// (comments have no runtime behavior); graded by commentaudit, not grepped.
func TestC1750_008_DagSourceChangedWithoutLosingAComment(t *testing.T) {
	root := acsassert.RepoRoot(t)
	git := worktreeGit{root: root}
	changed, err := git.ChangedFiles(baseCommit)
	if err != nil {
		t.Fatalf("list changed files: %v", err)
	}
	var baseline, current []byte
	scoped := 0
	for _, rel := range changed {
		if filepath.Dir(rel) != "go/"+dagDir || !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			continue
		}
		scoped++
		before, err := git.Show(baseCommit, rel)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("git show %s:%s: %v", baseCommit, rel, err)
		}
		after, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("read %s: %v", rel, err)
		}
		baseline = append(append(baseline, before...), '\n')
		current = append(append(current, after...), '\n')
	}
	if scoped == 0 {
		t.Fatalf("RED: no non-test Go file under %s changed since %s — the extraction has not happened", dagDir, baseCommit)
	}
	if deleted := commentaudit.AddedComments(current, baseline); len(deleted) > 0 {
		t.Errorf("RED: %d baseline comment line(s) deleted from %s since %s — move each with its code, never strip it to save a line:\n  %s", len(deleted), dagDir, baseCommit, strings.Join(deleted, "\n  "))
	}
}

type mutant struct{ name, anchor, replacement string }

var levelsMutants = []mutant{
	{"a self-dependency is reported as a cycle", `if d == node {`, `if false {`},
	{"a dangling reference is reported as a cycle", `if !known[d] {`, `if false {`},
	{"a dangling-reference error swaps its node names", `"dag: node %q depends on unknown node %q", node, d)`, `"dag: node %q depends on unknown node %q", d, node)`},
	{"an unknown dependency key's error names no key", `"dag: dependency key %q is not a known node", node)`, `"dag: dependency key %q is not a known node", "")`},
	{"the cycle error swaps its counts", `placed, len(indegree))`, `len(indegree), placed)`},
	{"a duplicate node name counts twice", `if placed != len(indegree) {`, `if placed != len(nodes) {`},
	{"a cycle beside a leveled node goes unreported", `if placed != len(indegree) {`, `if placed == 0 && len(indegree) > 0 {`},
	{"a cyclic graph returns its partial levels", `return nil, fmt.Errorf("dag: graph has a cycle`, `return levels, fmt.Errorf("dag: graph has a cycle`},
	{"an empty graph levels to a non-nil empty slice", `var levels [][]string`, `levels := [][]string{}`},
}

func TestC1750_009_LevelsCharacterizationKillsBaselineMutants(t *testing.T) {
	assertBaselineMutantsKilled(t, levelsTarget, levelsMutants)
}

const equivalenceTest = "TestACS1750LevelsMatchesBaseline"

const equivalenceHarness = `package dag

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

type acs1750Case struct {
	nodes []string
	deps  map[string][]string
}

func acs1750CloneList(v []string) []string {
	if v == nil {
		return nil
	}
	return append(make([]string, 0, len(v)), v...)
}

func acs1750Clone(c acs1750Case) acs1750Case {
	out := acs1750Case{nodes: acs1750CloneList(c.nodes)}
	if c.deps != nil {
		out.deps = make(map[string][]string, len(c.deps))
		for k, v := range c.deps {
			out.deps[k] = acs1750CloneList(v)
		}
	}
	return out
}

func acs1750Err(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}

func acs1750Corpus() []acs1750Case {
	names := []string{"a", "b", "c", "d"}
	var pairs [][2]string
	for _, from := range names {
		for _, to := range names {
			if from != to {
				pairs = append(pairs, [2]string{from, to})
			}
		}
	}
	cases := []acs1750Case{
		{nil, nil},
		{[]string{}, map[string][]string{}},
		{nil, map[string][]string{"a": nil}},
		{[]string{"a", "a"}, nil},
		{[]string{"b", "a", "b"}, map[string][]string{"a": {"b"}}},
	}
	for mask := 0; mask < 1<<len(pairs); mask++ {
		deps := map[string][]string{}
		for i, p := range pairs {
			if mask&(1<<i) != 0 {
				deps[p[0]] = append(deps[p[0]], p[1])
			}
		}
		cases = append(cases, acs1750Case{names, deps})
		variant := map[string][]string{}
		switch mask % 4 {
		case 1:
			cases = append(cases, acs1750Case{[]string{"d", "c", "b", "a", "c"}, deps})
		case 2:
			for _, n := range names {
				variant[n] = append([]string{}, deps[n]...)
			}
			cases = append(cases, acs1750Case{names, variant})
		case 3:
			for k, v := range deps {
				variant[k] = append(acs1750CloneList(v), v...)
			}
			cases = append(cases, acs1750Case{names, variant})
		}
	}
	refs := []string{"a", "b", "x"}
	var lists [][]string
	var grow func(prefix []string)
	grow = func(prefix []string) {
		lists = append(lists, prefix)
		if len(prefix) == 3 {
			return
		}
		for _, r := range refs {
			grow(append(acs1750CloneList(prefix), r))
		}
	}
	grow(nil)
	for _, key := range refs {
		for _, list := range lists {
			cases = append(cases, acs1750Case{[]string{"a", "b"}, map[string][]string{key: list}})
		}
	}
	return cases
}

func TestACS1750LevelsMatchesBaseline(t *testing.T) {
	mismatches := 0
	for _, c := range acs1750Corpus() {
		input, base := acs1750Clone(c), acs1750Clone(c)
		got, gotErr := Levels(input.nodes, input.deps)
		want, wantErr := acs1750BaselineLevels(base.nodes, base.deps)
		if !reflect.DeepEqual(input, acs1750Clone(c)) {
			t.Errorf("Levels(%q, %q) mutated its input to (%q, %q)", c.nodes, c.deps, input.nodes, input.deps)
		}
		if reflect.DeepEqual(got, want) && acs1750Err(gotErr) == acs1750Err(wantErr) {
			continue
		}
		mismatches++
		if mismatches <= 10 {
			t.Errorf("Levels(%q, %q) = %q, %s; baseline %q, %s", c.nodes, c.deps, got, acs1750Err(gotErr), want, acs1750Err(wantErr))
		}
	}
	if mismatches > 10 {
		t.Errorf("... and %d more inputs where Levels differs from its baseline", mismatches-10)
	}
}

`

func TestC1750_010_LevelsMatchesBaselineOverAnExhaustiveCorpus(t *testing.T) {
	baseline := baselineFuncSource(t, levelsTarget)
	renamed := strings.Replace(baseline, "func Levels(", "func acs1750BaselineLevels(", 1)
	if renamed == baseline {
		t.Fatalf("baseline %s has no func Levels( header to rename", levelsTarget.file)
	}
	tmp := t.TempDir()
	harness := filepath.Join(tmp, "zz_acs1750_equivalence_test.go")
	if err := os.WriteFile(harness, []byte(equivalenceHarness+renamed+"\n"), 0o644); err != nil {
		t.Fatalf("write harness: %v", err)
	}
	pkgDir := filepath.Join(goModuleDir(t), dagDir)
	overlay := writeOverlay(t, tmp, map[string]string{filepath.Join(pkgDir, "zz_acs1750_equivalence_test.go"): harness})
	r := execGo(goModuleDir(t), "test", "-count=1", "-json", "-run", "^"+equivalenceTest+"$", "-overlay", overlay, "./"+dagDir)
	if r.err != nil {
		t.Fatal(r.err)
	}
	if r.code != 0 || !testPassed(r.out, equivalenceTest) {
		t.Errorf("RED: Levels no longer matches its baseline (exit=%d) — the extraction changed behavior:\n%s", r.code, tail(r.out))
	}
}

func TestC1750_011_AlreadyShrunkPackagesLeftUntouched(t *testing.T) {
	changed, err := worktreeGit{root: acsassert.RepoRoot(t)}.ChangedFiles(baseCommit)
	if err != nil {
		t.Fatalf("list changed files: %v", err)
	}
	var touched []string
	for _, rel := range changed {
		for _, dir := range []string{cyclehealthDir, naminguardDir} {
			if strings.HasPrefix(rel, "go/"+dir+"/") {
				touched = append(touched, rel)
			}
		}
	}
	if len(touched) > 0 {
		t.Errorf("RED: %s and %s already fit the ratchet at %s (cycle 1749 shrank Check to 33 and Fix to 37 lines); the lane changed them anyway:\n  %s", checkTarget.key(), fixTarget.key(), baseCommit, strings.Join(touched, "\n  "))
	}
}

func TestC1750_012_NoProtectedSurfaceTouched(t *testing.T) {
	changed, err := worktreeGit{root: acsassert.RepoRoot(t)}.ChangedFiles(baseCommit)
	if err != nil {
		t.Fatalf("list changed files: %v", err)
	}
	var touched []string
	for _, rel := range changed {
		for _, prefix := range protectedPrefixes {
			if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
				touched = append(touched, rel)
			}
		}
	}
	if len(touched) > 0 {
		t.Errorf("RED: the lane touched protected surfaces since %s:\n  %s", baseCommit, strings.Join(touched, "\n  "))
	}
}

func assertBaselineMutantsKilled(t *testing.T, tg target, mutants []mutant) {
	t.Helper()
	baseline := baselineFuncSource(t, tg)
	site := currentFuncSite(t, tg)
	bodies := []string{baseline}
	for _, m := range mutants {
		if n := strings.Count(baseline, m.anchor); n != 1 {
			t.Fatalf("mutation anchor %q (%s) occurs %d times in the baseline %s, want exactly 1", m.anchor, m.name, n, tg.key())
		}
		bodies = append(bodies, strings.Replace(baseline, m.anchor, m.replacement, 1))
	}
	outcomes := make([]goRun, len(bodies))
	sem := make(chan struct{}, mutantWorkers)
	var wg sync.WaitGroup
	for i, body := range bodies {
		overlay := site.overlay(t, body)
		wg.Add(1)
		go func(i int, overlay string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			outcomes[i] = execGo(goModuleDir(t), "test", "-count=1", "-failfast", "-json", "-overlay", overlay, "./"+tg.dir)
		}(i, overlay)
	}
	wg.Wait()
	control := outcomes[0]
	if control.err != nil {
		t.Fatal(control.err)
	}
	if control.code != 0 {
		t.Fatalf("RED: the %s tests fail with the baseline %s substituted back (exit=%d) — they pin behavior the extraction changed, or the baseline no longer compiles beside the extracted helpers:\n%s", tg.dir, tg.fn, control.code, tail(control.out))
	}
	var survivors []string
	for i, m := range mutants {
		o := outcomes[i+1]
		switch {
		case o.err != nil:
			t.Fatalf("mutant %s: %v", m.name, o.err)
		case o.code == 0:
			survivors = append(survivors, m.name)
		case !testFailed(o.out):
			survivors = append(survivors, fmt.Sprintf("%s (the mutated baseline does not compile here, exit=%d):\n%s", m.name, o.code, tail(o.out)))
		}
	}
	if len(survivors) > 0 {
		t.Errorf("RED: %d/%d behavior mutants of the baseline %s survive the %s tests — add characterization tests that pin them:\n  %s", len(survivors), len(mutants), tg.fn, tg.dir, strings.Join(survivors, "\n  "))
	}
}

func baselineFile(t *testing.T, tg target) []byte {
	t.Helper()
	root := acsassert.RepoRoot(t)
	src, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "show", baseCommit+":go/"+tg.dir+"/"+tg.file)
	if code != 0 {
		t.Fatalf("git show %s:go/%s/%s failed: %v\n%s", baseCommit, tg.dir, tg.file, err, stderr)
	}
	return []byte(src)
}

func baselineFuncSource(t *testing.T, tg target) string {
	t.Helper()
	src := baselineFile(t, tg)
	start, end, found := funcSpan(t, tg.file, src, tg.fn)
	if !found {
		t.Fatalf("baseline go/%s/%s has no func %s", tg.dir, tg.file, tg.fn)
	}
	return string(src[start:end])
}

type importSpec struct{ name, path string }

func fileImports(t *testing.T, name string, src []byte) []importSpec {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), name, src, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse imports of %s: %v", name, err)
	}
	var specs []importSpec
	for _, spec := range file.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatalf("import path %s in %s: %v", spec.Path.Value, name, err)
		}
		n := pathBase(p)
		if spec.Name != nil {
			n = spec.Name.Name
		}
		specs = append(specs, importSpec{name: n, path: p})
	}
	return specs
}

func pathBase(p string) string {
	return p[strings.LastIndex(p, "/")+1:]
}

func reconcileImports(t *testing.T, name string, src []byte, baseline []importSpec) []byte {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse substituted %s: %v", name, err)
	}
	used := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				used[id.Name] = true
			}
		}
		return true
	})
	var keep []importSpec
	seen := map[string]bool{}
	for _, spec := range append(fileImports(t, name, src), baseline...) {
		if seen[spec.path] || !(used[spec.name] || spec.name == "_" || spec.name == ".") {
			continue
		}
		seen[spec.path] = true
		keep = append(keep, spec)
	}
	var block strings.Builder
	block.WriteString("import (\n")
	for _, spec := range keep {
		fmt.Fprintf(&block, "\t%s %q\n", spec.name, spec.path)
	}
	block.WriteString(")")
	start, end := -1, -1
	for _, decl := range file.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.IMPORT {
			if start < 0 {
				start = fset.Position(gen.Pos()).Offset
			}
			end = fset.Position(gen.End()).Offset
		}
	}
	if start < 0 {
		start = fset.Position(file.Name.End()).Offset
		end = start
		block.WriteString("\n")
		return append(append(append([]byte{}, src[:start]...), "\n\n"+block.String()...), src[end:]...)
	}
	return append(append(append([]byte{}, src[:start]...), block.String()...), src[end:]...)
}

type funcSite struct {
	path       string
	src        []byte
	start, end int
	baseline   []importSpec
}

func currentFuncSite(t *testing.T, tg target) funcSite {
	t.Helper()
	dir := filepath.Join(goModuleDir(t), tg.dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if start, end, found := funcSpan(t, name, src, tg.fn); found {
			return funcSite{path: path, src: src, start: start, end: end, baseline: fileImports(t, tg.file, baselineFile(t, tg))}
		}
	}
	t.Fatalf("RED: no non-test file under %s declares func %s — keep the function (and its signature) when extracting", tg.dir, tg.fn)
	return funcSite{}
}

func (s funcSite) overlay(t *testing.T, body string) string {
	t.Helper()
	tmp := t.TempDir()
	substituted := filepath.Join(tmp, filepath.Base(s.path))
	var buf bytes.Buffer
	buf.Write(s.src[:s.start])
	buf.WriteString(body)
	buf.Write(s.src[s.end:])
	if err := os.WriteFile(substituted, reconcileImports(t, s.path, buf.Bytes(), s.baseline), 0o644); err != nil {
		t.Fatalf("write substituted source: %v", err)
	}
	return writeOverlay(t, tmp, map[string]string{s.path: substituted})
}

func funcSpan(t *testing.T, name string, src []byte, fn string) (start, end int, found bool) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	for _, decl := range file.Decls {
		if d, ok := decl.(*ast.FuncDecl); ok && d.Recv == nil && d.Name.Name == fn && d.Body != nil {
			return fset.Position(d.Pos()).Offset, fset.Position(d.End()).Offset, true
		}
	}
	return 0, 0, false
}

func writeOverlay(t *testing.T, dir string, replace map[string]string) string {
	t.Helper()
	body, err := json.Marshal(map[string]map[string]string{"Replace": replace})
	if err != nil {
		t.Fatalf("marshal overlay: %v", err)
	}
	path := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write overlay: %v", err)
	}
	return path
}

type testEvent struct {
	Action string
	Test   string
}

func testEvents(out string) []testEvent {
	var events []testEvent
	for _, line := range strings.Split(out, "\n") {
		var ev testEvent
		if json.Unmarshal([]byte(line), &ev) == nil && ev.Action != "" {
			events = append(events, ev)
		}
	}
	return events
}

func testFailed(out string) bool {
	for _, ev := range testEvents(out) {
		if ev.Action == "fail" && ev.Test != "" {
			return true
		}
	}
	return false
}

func testPassed(out, name string) bool {
	for _, ev := range testEvents(out) {
		if ev.Action == "pass" && ev.Test == name {
			return true
		}
	}
	return false
}

func tail(out string) string {
	const keep = 4000
	if len(out) <= keep {
		return out
	}
	return "…" + out[len(out)-keep:]
}

type goRun struct {
	out  string
	code int
	err  error
}

func execGo(dir string, args ...string) goRun {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		return goRun{out: string(out)}
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return goRun{out: string(out), err: fmt.Errorf("go %s: %w", strings.Join(args, " "), err)}
	}
	return goRun{out: string(out), code: exitErr.ExitCode()}
}

type worktreeGit struct{ root string }

func (g worktreeGit) run(args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("git", append([]string{"-C", g.root}, args...)...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func (g worktreeGit) ChangedFiles(base string) ([]string, error) {
	tracked, err := g.run("diff", "--name-only", "--no-renames", base)
	if err != nil {
		return nil, err
	}
	untracked, err := g.run("ls-files", "--others", "--exclude-standard", "--full-name", ":/")
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(tracked) + "\n" + string(untracked)), nil
}

func (g worktreeGit) Show(base, path string) ([]byte, error) {
	if _, err := g.run("cat-file", "-e", base+":"+path); err != nil {
		return nil, fs.ErrNotExist
	}
	return g.run("show", base+":"+path)
}

func (g worktreeGit) Root() (string, error) { return g.root, nil }
