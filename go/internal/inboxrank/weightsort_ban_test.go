package inboxrank_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const rankPackage = "internal/inboxrank"

var weightName = regexp.MustCompile(`(?i)^weight[a-z0-9]?$`)

var sortCalls = map[string][]string{
	"sort":   {"Slice", "SliceStable", "Sort", "Stable"},
	"slices": {"SortFunc", "SortStableFunc", "MaxFunc", "MinFunc", "IsSortedFunc", "BinarySearchFunc"},
}

func TestNoProductionCodeOrdersByWeightOutsideTheRank(t *testing.T) {
	moduleDir, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, site := range weightOrderingSites(t, moduleDir) {
		t.Errorf("%s orders by weight: rank inbox items with inboxrank.Order (or Inputs.Order) instead; a weight that is not an inbox item's is named for what it weighs", site)
	}
}

func TestWeightOrderingSites_FindEveryShapeOfAWeightSortAndNoValidation(t *testing.T) {
	module := t.TempDir()
	fixtures.MustWrite(t, filepath.Join(module, "internal", "stray", "stray.go"), `package stray

import (
	"cmp"
	"slices"
	"sort"
)

type item struct{ ID string; Weight float64 }

func SortSlice(xs []item) { sort.SliceStable(xs, func(i, j int) bool { return xs[i].Weight > xs[j].Weight }) }

func SortFunc(xs []item) { slices.SortFunc(xs, func(a, b item) int { return cmp.Compare(b.Weight, a.Weight) }) }

func byWeight(a, b item) int { return cmp.Compare(a.Weight, b.Weight) }

func NamedComparator(xs []item) { slices.SortFunc(xs, byWeight) }

func heavier(weightA, weightB float64) bool { return weightA > weightB }

type byW []item

func (s byW) Less(i, j int) bool { return s[i].Weight < s[j].Weight }

func Validate(it item, floor float64) bool { return it.Weight <= 0 || it.Weight < floor }

func SortByID(xs []item) { sort.Slice(xs, func(i, j int) bool { return xs[i].ID < xs[j].ID }) }
`)
	fixtures.MustWrite(t, filepath.Join(module, "internal", "stray", "stray_test.go"), "package stray\n\nimport \"sort\"\n\nfunc testOnly(xs []item) { sort.Slice(xs, func(i, j int) bool { return xs[i].Weight > xs[j].Weight }) }\n")
	fixtures.MustWrite(t, filepath.Join(module, rankPackage, "rank.go"), "package inboxrank\n\nimport \"sort\"\n\ntype item struct{ Weight float64 }\n\nfunc Rank(xs []item) { sort.Slice(xs, func(i, j int) bool { return xs[i].Weight > xs[j].Weight }) }\n")
	want := []string{
		"internal/stray/stray.go:Less",
		"internal/stray/stray.go:NamedComparator",
		"internal/stray/stray.go:SortFunc",
		"internal/stray/stray.go:SortSlice",
		"internal/stray/stray.go:byWeight",
		"internal/stray/stray.go:heavier",
	}
	if got := weightOrderingSites(t, module); !slices.Equal(got, want) {
		t.Errorf("weightOrderingSites = %v, want %v", got, want)
	}
}

func weightOrderingSites(t *testing.T, moduleDir string) []string {
	t.Helper()
	var sites []string
	err := filepath.WalkDir(moduleDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(moduleDir, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			return skipGuardDir(rel, d.Name())
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || filepath.ToSlash(filepath.Dir(rel)) == rankPackage {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, name := range weightOrderingFuncs(file) {
			sites = append(sites, rel+":"+name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(sites)
	return sites
}

func skipGuardDir(rel, name string) error {
	if name == "testdata" || name == "vendor" || name == "acs" || (rel != "." && strings.HasPrefix(name, ".")) {
		return filepath.SkipDir
	}
	return nil
}

func weightOrderingFuncs(file *ast.File) []string {
	var names []string
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if ok && fd.Body != nil && ordersByWeight(fd) {
			names = append(names, fd.Name.Name)
		}
	}
	return names
}

func ordersByWeight(fd *ast.FuncDecl) bool {
	if fd.Recv != nil && fd.Name.Name == "Less" && mentionsWeight(fd.Body) {
		return true
	}
	found := false
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.CallExpr:
			found = found || (isSortCall(v) && slices.ContainsFunc(v.Args[min(1, len(v.Args)):], argMentionsWeight)) || comparesTwoWeights(v)
		case *ast.BinaryExpr:
			found = found || (isOrdering(v.Op) && isWeight(v.X) && isWeight(v.Y))
		}
		return !found
	})
	return found
}

func isSortCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && slices.Contains(sortCalls[pkg.Name], sel.Sel.Name)
}

func comparesTwoWeights(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) != 2 {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "cmp" && isWeight(call.Args[0]) && isWeight(call.Args[1])
}

func isOrdering(op token.Token) bool {
	return op == token.LSS || op == token.GTR || op == token.LEQ || op == token.GEQ
}

func isWeight(e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.Ident:
		return weightName.MatchString(v.Name)
	case *ast.SelectorExpr:
		return weightName.MatchString(v.Sel.Name)
	case *ast.ParenExpr:
		return isWeight(v.X)
	}
	return false
}

func argMentionsWeight(arg ast.Expr) bool { return mentionsWeight(arg) }

func mentionsWeight(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && strings.Contains(strings.ToLower(id.Name), "weight") {
			found = true
		}
		return !found
	})
	return found
}
