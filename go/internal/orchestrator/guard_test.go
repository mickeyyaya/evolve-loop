package orchestrator

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var bannedCalls = map[string]map[string]string{
	"time":    {"Sleep": "a poll", "Tick": "a poll", "NewTicker": "a poll"},
	"syscall": {"Kill": "a broad kill", "Exec": "a process start"},
	"os":      {"FindProcess": "a stop by pid", "StartProcess": "a process start"},
}

var bannedImports = map[string]string{
	"os/exec": "a process start",
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec":  "a process start or a name-matched stop",
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree": "a name-matched stop",
}

func productionFiles(t *testing.T) []*ast.File {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		t.Fatal("no production files parsed")
	}
	return files
}

func violations(files []*ast.File) []string {
	var out []string
	for _, f := range files {
		for _, imp := range f.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if why, banned := bannedImports[path]; banned {
				out = append(out, "import "+path+" ("+why+")")
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); ok {
				if why, banned := bannedCalls[pkg.Name][sel.Sel.Name]; banned {
					out = append(out, pkg.Name+"."+sel.Sel.Name+" ("+why+")")
				}
			}
			return true
		})
	}
	return out
}

func TestOrchestrator_NeverPollsAndNeverStopsALaneByItself(t *testing.T) {
	if got := violations(productionFiles(t)); len(got) != 0 {
		t.Errorf("the orchestrator package uses %v; it wakes on events and the one deadline timer only, and a lane stop goes through the launch's own injected stop", got)
	}
}

func TestOrchestratorGuard_FindsAPlantedPollAndKill(t *testing.T) {
	src := "package p\nimport (\"os/exec\"; \"syscall\"; \"time\")\nfunc f() { time.Sleep(1); _ = time.NewTicker; syscall.Kill(pid, 15); _ = exec.Command }\n"
	f, err := parser.ParseFile(token.NewFileSet(), "planted.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := violations([]*ast.File{f}); len(got) != 4 {
		t.Errorf("violations of the planted file = %v, want 4 (exec import, Sleep, NewTicker, Kill)", got)
	}
}
