package sysexec

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/test/structure"
)

const bridgeDir = "internal/bridge"

var baseLLMBinaries = []string{"claude", "agy", "codex", "gemini", "ollama"}

var llmLaunchSitesOutsideTheBridge = []string{
	"internal/cliupdate/exec.go",
	"internal/looppreflight/versioninventory.go",
	"internal/modelquery/exec.go",
}

var bridgeLaunchAPI = []string{"LaunchLLM", "launchSwept", "launchPane", "launchPurposeConfig", "newRecipeDriver"}

type llmSpawnScan struct {
	fset     *token.FileSet
	rel      string
	bins     map[string]bool
	inBridge bool
	research bool
	execPkg  string
	runFuncs map[string]bool
	found    []string
}

func llmBinaries(root string) (map[string]bool, error) {
	bins := map[string]bool{}
	for _, b := range baseLLMBinaries {
		bins[b] = true
	}
	manifests, err := filepath.Glob(filepath.Join(root, bridgeDir, "manifests", "*.json"))
	if err != nil || len(manifests) == 0 {
		return nil, fmt.Errorf("no bridge manifest under %s (err %v)", root, err)
	}
	for _, path := range manifests {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var m struct {
			Binary string `json:"binary"`
		}
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if m.Binary != "" {
			bins[m.Binary] = true
		}
	}
	return bins, nil
}

func llmSpawns(root string, files []string, bins map[string]bool) ([]string, error) {
	var found []string
	for _, rel := range files {
		rel = filepath.ToSlash(rel)
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filepath.Join(root, rel), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		s := &llmSpawnScan{fset: fset, rel: rel, bins: bins, inBridge: filepath.ToSlash(filepath.Dir(rel)) == bridgeDir, research: slices.Contains(llmLaunchSitesOutsideTheBridge, rel), execPkg: execImportName(file)}
		for _, decl := range file.Decls {
			s.scanDecl(decl)
		}
		found = append(found, s.found...)
	}
	return found, nil
}

func (s *llmSpawnScan) scanDecl(decl ast.Decl) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		s.scanFunc(d.Name.Name, d)
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			for i := 0; ok && i < len(vs.Values) && i < len(vs.Names); i++ {
				s.scanFunc(vs.Names[i].Name, vs.Values[i])
			}
		}
	}
}

func (s *llmSpawnScan) inLaunchAPI(name string) bool {
	return s.inBridge && (slices.Contains(bridgeLaunchAPI, name) || strings.HasPrefix(filepath.Base(s.rel), "driver_"))
}

func (s *llmSpawnScan) scanFunc(fn string, body ast.Node) {
	if s.inLaunchAPI(fn) {
		return
	}
	s.runFuncs = map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncType:
			s.noteRunFuncParams(x)
		case *ast.CallExpr:
			s.checkCall(fn, x)
		case *ast.KeyValueExpr:
			if key, ok := x.Key.(*ast.Ident); ok && key.Name == "AllowBypass" && isTrue(x.Value) {
				s.report(fn, x, "AllowBypass: true")
			}
		case *ast.AssignStmt:
			s.checkBypassAssign(fn, x)
		}
		return true
	})
}

func (s *llmSpawnScan) noteRunFuncParams(ft *ast.FuncType) {
	for _, field := range ft.Params.List {
		if sel, ok := field.Type.(*ast.SelectorExpr); ok && isPkg(sel.X, "sysexec") && sel.Sel.Name == "RunFunc" {
			for _, name := range field.Names {
				s.runFuncs[name.Name] = true
			}
		}
	}
}

func (s *llmSpawnScan) checkBypassAssign(fn string, as *ast.AssignStmt) {
	for i, lhs := range as.Lhs {
		if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "AllowBypass" && i < len(as.Rhs) && isTrue(as.Rhs[i]) {
			s.report(fn, as, "AllowBypass = true")
		}
	}
}

func (s *llmSpawnScan) checkCall(fn string, call *ast.CallExpr) {
	what, program, ok := s.processStart(call)
	if ok {
		literal, isLiteral := stringLiteral(program)
		switch {
		case isLiteral && s.bins[literal]:
			s.report(fn, call, what+" "+literal)
		case !isLiteral && (s.research || (s.inBridge && what == "Runner")):
			s.report(fn, call, what)
		}
		return
	}
	if !s.inBridge {
		return
	}
	switch f := call.Fun.(type) {
	case *ast.SelectorExpr:
		if f.Sel.Name == "Launch" && len(call.Args) == 3 {
			s.report(fn, call, "Driver.Launch")
		}
	case *ast.Ident:
		if f.Name == "launchCmdLine" {
			s.report(fn, call, "launchCmdLine")
		}
	}
}

func (s *llmSpawnScan) processStart(call *ast.CallExpr) (string, ast.Expr, bool) {
	arg := func(i int) ast.Expr {
		if i < len(call.Args) {
			return call.Args[i]
		}
		return nil
	}
	switch f := call.Fun.(type) {
	case *ast.SelectorExpr:
		switch {
		case isPkg(f.X, "sysexec") && (f.Sel.Name == "Command" || f.Sel.Name == "DefaultRunner"):
			return "sysexec." + f.Sel.Name, arg(1), true
		case isPkg(f.X, s.execPkg) && f.Sel.Name == "Command":
			return "exec.Command", arg(0), true
		case isPkg(f.X, s.execPkg) && f.Sel.Name == "CommandContext":
			return "exec.CommandContext", arg(1), true
		case f.Sel.Name == "Runner":
			return "Runner", arg(1), true
		}
	case *ast.Ident:
		if s.runFuncs[f.Name] {
			return "sysexec.RunFunc " + f.Name, arg(1), true
		}
	}
	return "", nil, false
}

func (s *llmSpawnScan) report(fn string, n ast.Node, what string) {
	s.found = append(s.found, s.rel+":"+strconv.Itoa(s.fset.Position(n.Pos()).Line)+": "+fn+": "+what)
}

func stringLiteral(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	return v, err == nil
}

func isTrue(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "true"
}

func treeLLMSpawns(t *testing.T) []string {
	t.Helper()
	root := filepath.Join("..", "..")
	bins, err := llmBinaries(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := structure.SourceFiles(root, pipelineSourceDirs...)
	if err != nil {
		t.Fatalf("walk %v: %v", pipelineSourceDirs, err)
	}
	for _, rel := range llmLaunchSitesOutsideTheBridge {
		if !slices.Contains(files, filepath.FromSlash(rel)) {
			t.Fatalf("launch site %s of research F1.2 is gone; update llmLaunchSitesOutsideTheBridge and spec §5", rel)
		}
	}
	spawns, err := llmSpawns(root, files, bins)
	if err != nil {
		t.Fatal(err)
	}
	return spawns
}

func TestOnlyTheBridgeStartsAnLLMBinary(t *testing.T) {
	t.Parallel()
	if spawns := treeLLMSpawns(t); len(spawns) > 0 {
		t.Errorf("%d LLM spawn(s) outside the bridge launch API %v; route each through bridge.LaunchLLM or launchPane:\n%s", len(spawns), bridgeLaunchAPI, strings.Join(spawns, "\n"))
	}
}

func TestLLMBinaries_AddsEachManifestBinaryToTheBaseNames(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, bridgeDir, "manifests")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := llmBinaries(root); err == nil {
		t.Fatal("a tree with no manifest must fail, not report no LLM binary")
	}
	if err := writeFile(filepath.Join(dir, "x.json"), `{"binary": "newllm"}`); err != nil {
		t.Fatal(err)
	}
	bins, err := llmBinaries(root)
	if err != nil || !bins["newllm"] || !bins["gemini"] {
		t.Fatalf("bins = %v, err = %v; want the manifest binary and the base names", bins, err)
	}
	if err := writeFile(filepath.Join(dir, "y.json"), `{`); err != nil {
		t.Fatal(err)
	}
	if _, err := llmBinaries(root); err == nil {
		t.Fatal("a manifest that does not parse must fail")
	}
}

func TestLLMSpawns_FindsEachSpawnShapeOutsideTheLaunchAPI(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"internal/x/x.go": `package x
import "os/exec"
func a() { sysexec.Command(ctx, "claude", "-p"); exec.Command("newllm"); exec.CommandContext(ctx, "agy"); sysexec.Command(ctx, "git") }
func b(run sysexec.RunFunc) { run(ctx, "codex", "", nil, nil, nil, nil, nil); run(ctx, name, "", nil, nil, nil, nil, nil) }
func c() { sysexec.Command(ctx, bin) }
var e, f = func() { sysexec.Command(ctx, "claude") }, 1
`,
		"internal/y/y.go": `package y
import osexec "os/exec"
func g() { osexec.Command("claude"); exec.Command("agy") }
`,
		"internal/bridge/driver_x.go": `package bridge
func xLaunch() { launchCmdLine("claude", nil); deps.Runner(ctx, name, "", nil, nil, nil, nil, nil) }
`,
		"internal/modelquery/exec.go": `package modelquery
func d() { sysexec.Command(ctx, name) }
`,
		"internal/bridge/b.go": `package bridge
func doctor() { deps.Runner(ctx, binary, "", nil, nil, nil, nil, nil); deps.Runner(ctx, "git", "", nil, nil, nil, nil, nil) }
func smoke() { d.Launch(ctx, cfg, deps); cfg.AllowBypass = true; cfg.AllowBypass = false }
func factory() *Config { return &Config{AllowBypass: true} }
func recipe() { launchCmdLine("claude", nil); sysexec.Command(ctx, name) }
func launchSwept() { driver.Launch(ctx, cfg, deps); cfg.AllowBypass = true }
func (agyDriver) Launch() { deps.Runner(ctx, name, "", nil, nil, nil, nil, nil) }
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
	bins := map[string]bool{"claude": true, "agy": true, "codex": true, "newllm": true}

	got, err := llmSpawns(root, rels, bins)

	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"internal/bridge/b.go:2: doctor: Runner",
		"internal/bridge/b.go:3: smoke: Driver.Launch",
		"internal/bridge/b.go:3: smoke: AllowBypass = true",
		"internal/bridge/b.go:4: factory: AllowBypass: true",
		"internal/bridge/b.go:5: recipe: launchCmdLine",
		"internal/bridge/b.go:7: Launch: Runner",
		"internal/modelquery/exec.go:2: d: sysexec.Command",
		"internal/x/x.go:3: a: sysexec.Command claude",
		"internal/x/x.go:3: a: exec.Command newllm",
		"internal/x/x.go:3: a: exec.CommandContext agy",
		"internal/x/x.go:4: b: sysexec.RunFunc run codex",
		"internal/x/x.go:6: e: sysexec.Command claude",
		"internal/y/y.go:3: g: exec.Command claude",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("spawns =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if _, err := llmSpawns(root, []string{"absent.go"}, bins); err == nil {
		t.Fatal("a file that does not parse must fail, not report no spawn")
	}
}
