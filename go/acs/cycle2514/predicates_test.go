//go:build acs

package cycle2514

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/testaudit"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	namedFileBase    = "apicover_named_test.go"
	baselineShardRel = "/testdata/testaudit-baseline.json"
	ownSetupRule     = "asserts-own-setup"

	protectedDiscardCeiling  = 8
	protectedOwnSetupCeiling = 1
)

func isUnprotected(rel string) bool { return !isProtected(rel) }

func isProtected(rel string) bool {
	return guards.IsProtectedScope("go/" + filepath.ToSlash(rel))
}

func namedFiles(t *testing.T, goDir string) []string {
	t.Helper()
	rels, err := acsassert.WalkModule(goDir, func(rel string) bool {
		return filepath.Base(rel) == namedFileBase
	})
	if err != nil {
		t.Fatalf("list the %s files: %v", namedFileBase, err)
	}
	if len(rels) == 0 {
		t.Fatalf("no %s file found under %s", namedFileBase, goDir)
	}
	return rels
}

func parseSource(t *testing.T, name string, src []byte) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return fset, file
}

func allBlank(names []*ast.Ident) bool {
	for _, n := range names {
		if n.Name != "_" {
			return false
		}
	}
	return len(names) > 0
}

func discardLines(fset *token.FileSet, file *ast.File) []int {
	var lines []int
	ast.Inspect(file, func(n ast.Node) bool {
		if spec, ok := n.(*ast.ValueSpec); ok && allBlank(spec.Names) && len(spec.Values) > 0 {
			lines = append(lines, fset.Position(spec.Pos()).Line)
		}
		return true
	})
	return lines
}

func discardLinesIn(t *testing.T, goDir, rel string) []int {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(goDir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	fset, file := parseSource(t, rel, raw)
	return discardLines(fset, file)
}

func TestC2514_001_NoApicoverNamedFileDiscardsAValueIntoABlank(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	var left []string
	for _, rel := range namedFiles(t, goDir) {
		if isProtected(rel) {
			continue
		}
		for _, line := range discardLinesIn(t, goDir, rel) {
			left = append(left, fmt.Sprintf("%s:%d", rel, line))
		}
	}
	if len(left) > 0 {
		t.Errorf("RED: %d `var _ = ...` discard line(s) remain in %s files; each export must be called and its result asserted:\n%s",
			len(left), namedFileBase, strings.Join(left, "\n"))
	}
}

func declaredTestNames(t *testing.T, goDir string, keep func(rel string) bool) map[string]bool {
	t.Helper()
	declared := map[string]bool{}
	for _, rel := range namedFiles(t, goDir) {
		if !keep(rel) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(goDir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		_, file := parseSource(t, rel, raw)
		for _, d := range file.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil {
				declared[filepath.ToSlash(filepath.Dir(rel))+" "+fn.Name.Name] = true
			}
		}
	}
	return declared
}

func relativePackage(goDir, pkg string) string {
	if rel, err := filepath.Rel(goDir, pkg); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(pkg)
}

func ownSetupFindings(t *testing.T, goDir string, keep func(rel string) bool) []string {
	t.Helper()
	cfg, err := testaudit.DefaultConfig()
	if err != nil {
		t.Fatalf("testaudit.DefaultConfig: %v", err)
	}
	found, err := testaudit.Analyze(filepath.Join(goDir, "internal"), cfg)
	if err != nil {
		t.Fatalf("testaudit.Analyze: %v", err)
	}
	declared := declaredTestNames(t, goDir, keep)
	var rows []string
	for _, f := range found {
		if f.Rule == ownSetupRule && declared[relativePackage(goDir, f.Package)+" "+f.Test] {
			rows = append(rows, fmt.Sprintf("%s %s %s", relativePackage(goDir, f.Package), f.Test, f.Rule))
		}
	}
	return rows
}

func ownSetupBaselineRows(t *testing.T, goDir string, keep func(rel string) bool) []string {
	t.Helper()
	declared := declaredTestNames(t, goDir, keep)
	shards, err := acsassert.WalkModule(goDir, func(rel string) bool {
		return strings.HasSuffix("/"+rel, baselineShardRel)
	})
	if err != nil {
		t.Fatalf("scan baseline shards: %v", err)
	}
	var rows []string
	for _, rel := range shards {
		raw, err := os.ReadFile(filepath.Join(goDir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		var shard struct {
			Rows []testaudit.Finding `json:"rows"`
		}
		if err := json.Unmarshal(raw, &shard); err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		for _, r := range shard.Rows {
			if r.Rule == ownSetupRule && declared[filepath.ToSlash(filepath.Clean(r.Package))+" "+r.Test] {
				rows = append(rows, fmt.Sprintf("%s: %s %s", rel, r.Package, r.Test))
			}
		}
	}
	return rows
}

func TestC2514_002_NoAssertsOwnSetupFindingInApicoverNamedFiles(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	if left := ownSetupFindings(t, goDir, isUnprotected); len(left) > 0 {
		t.Errorf("RED: %d %s finding(s) remain in %s files:\n%s", len(left), ownSetupRule, namedFileBase, strings.Join(left, "\n"))
	}
	if rows := ownSetupBaselineRows(t, goDir, isUnprotected); len(rows) > 0 {
		t.Errorf("RED: %d %s row(s) for an %s test sit in a baseline shard:\n%s", len(rows), ownSetupRule, namedFileBase, strings.Join(rows, "\n"))
	}
}

const discardSource = `package probe

var _ = Entry{ID: "x"}

func TestNames(t *testing.T) {
	var _ = Ledger{}
}
`

const typedDiscardSource = `package probe

var _ func(string) error = Remove
`

const callSource = `package probe

func TestRemove(t *testing.T) {
	if err := Remove("x"); err == nil {
		t.Fatal("missing path accepted")
	}
}
`

const quotedDiscardSource = "package probe\n\nconst src = \"var _ = Entry{}\"\n"

const selfAssertedSource = `package probe

import "testing"

type result struct {
	Repinned bool
	NewSHA   string
}

func TestSelfAssert(t *testing.T) {
	want := result{Repinned: true, NewSHA: "new"}
	if want.NewSHA != "new" || !want.Repinned {
		t.Fatalf("result fields: %+v", want)
	}
}
`

func TestC2514_005_TheProtectedCarveOutNeverGrows(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	discards := 0
	for _, rel := range namedFiles(t, goDir) {
		if isProtected(rel) {
			discards += len(discardLinesIn(t, goDir, rel))
		}
	}
	if discards > protectedDiscardCeiling {
		t.Errorf("RED: %d discard line(s) sit in protected %s files, ceiling %d; the console follow-up owns that carve-out", discards, namedFileBase, protectedDiscardCeiling)
	}
	own := ownSetupFindings(t, goDir, isProtected)
	if len(own) > protectedOwnSetupCeiling {
		t.Errorf("RED: %d %s finding(s) sit in protected %s files, ceiling %d:\n%s", len(own), ownSetupRule, namedFileBase, protectedOwnSetupCeiling, strings.Join(own, "\n"))
	}
}

func TestC2514_003_DetectorsFlagADiscardAndSpareACallOrAQuotedString(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{"untyped and function-local discards", discardSource, 2},
		{"typed discard of a function value", typedDiscardSource, 1},
		{"a call whose error is asserted", callSource, 0},
		{"a quoted discard line in a string literal", quotedDiscardSource, 0},
	}
	for _, c := range cases {
		fset, file := parseSource(t, "probe.go", []byte(c.src))
		if got := len(discardLines(fset, file)); got != c.want {
			t.Errorf("%s: discardLines = %d, want %d", c.name, got, c.want)
		}
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "probe_test.go"), []byte(selfAssertedSource), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := testaudit.DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	found, err := testaudit.Analyze(dir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(found, func(f testaudit.Finding) bool { return f.Test == "TestSelfAssert" && f.Rule == ownSetupRule }) {
		t.Errorf("a test that asserts only its own literal is not flagged %s: %v", ownSetupRule, found)
	}
}

func enforcedPackagesWithNamedFile(t *testing.T, goDir string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(goDir, ".apicover-enforce"))
	if err != nil {
		t.Fatal(err)
	}
	enforced := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		pkg := strings.TrimSpace(line)
		if pkg != "" && !strings.HasPrefix(pkg, "#") {
			enforced[filepath.ToSlash(strings.TrimPrefix(pkg, "./"))] = true
		}
	}
	var pkgs []string
	for _, rel := range namedFiles(t, goDir) {
		if dir := filepath.ToSlash(filepath.Dir(rel)); enforced[dir] {
			pkgs = append(pkgs, dir)
		}
	}
	return pkgs
}

func TestC2514_004_EveryExportStaysNamedInTheEnforcedPackagesOfTheRewrittenFiles(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	pkgs := enforcedPackagesWithNamedFile(t, goDir)
	if len(pkgs) == 0 {
		t.Fatal("no apicover-enforced package carries an apicover_named_test.go")
	}
	var unnamed []string
	for _, pkg := range pkgs {
		dir := filepath.Join(goDir, filepath.FromSlash(pkg))
		syms, err := apicover.Enumerate(context.Background(), dir)
		if err != nil {
			t.Fatalf("enumerate %s: %v", pkg, err)
		}
		named, err := apicover.NamesReferencedInTests(context.Background(), dir)
		if err != nil {
			t.Fatalf("scan tests of %s: %v", pkg, err)
		}
		for _, s := range apicover.Classify(syms, named, nil).Uncovered {
			unnamed = append(unnamed, fmt.Sprintf("%s %s (%s)", pkg, s.Name, s.File))
		}
	}
	if len(unnamed) > 0 {
		t.Errorf("RED: %d exported symbol(s) are named by no test, so apicover -enforce fails:\n%s", len(unnamed), strings.Join(unnamed, "\n"))
	}
}

func TestC2514_006_EveryTmuxTestPackageUsesTheOrphanGuard(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1",
		"-run", "^TestEveryTmuxTestPackageUsesTheOrphanGuard$", "./internal/tmuxtest")
	if err != nil {
		t.Fatalf("run the orphan-guard contract: %v", err)
	}
	if code != 0 {
		t.Errorf("RED: the orphan-guard contract fails (exit %d); faketmux needs a TestMain that calls tmuxtest.Main:\n%s%s", code, out, errOut)
	}
}

func TestC2514_007_TheFaketmuxTestPackageStillCompilesWithItsGuard(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "vet", "./internal/faketmux")
	if err != nil {
		t.Fatalf("vet faketmux: %v", err)
	}
	if code != 0 {
		t.Errorf("faketmux test package does not compile or vet (exit %d), for example through an import cycle with tmuxtest:\n%s%s", code, out, errOut)
	}
}
