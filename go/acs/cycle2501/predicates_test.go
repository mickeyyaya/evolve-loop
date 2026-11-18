//go:build acs

package cycle2501

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
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

const baselineShard = "testdata/testaudit-baseline.json"

var tautologyRules = []string{"no-assertion", "log-only", "asserts-own-setup", "both-branches-pass"}

type flaggedTest struct {
	Package string `json:"package"`
	Test    string `json:"test"`
	Rule    string `json:"rule"`
	File    string `json:"file"`
}

func inRestScope(pkg string) bool {
	p := filepath.ToSlash(filepath.Clean(pkg))
	return p != "internal/core" && p != "internal/bridge" && p != "cmd/evolve" && p != "acs" && !strings.HasPrefix(p, "acs/")
}

func declaringFile(dir, test string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, e.Name()), nil, parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		for _, d := range file.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == test {
				return filepath.Join(dir, e.Name())
			}
		}
	}
	return ""
}

func inDir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(prev); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
}

func tautologyFindings(t *testing.T, goDir string) (open, protected []string) {
	t.Helper()
	cfg, err := testaudit.DefaultConfig()
	if err != nil {
		t.Fatalf("testaudit.DefaultConfig: %v", err)
	}
	inDir(t, goDir)
	found, err := testaudit.Analyze(".", cfg)
	if err != nil {
		t.Fatalf("testaudit.Analyze: %v", err)
	}
	for _, f := range found {
		if !slices.Contains(tautologyRules, f.Rule) || !inRestScope(f.Package) {
			continue
		}
		row := fmt.Sprintf("%s %s %s", f.Package, f.Test, f.Rule)
		file := declaringFile(filepath.Join(goDir, f.Package), f.Test)
		if file != "" && guards.IsProtectedScope(file) {
			protected = append(protected, row)
			continue
		}
		open = append(open, row)
	}
	return open, protected
}

func shardTautologyRows(t *testing.T, goDir string) []string {
	t.Helper()
	var rows []string
	err := filepath.WalkDir(goDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "vendor" || strings.HasPrefix(d.Name(), ".")) {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(filepath.ToSlash(path), "/"+baselineShard) {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var shard struct {
			Rows []testaudit.Finding `json:"rows"`
		}
		if err := json.Unmarshal(raw, &shard); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		for _, r := range shard.Rows {
			if slices.Contains(tautologyRules, r.Rule) && inRestScope(r.Package) {
				rows = append(rows, fmt.Sprintf("%s %s %s", r.Package, r.Test, r.Rule))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan baseline shards: %v", err)
	}
	return rows
}

func head(lines []string, n int) string {
	if len(lines) > n {
		return strings.Join(lines[:n], "\n") + fmt.Sprintf("\n... and %d more", len(lines)-n)
	}
	return strings.Join(lines, "\n")
}

func TestC2501_001_NoTautologicalFindingLeftOutsideCoreBridgeCmdAcs(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	if left, _ := tautologyFindings(t, goDir); len(left) > 0 {
		t.Errorf("RED: %d tautological finding(s) remain outside internal/core, internal/bridge, cmd/evolve and acs:\n%s", len(left), head(left, 40))
	}
	if rows := shardTautologyRows(t, goDir); len(rows) > 0 {
		t.Errorf("RED: %d tautology row(s) still sit in a testaudit baseline shard:\n%s", len(rows), head(rows, 40))
	}
}

func TestC2501_002_FlaggedTestsAreRewrittenNotDeleted(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	raw, err := os.ReadFile("testdata/flagged-tests.json")
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct {
		Rows []flaggedTest `json:"rows"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Rows) < 100 {
		t.Fatalf("flagged-test snapshot holds %d rows, the RED tree had 141", len(snapshot.Rows))
	}
	var gone []string
	for _, row := range snapshot.Rows {
		if declaringFile(filepath.Join(goDir, row.Package), row.Test) == "" {
			gone = append(gone, fmt.Sprintf("%s %s (was %s in %s)", row.Package, row.Test, row.Rule, row.File))
		}
	}
	if len(gone) > 0 {
		t.Errorf("RED: %d flagged test(s) were deleted or renamed instead of rewritten — a deleted test leaves the baseline without ever failing a mutant:\n%s", len(gone), head(gone, 40))
	}
}

func TestC2501_003_AnalyzerStillSeparatesTautologyFromBehavior(t *testing.T) {
	for _, rule := range tautologyRules {
		if !slices.Contains(testaudit.RuleIDs(), rule) {
			t.Errorf("testaudit no longer defines rule %q, so predicate 001 would pass for the wrong reason", rule)
		}
	}
	dir := t.TempDir()
	src := `package probe

import "testing"

func TestOwnSetup(t *testing.T) {
	cfg := struct{ Name string }{Name: "x"}
	if cfg.Name != "x" {
		t.Fatal("never fires")
	}
}

func TestNoAssertion(t *testing.T) {
	_ = add(1, 2)
}

func TestBehavior(t *testing.T) {
	if got := add(2, 3); got != 5 {
		t.Errorf("add(2,3) = %d", got)
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "probe_test.go"), []byte(src), 0o644); err != nil {
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
	flagged := map[string]string{}
	for _, f := range found {
		flagged[f.Test] = f.Rule
	}
	if flagged["TestOwnSetup"] != "asserts-own-setup" {
		t.Errorf("a test asserting only its own literal is not flagged: %v", flagged)
	}
	if flagged["TestNoAssertion"] != "no-assertion" {
		t.Errorf("a test with no assertion is not flagged: %v", flagged)
	}
	if rule, bad := flagged["TestBehavior"]; bad {
		t.Errorf("a test asserting a returned value is flagged %q", rule)
	}
}

const protectedCarveOutCeiling = 16

func TestC2501_004_ProtectedCarveOutNeverGrows(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	_, protected := tautologyFindings(t, goDir)
	if len(protected) > protectedCarveOutCeiling {
		t.Errorf("RED: %d tautological finding(s) sit in protected test files, the console follow-up owns at most %d:\n%s", len(protected), protectedCarveOutCeiling, head(protected, 40))
	}
}

func unnamedExports(t *testing.T, goDir string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(goDir, ".apicover-enforce"))
	if err != nil {
		t.Fatal(err)
	}
	var unnamed []string
	for _, line := range strings.Split(string(raw), "\n") {
		pkg := strings.TrimSpace(line)
		if pkg == "" || strings.HasPrefix(pkg, "#") {
			continue
		}
		dir := filepath.Join(goDir, filepath.FromSlash(strings.TrimPrefix(pkg, "./")))
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
	return unnamed
}

func TestC2501_005_RewritesKeepEveryEnforcedExportNamed(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	if unnamed := unnamedExports(t, goDir); len(unnamed) > 0 {
		t.Errorf("RED: %d exported symbol(s) of an apicover-enforced package are named by no test, so CI api-coverage enforce fails:\n%s", len(unnamed), head(unnamed, 40))
	}
}

func flaggedByTestaudit(t *testing.T, src string) map[string]string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "probe_test.go"), []byte(src), 0o644); err != nil {
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
	flagged := map[string]string{}
	for _, f := range found {
		flagged[f.Test] = f.Rule
	}
	return flagged
}

const filledFromProductionSrc = `package probe

import "testing"

type item struct {
	name string
	n    int
}

type summary struct{ total int }

func produce() []item { return []item{{"a", 1}, {"a", 2}} }

func sum(items []item) int {
	total := 0
	for _, it := range items {
		total += it.n
	}
	return total
}

func TestMapKeyedFromProduction(t *testing.T) {
	byName := map[string]int{}
	for _, it := range produce() {
		byName[it.name] = it.n
	}
	if byName["a"] != 2 {
		t.Fatalf("byName = %v", byName)
	}
}

func TestCounterFromProduction(t *testing.T) {
	counts := map[string]int{}
	for _, it := range produce() {
		counts[it.name]++
	}
	if counts["a"] != 2 {
		t.Fatalf("counts = %v", counts)
	}
}

func TestFieldFromProduction(t *testing.T) {
	agg := summary{}
	agg.total = sum(produce())
	if agg.total != 3 {
		t.Fatalf("agg = %+v", agg)
	}
}

func TestSliceAppendedFromProduction(t *testing.T) {
	names := []string{}
	for _, it := range produce() {
		names = append(names, it.name)
	}
	if len(names) != 2 {
		t.Fatalf("names = %v", names)
	}
}
`

func TestC2501_006_ContainerFilledFromProductionIsNotOwnSetup(t *testing.T) {
	flagged := flaggedByTestaudit(t, filledFromProductionSrc)
	for _, name := range []string{"TestMapKeyedFromProduction", "TestCounterFromProduction", "TestFieldFromProduction", "TestSliceAppendedFromProduction"} {
		if rule, bad := flagged[name]; bad {
			t.Errorf("RED: %s fills its container from production output and is flagged %q", name, rule)
		}
	}
}

const filledFromLiteralsSrc = `package probe

import "testing"

type config struct{ Name string }

func TestMapOfOwnLiterals(t *testing.T) {
	m := map[string]int{}
	m["a"] = 1
	if m["a"] != 1 {
		t.Fatal("never fires")
	}
}

func TestFieldOfOwnLiteral(t *testing.T) {
	cfg := config{}
	cfg.Name = "x"
	if cfg.Name != "x" {
		t.Fatal("never fires")
	}
}

func TestCounterOfOwnLiteral(t *testing.T) {
	m := map[string]int{}
	m["a"]++
	if m["a"] != 1 {
		t.Fatal("never fires")
	}
}
`

func TestC2501_007_ContainerFilledFromLiteralsIsStillOwnSetup(t *testing.T) {
	flagged := flaggedByTestaudit(t, filledFromLiteralsSrc)
	for _, name := range []string{"TestMapOfOwnLiterals", "TestFieldOfOwnLiteral", "TestCounterOfOwnLiteral"} {
		if flagged[name] != "asserts-own-setup" {
			t.Errorf("%s asserts only values it wrote from literals and is not flagged asserts-own-setup: %q", name, flagged[name])
		}
	}
}
