package codeshape

import (
	"strings"
	"testing"
)

func TestProduction_OnlyNonTestSourceOutsideFixtureTrees(t *testing.T) {
	t.Parallel()
	want := map[string]bool{"internal/a/a.go": true, "cmd/x/main.go": true, "internal/a/a_test.go": false, "internal/a/testdata/f.go": false, "acs/c1/p.go": false, "test/e2e/x.go": false, "vendor/v/v.go": false, ".git/x.go": false, "internal/a/doc.md": false}
	for path, ok := range want {
		if Production(path) != ok {
			t.Errorf("Production(%q) = %v, want %v", path, !ok, ok)
		}
	}
}

func TestLogicalLinesAndWindows_DropBlankAndBraceLinesBeforeWindowing(t *testing.T) {
	t.Parallel()
	lines := LogicalLines("package p\n\nimport \"x\"\n\nfunc f() {\n\ta := 1\n}\n")
	if strings.Join(lines, "|") != "func f() {|a := 1" {
		t.Errorf("lines = %q", lines)
	}
	if got := Windows([]string{"1", "2", "3", "4", "5", "6"}); len(got) != 2 || got[1] != "2\n3\n4\n5\n6" {
		t.Errorf("windows = %q, want two overlapping 5-line windows", got)
	}
	if Windows([]string{"1"}) != nil {
		t.Error("fewer than 5 lines give no window")
	}
}

func TestDiffOf_ShipDiffOfACommit(t *testing.T) {
	t.Parallel()
	repo, cycles := shipFixture(t)
	var d ShipDiff
	d, err := DiffOf(repo, cycles[1].Commit)
	if err != nil || len(d.Ranges) != 2 || len(d.Added) != 2 {
		t.Errorf("diff = %+v, %v; want the two changed files", d, err)
	}
	if _, err := DiffOf(repo, "nope"); err == nil {
		t.Error("an unknown commit must be an error")
	}
}

func TestPackage_LinesAndExportedOfOneDirectory(t *testing.T) {
	t.Parallel()
	tree, err := Scan(fixtureTree(t))
	if err != nil {
		t.Fatal(err)
	}
	var p *Package = tree.Packages["cmd/tool"]
	if p == nil || p.Exported != 0 || p.Lines != 1 {
		t.Errorf("cmd/tool = %+v, want one logical line and no export", p)
	}
}
