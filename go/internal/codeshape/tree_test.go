package codeshape

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const shapes = `package b

type Exported struct{}

const Limit, hidden = 1, 2

func (Exported) Method() {}

func (*unexported) skip() {}

type unexported struct{}

func Long() {
` + "\tx := 0\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\tx++\n\t_ = x\n" + `}

func Deep(x int) {
	for {
		if x > 0 {
			switch {
			case x > 1:
				if x > 2 {
					return
				}
			}
		}
	}
}

func Wide(a, b, c, d, e int) {}

func Generic[T any](v T) T { return v }

type Box[T any] struct{}

func (Box[T]) Get() {}

type Pair[K, V any] struct{}

func (*Pair[K, V]) Put() {}
`

func fixtureTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "internal/b/b.go", shapes)
	writeFile(t, root, "internal/b/b_test.go", "package b\n\nfunc TestX() {}\n")
	writeFile(t, root, "internal/b/testdata/x.go", "package x\n\nfunc Skip() {}\n")
	writeFile(t, root, "acs/c1/p.go", "package c1\n\nfunc Skip() {}\n")
	writeFile(t, root, ".hidden/h.go", "package h\n\nfunc Skip() {}\n")
	writeFile(t, root, "cmd/tool/main.go", "package main\n\nfunc main() {}\n")
	return root
}

func TestScan_TheFourLimitsMatchAHandCount(t *testing.T) {
	t.Parallel()
	tree, err := Scan(fixtureTree(t))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Func{}
	for _, f := range tree.Funcs {
		byName[f.Name] = f
	}
	if len(tree.Funcs) != 9 {
		t.Fatalf("funcs = %d %v, want the 9 production functions", len(tree.Funcs), byName)
	}
	cases := map[string]bool{"Exported.Method": true, "unexported.skip": true, "Long": false, "Deep": false, "Wide": false, "Generic": true, "main": true, "Box.Get": true, "Pair.Put": true}
	for name, conforms := range cases {
		if byName[name].Conforms() != conforms {
			t.Errorf("%s = %+v, conforms want %v", name, byName[name], conforms)
		}
	}
	if byName["Long"].Lines != 31 || byName["Deep"].Nest != 4 || byName["Deep"].Cyclo != 5 || byName["Wide"].Params != 5 {
		t.Errorf("Long %+v Deep %+v Wide %+v, want 31 lines, nesting 4 with cyclomatic 5, 5 params", byName["Long"], byName["Deep"], byName["Wide"])
	}
	if k, n := Conforming(tree.Funcs); k != 6 || n != 9 {
		t.Errorf("conforming = %d/%d, want 6/9", k, n)
	}
}

func TestScan_DepthCountsExportedSymbolsOfInternalPackages(t *testing.T) {
	t.Parallel()
	tree, err := Scan(fixtureTree(t))
	if err != nil {
		t.Fatal(err)
	}
	b := tree.Packages["internal/b"]
	if b == nil || b.Exported != 11 {
		t.Fatalf("internal/b = %+v, want 11 exported: Exported, Limit, Method, Long, Deep, Wide, Generic, Box, Get, Pair and Put", b)
	}
	if d := tree.Depths(); len(d) != 1 || d[0] != float64(b.Lines)/11 {
		t.Errorf("depths = %v, want only internal/b", d)
	}
}

func TestScan_ASyntaxErrorOrAnUnreadableTreeFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, "internal/x/x.go", "package x\n\nfunc {\n")
	if _, err := Scan(root); err == nil {
		t.Error("a file that does not parse must fail the scan")
	}
	if _, err := Scan(filepath.Join(root, "absent")); !os.IsNotExist(err) {
		t.Errorf("an absent tree = %v, want a not-exist error", err)
	}
	if _, err := ParseFuncs("x.go", []byte("package")); err == nil {
		t.Error("ParseFuncs on bad source must fail")
	}
}

func TestDuplicates_TreeAndAddedLines(t *testing.T) {
	t.Parallel()
	block := "a := 1\nb := 2\nc := 3\nd := 4\ne := 5\n"
	root := t.TempDir()
	writeFile(t, root, "internal/p/one.go", "package p\n\nfunc One() {\n"+block+"}\n")
	writeFile(t, root, "internal/p/two.go", "package p\n\nfunc Two() {\n"+block+"}\n")
	tree, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if tree.DuplicateWindows() != 2 {
		t.Errorf("duplicate windows = %d, want the one block in two places", tree.DuplicateWindows())
	}
	dup, lines := tree.DiffDuplicates([][]string{strings.Split(strings.TrimSpace(block), "\n"), {"x := 9", "y := 8", "", "z := 7", "w := 6", "v := 5"}})
	if dup != 1 || lines != 10 {
		t.Errorf("diff = %d dup over %d lines, want 1 over 10: only an added window that is also elsewhere counts", dup, lines)
	}
	if r := TreeDuplication(tree); r.Index != "CQ4" || r.Interval.Value != float64(2*1000)/float64(tree.Lines) || r.Method != qualitypanel.MethodCensus {
		t.Errorf("reading = %+v", r)
	}
}

func TestRuleLines_CountsConditionalListLinesInAgentsAndSkills(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, "agents/a.md", "- If the build fails, stop.\n- **When** done, report.\n1. Unless asked, skip.\nIf plain prose, no.\n- Iffy is not a rule.\n")
	writeFile(t, root, "skills/s/SKILL.md", "* When x, y.\n")
	writeFile(t, root, "skills/s/notes.md", "- If ignored.\n")

	lines, files, err := RuleLines(root)

	if err != nil || lines != 4 || files != 2 {
		t.Errorf("rule lines %d in %d files (%v), want 4 in 2", lines, files, err)
	}
	if r := JudgmentSurface(lines, files); r.Index != "CQ5" || r.Interval.Value != 4 || r.N != 2 {
		t.Errorf("reading = %+v", r)
	}
	if err := os.Mkdir(filepath.Join(root, "agents", "dir.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := RuleLines(root); err == nil {
		t.Error("an unreadable persona file must fail")
	}
}
