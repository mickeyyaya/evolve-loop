package landcmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

var hostSwappers = map[string]bool{"RunPR": true, "RunCI": true, "runPR": true, "runCI": true}

func parallelHostSwappers(t *testing.T, files []string) []string {
	t.Helper()
	var offenders []string
	fset := token.NewFileSet()
	for _, file := range files {
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			swaps, parallel := false, false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch f := call.Fun.(type) {
				case *ast.Ident:
					swaps = swaps || hostSwappers[f.Name]
				case *ast.SelectorExpr:
					parallel = parallel || f.Sel.Name == "Parallel"
				}
				return true
			})
			if swaps && parallel {
				offenders = append(offenders, filepath.Base(file)+":"+fn.Name.Name)
			}
		}
	}
	return offenders
}

func TestPartialDepsInstallers_NeverRunInParallel(t *testing.T) {
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	if got := parallelHostSwappers(t, files); len(got) != 0 {
		t.Fatalf("%v call RunPR or RunCI with their own deps, or runPR or runCI with empty deps, and also call t.Parallel: the call stores those deps process-wide, so a parallel test reads deps it did not install; a parallel test reaches RunPR and RunCI only through runPRCmd and runCICmd, which install the full testDeps", got)
	}
}

func TestPartialDepsInstallers_TheScanFindsEachInstallerInAParallelTest(t *testing.T) {
	for _, name := range []string{"RunPR", "RunCI", "runPR", "runCI"} {
		t.Run(name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "probe_test.go")
			src := "package landcmd\nimport \"testing\"\nfunc TestSerial(t *testing.T) { " + name + "() }\nfunc TestRacy(t *testing.T) { t.Parallel(); " + name + "() }\nfunc TestOnlyParallel(t *testing.T) { t.Parallel(); runPRCmd() }\n"
			if err := os.WriteFile(file, []byte(src), 0o600); err != nil {
				t.Fatal(err)
			}
			if got := parallelHostSwappers(t, []string{file}); len(got) != 1 || got[0] != "probe_test.go:TestRacy" {
				t.Fatalf("offenders %v, want only probe_test.go:TestRacy for %s", got, name)
			}
		})
	}
}
