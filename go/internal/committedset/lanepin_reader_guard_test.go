package committedset

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const pinFileLiteral = "lane-scope.json"

type pinRef struct {
	Pos  string
	Func string
}

func flattenSum(e ast.Expr) []ast.Expr {
	if b, ok := e.(*ast.BinaryExpr); ok && b.Op == token.ADD {
		return append(flattenSum(b.X), flattenSum(b.Y)...)
	}
	return []ast.Expr{e}
}

func stringLit(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

func mergedLiterals(operands []ast.Expr) []string {
	var out []string
	run := ""
	open := false
	for _, op := range operands {
		if s, ok := stringLit(op); ok {
			run, open = run+s, true
			continue
		}
		if open {
			out = append(out, run)
		}
		run, open = "", false
	}
	if open {
		out = append(out, run)
	}
	return out
}

func declName(d ast.Decl) string {
	if fn, ok := d.(*ast.FuncDecl); ok {
		return fn.Name.Name
	}
	return ""
}

func pinFileRefs(fset *token.FileSet, file *ast.File) []pinRef {
	var out []pinRef
	for _, decl := range file.Decls {
		name := declName(decl)
		add := func(n ast.Node) { out = append(out, pinRef{Pos: fset.Position(n.Pos()).String(), Func: name}) }
		var visit func(n ast.Node) bool
		visit = func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.Ident:
				if v.Name == "LanePinFile" || v.Name == "LaneScopeFile" {
					add(v)
				}
			case *ast.BasicLit:
				if s, ok := stringLit(v); ok && strings.Contains(s, pinFileLiteral) {
					add(v)
				}
			case *ast.BinaryExpr:
				if v.Op != token.ADD {
					return true
				}
				operands := flattenSum(v)
				if slices.ContainsFunc(mergedLiterals(operands), func(s string) bool { return strings.Contains(s, pinFileLiteral) }) {
					add(v)
				}
				for _, op := range operands {
					if _, ok := stringLit(op); !ok {
						ast.Inspect(op, visit)
					}
				}
				return false
			}
			return true
		}
		ast.Inspect(decl, visit)
	}
	return out
}

var allowedPinFileRefs = map[string]string{
	"cmd/evolve/cmd_fleet.go|runFleet":                          "a flag help text and a message",
	"internal/core/lanescope.go|":                               "the constant LaneScopeFile",
	"internal/core/lanescope.go|materializeLaneScope":           "the write of the pin file",
	"internal/core/lanescope.go|normalizeScoutGoalHash":         "warning messages",
	"internal/core/workspace_guard.go|archivePollutedWorkspace": "the name check that spares the pin file",
	"internal/core/lane_pin_drift.go|init":                      "the text of the signal code",
	"internal/core/lane_pin_drift.go|reportLanePinDrift":        "the text of the signal",
	"internal/bridge/read_isolation.go|readIsolation":           "the sandbox pin that blocks a cell write",
	"internal/topngate/gate.go|check":                           "a message",
	"internal/core/blocker_outcome.go|consoleRouteChoice":       "a path in a failure text",
	"internal/phases/triage/triage.go|lanePinRefusal":           "a message",
}

func TestPinFileRefs_OnlyTheListedPlacesOutsideCommittedsetNameThePinFile(t *testing.T) {
	t.Parallel()
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	found := map[string][]string{}
	err = filepath.WalkDir(moduleRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if d.Name() == "testdata" || d.Name() == "acs" || rel == "internal/committedset" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		for _, ref := range pinFileRefs(fset, file) {
			key := rel + "|" + ref.Func
			found[key] = append(found[key], ref.Pos)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var unlisted, stale []string
	for key, at := range found {
		if _, ok := allowedPinFileRefs[key]; !ok {
			unlisted = append(unlisted, key+" at "+strings.Join(at, ", "))
		}
	}
	for key := range allowedPinFileRefs {
		if _, ok := found[key]; !ok {
			stale = append(stale, key)
		}
	}
	sort.Strings(unlisted)
	sort.Strings(stale)
	if len(unlisted) != 0 {
		t.Errorf("production code outside committedset names the pin file, so a cell could forge the pin: %v. Read it through committedset.LanePin, LaneDoc or ReadLaneDoc, or add the place to allowedPinFileRefs on purpose", unlisted)
	}
	if len(stale) != 0 {
		t.Errorf("allowedPinFileRefs lists places that no longer name the pin file: %v", stale)
	}
}
