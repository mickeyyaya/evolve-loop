package sysexec

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/test/structure"
)

const (
	runnerSourceFile = "internal/modelquery/exec.go"
	runnerTypeName   = "Runner"
	runnerProgramArg = 1
)

func runnerArity(root string) (int, error) {
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, runnerSourceFile), nil, parser.SkipObjectResolution)
	if err != nil {
		return 0, err
	}
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		for i := 0; ok && i < len(gd.Specs); i++ {
			if ts, isType := gd.Specs[i].(*ast.TypeSpec); isType && ts.Name.Name == runnerTypeName {
				if ft, isFunc := ts.Type.(*ast.FuncType); isFunc {
					return ft.Params.NumFields(), nil
				}
			}
		}
	}
	return 0, fmt.Errorf("%s declares no func type %s", runnerSourceFile, runnerTypeName)
}

func runnerLLMCalls(root string, files []string, bins map[string]bool, arity int) ([]string, error) {
	var found []string
	for _, rel := range files {
		rel = filepath.ToSlash(rel)
		if filepath.ToSlash(filepath.Dir(rel)) == bridgeDir {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filepath.Join(root, rel), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		s := &llmSpawnScan{fset: fset, rel: rel, bins: bins, execPkg: execImportName(file)}
		s.consts = s.fileLLMNames(file)
		for _, decl := range file.Decls {
			s.scanRunnerCalls(decl, arity)
		}
		found = append(found, s.found...)
	}
	return found, nil
}

func (s *llmSpawnScan) scanRunnerCalls(decl ast.Decl, arity int) {
	s.tainted = maps.Clone(s.consts)
	ast.Inspect(decl, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			s.noteTaint(x.Lhs, x.Rhs)
		case *ast.ValueSpec:
			s.noteTaint(identExprs(x.Names), x.Values)
		case *ast.CallExpr:
			s.checkRunnerCall(x, arity)
		}
		return true
	})
}

func (s *llmSpawnScan) checkRunnerCall(call *ast.CallExpr, arity int) {
	if _, _, isStart := s.processStart(call); isStart || len(call.Args) != arity {
		return
	}
	program := call.Args[runnerProgramArg]
	if _, isField := program.(*ast.SelectorExpr); isField {
		return
	}
	if name, llm := s.llmProgram(program); llm {
		s.report("Runner-shaped call", call, name)
	}
}

func TestRunnerArity_IsTheParameterCountOfModelqueryRunner(t *testing.T) {
	t.Parallel()
	got, err := runnerArity(filepath.Join("..", ".."))
	if err != nil || got != 4 {
		t.Fatalf("modelquery.Runner takes %d parameters (err %v), the guard matches 4; update the guard", got, err)
	}
}

func TestRunnerArity_AMissingTypeFailsLoudly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "modelquery")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(filepath.Join(dir, "exec.go"), "package modelquery\ntype Runner struct{}\ntype Other func(a, b int)\nvar V int\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := runnerArity(root); err == nil {
		t.Fatal("a Runner that is not a func type must fail the pin")
	}
	if _, err := runnerArity(t.TempDir()); err == nil {
		t.Fatal("a missing exec.go must fail the pin")
	}
}

func TestRunnerLLMCalls_FindsEachShapeThatReachesALauncherWithoutTheBridge(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"internal/x/x.go": `package x
const agyBin = "agy"
func lit(run Runner) { run(ctx, "ollama", []string{"list"}, "") }
func cst(run Runner) { run(ctx, agyBin, nil, "") }
func tnt(run Runner) { bin := "claude"; run(ctx, bin, nil, "") }
func viaManifest(run Runner, r Resolved) { run(ctx, r.Manifest.Binary, nil, "") }
func other(run Runner) { run(ctx, "git", nil, ""); run(ctx, name, nil, ""); run(ctx, "agy", nil) }
func start() { sysexec.Command(ctx, "agy", "a", "b") }
var lit2 = func(run Runner) { run(ctx, "codex", nil, "") }
`,
		"internal/bridge/model_lister.go": `package bridge
func list(run Runner, m Manifest) { run(ctx, m.Binary, nil, "") }
`,
	}
	var rels []string
	for rel, src := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := writeFile(path, src); err != nil {
			t.Fatal(err)
		}
		rels = append(rels, rel)
	}
	slices.Sort(rels)
	bins := map[string]bool{"claude": true, "agy": true, "codex": true, "ollama": true}

	got, err := runnerLLMCalls(root, rels, bins, 4)

	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"internal/x/x.go:3: Runner-shaped call: ollama",
		"internal/x/x.go:4: Runner-shaped call: agyBin",
		"internal/x/x.go:5: Runner-shaped call: bin",
		"internal/x/x.go:9: Runner-shaped call: codex",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("calls =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if _, err := runnerLLMCalls(root, []string{"absent.go"}, bins, 4); err == nil {
		t.Fatal("a file that does not parse must fail, not report no call")
	}
}

func TestOnlyTheBridgeCallsARunnerWithAnLLMBinary(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	bins, err := llmBinaries(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := structure.SourceFiles(root, pipelineSourceDirs...)
	if err != nil {
		t.Fatal(err)
	}
	arity, err := runnerArity(root)
	if err != nil {
		t.Fatal(err)
	}
	calls, err := runnerLLMCalls(root, files, bins, arity)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) > 0 {
		t.Errorf("%d Runner-shaped call(s) name an LLM binary outside the bridge; the bridge model lister owns them:\n%s", len(calls), strings.Join(calls, "\n"))
	}
}
