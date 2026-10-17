package bridge

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/test/structure"
)

func TestDriverNames_AreBuiltOnlyByResolve(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	files, err := structure.SourceFiles(root, "cmd", "internal", "pkg")
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	var offenders []string
	byDir := map[string][]string{}
	for _, rel := range files {
		dir := filepath.Dir(rel)
		byDir[dir] = append(byDir[dir], filepath.Join(root, rel))
	}
	for _, rel := range files {
		consts := packageStrings(t, byDir[filepath.Dir(rel)])
		offenders = append(offenders, tmuxNameBuilds(t, filepath.Join(root, rel), rel, consts)...)
	}
	if len(offenders) > 0 {
		t.Errorf("%d site(s) build a driver name by hand; call bridge.Resolve:\n%s", len(offenders), strings.Join(offenders, "\n"))
	}
}

func TestTmuxNameBuilds_FindsEachBannedShape(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "x.go")
	src := "package x\nimport \"fmt\"\nconst suffixConst = \"-tmux\"\nvar suffixVar = \"%s-tmux\"\nfunc f(family string) []string {\n" +
		"\tname := family\n\tname += \"-tmux\"\n\tname += suffixConst\n\t_ = family + suffixVar\n\t_ = suffixConst + family\n\treturn []string{family + \"-tmux\", \"-tmux\" + family, fmt.Sprintf(\"%s-tmux\", family), \"run <family>-tmux \" + family, fmt.Sprintf(\"the %s-tmux manifest\", family), family + \"-p\"}\n}\n"
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	got := tmuxNameBuilds(t, path, "x.go", packageStrings(t, []string{path}))

	if len(got) != 7 {
		t.Errorf("found %q, want the seven name builds, not the prose or the -p join", got)
	}
}

func packageStrings(t *testing.T, paths []string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, path := range paths {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			spec, ok := n.(*ast.ValueSpec)
			if !ok || len(spec.Names) != len(spec.Values) {
				return true
			}
			for i, v := range spec.Values {
				if lit, ok := v.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if s, err := strconv.Unquote(lit.Value); err == nil {
						out[spec.Names[i].Name] = s
					}
				}
			}
			return true
		})
	}
	return out
}

func tmuxNameBuilds(t *testing.T, path, rel string, consts map[string]string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		if buildsTmuxName(n, consts) {
			out = append(out, rel+":"+strconv.Itoa(fset.Position(n.Pos()).Line))
		}
		return true
	})
	return out
}

func buildsTmuxName(n ast.Node, consts map[string]string) bool {
	switch x := n.(type) {
	case *ast.BinaryExpr:
		return x.Op == token.ADD && (isTmuxLiteral(x.X, consts) || isTmuxLiteral(x.Y, consts))
	case *ast.AssignStmt:
		return x.Tok == token.ADD_ASSIGN && len(x.Rhs) == 1 && isTmuxLiteral(x.Rhs[0], consts)
	case *ast.CallExpr:
		sel, ok := x.Fun.(*ast.SelectorExpr)
		return ok && sel.Sel.Name == "Sprintf" && len(x.Args) > 0 && isTmuxLiteral(x.Args[0], consts)
	}
	return false
}

func isTmuxLiteral(e ast.Expr, consts map[string]string) bool {
	var s string
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return false
		}
		u, err := strconv.Unquote(v.Value)
		if err != nil {
			return false
		}
		s = u
	case *ast.Ident:
		s = consts[v.Name]
	default:
		return false
	}
	return s == tmuxSuffix || s == "%s"+tmuxSuffix
}
