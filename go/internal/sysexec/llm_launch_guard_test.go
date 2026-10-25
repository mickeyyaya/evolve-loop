package sysexec

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path"
	"path/filepath"
	"reflect"
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

var bridgeDriverLaunchFuncs = []string{
	"driver_agy.go:agyDriver.Launch", "driver_agytmux.go:agyTmuxLaunch", "driver_claudep.go:claudePDriver.Launch",
	"driver_claudetmux.go:claudeTmuxDriver.Launch", "driver_codex.go:codexDriver.Launch", "driver_codextmux.go:codexTmuxDriver.Launch",
}

var shellPrograms = []string{"sh", "bash", "zsh", "dash", "tmux"}

const runFuncArity = 8

type llmSpawnScan struct {
	fset     *token.FileSet
	rel      string
	bins     map[string]bool
	inBridge bool
	research bool
	execPkg  string
	tainted  map[string]bool
	consts   map[string]bool
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
		s.consts = s.fileLLMNames(file)
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
		s.scanFunc(funcKey(d), d)
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			for i := 0; ok && i < len(vs.Values) && i < len(vs.Names); i++ {
				s.scanFunc(vs.Names[i].Name, vs.Values[i])
			}
		}
	}
}

func funcKey(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return d.Name.Name
	}
	recv := d.Recv.List[0].Type
	if star, ok := recv.(*ast.StarExpr); ok {
		recv = star.X
	}
	if id, ok := recv.(*ast.Ident); ok {
		return id.Name + "." + d.Name.Name
	}
	return d.Name.Name
}

func (s *llmSpawnScan) inLaunchAPI(fn string) bool {
	return s.inBridge && (slices.Contains(bridgeLaunchAPI, fn) || slices.Contains(bridgeDriverLaunchFuncs, path.Base(s.rel)+":"+fn))
}

func (s *llmSpawnScan) scanFunc(fn string, body ast.Node) {
	if s.inLaunchAPI(fn) {
		return
	}
	s.tainted = maps.Clone(s.consts)
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			s.checkCall(fn, x)
		case *ast.KeyValueExpr:
			if key, ok := x.Key.(*ast.Ident); ok && key.Name == "AllowBypass" && isTrue(x.Value) {
				s.report(fn, x, "AllowBypass: true")
			}
		case *ast.AssignStmt:
			s.noteTaint(x.Lhs, x.Rhs)
			s.checkBypassAssign(fn, x)
		case *ast.ValueSpec:
			s.noteTaint(identExprs(x.Names), x.Values)
		}
		return true
	})
}

func (s *llmSpawnScan) fileLLMNames(file *ast.File) map[string]bool {
	s.tainted = map[string]bool{}
	for _, decl := range file.Decls {
		if gd, ok := decl.(*ast.GenDecl); ok {
			for _, spec := range gd.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					s.noteTaint(identExprs(vs.Names), vs.Values)
				}
			}
		}
	}
	return s.tainted
}

func identExprs(names []*ast.Ident) []ast.Expr {
	out := make([]ast.Expr, len(names))
	for i, name := range names {
		out[i] = name
	}
	return out
}

func (s *llmSpawnScan) noteTaint(lhs, rhs []ast.Expr) {
	for i, l := range lhs {
		id, ok := l.(*ast.Ident)
		if !ok {
			continue
		}
		var value ast.Expr
		switch {
		case len(rhs) == len(lhs):
			value = rhs[i]
		case len(rhs) == 1 && i == 0:
			value = rhs[0]
		}
		if _, llm := s.llmProgram(value); llm {
			s.tainted[id.Name] = true
		}
	}
}

func (s *llmSpawnScan) llmProgram(e ast.Expr) (string, bool) {
	switch x := e.(type) {
	case *ast.BasicLit:
		v, ok := stringLiteral(x)
		return v, ok && s.bins[v]
	case *ast.Ident:
		return x.Name, s.tainted[x.Name]
	case *ast.SelectorExpr:
		return "." + x.Sel.Name, x.Sel.Name == "Binary"
	case *ast.CallExpr:
		if sel, ok := x.Fun.(*ast.SelectorExpr); ok && isPkg(sel.X, s.execPkg) && sel.Sel.Name == "LookPath" && len(x.Args) == 1 {
			name, llm := s.llmProgram(x.Args[0])
			return "LookPath(" + name + ")", llm
		}
	}
	return "", false
}

func (s *llmSpawnScan) checkBypassAssign(fn string, as *ast.AssignStmt) {
	for i, lhs := range as.Lhs {
		if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "AllowBypass" && i < len(as.Rhs) && isTrue(as.Rhs[i]) {
			s.report(fn, as, "AllowBypass = true")
		}
	}
}

func (s *llmSpawnScan) checkCall(fn string, call *ast.CallExpr) {
	if what, idx, ok := s.processStart(call); ok {
		s.checkProcessStart(fn, call, what, idx)
		return
	}
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "SendKeys" {
		if bin, found := s.llmWordInLiterals(call.Args); found {
			s.report(fn, call, "SendKeys "+bin)
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

func (s *llmSpawnScan) checkProcessStart(fn string, call *ast.CallExpr, what string, idx int) {
	if idx >= len(call.Args) {
		return
	}
	program := call.Args[idx]
	if name, llm := s.llmProgram(program); llm {
		s.report(fn, call, what+" "+name)
		return
	}
	if literal, ok := stringLiteral(program); ok {
		if slices.Contains(shellPrograms, filepath.Base(literal)) {
			if bin, found := s.llmWordInLiterals(call.Args[idx+1:]); found {
				s.report(fn, call, what+" "+literal+" "+bin)
			}
		}
		return
	}
	if s.research || (s.inBridge && what == "Runner") {
		s.report(fn, call, what)
	}
}

func (s *llmSpawnScan) llmWordInLiterals(args []ast.Expr) (string, bool) {
	for _, a := range args {
		v, ok := stringLiteral(a)
		if !ok {
			continue
		}
		for _, word := range strings.FieldsFunc(v, isShellSeparator) {
			if base := filepath.Base(word); s.bins[base] {
				return base, true
			}
		}
	}
	return "", false
}

func isShellSeparator(r rune) bool {
	return strings.ContainsRune(" \t\n;|&()<>'\"`$=", r)
}

func (s *llmSpawnScan) processStart(call *ast.CallExpr) (string, int, bool) {
	switch f := call.Fun.(type) {
	case *ast.SelectorExpr:
		switch {
		case isPkg(f.X, "sysexec") && (f.Sel.Name == "Command" || f.Sel.Name == "DefaultRunner"):
			return "sysexec." + f.Sel.Name, 1, true
		case isPkg(f.X, "sysexec") && (f.Sel.Name == "Output" || f.Sel.Name == "Capture" || f.Sel.Name == "CombinedOutput"):
			return "sysexec." + f.Sel.Name, 3, true
		case isPkg(f.X, s.execPkg) && f.Sel.Name == "Command":
			return "exec.Command", 0, true
		case isPkg(f.X, s.execPkg) && f.Sel.Name == "CommandContext":
			return "exec.CommandContext", 1, true
		case isPkg(f.X, "os") && f.Sel.Name == "StartProcess":
			return "os.StartProcess", 0, true
		case isPkg(f.X, "syscall") && (f.Sel.Name == "Exec" || f.Sel.Name == "ForkExec"):
			return "syscall." + f.Sel.Name, 0, true
		case f.Sel.Name == "Runner":
			return "Runner", 1, true
		}
	}
	if len(call.Args) == runFuncArity {
		return "RunFunc call", 1, true
	}
	return "", 0, false
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

func TestRunFuncArity_IsTheParameterCountOfRunFunc(t *testing.T) {
	t.Parallel()
	if got := reflect.TypeOf(RunFunc(nil)).NumIn(); got != runFuncArity {
		t.Fatalf("RunFunc takes %d parameters, the guard matches %d; update runFuncArity", got, runFuncArity)
	}
}

func TestBridgeDriverLaunchFuncs_EachNamesARealDriverFunction(t *testing.T) {
	t.Parallel()
	for _, entry := range bridgeDriverLaunchFuncs {
		file, fn, _ := strings.Cut(entry, ":")
		parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "..", bridgeDir, file), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("%s: %v", entry, err)
		}
		if !slices.ContainsFunc(parsed.Decls, func(d ast.Decl) bool { fd, ok := d.(*ast.FuncDecl); return ok && funcKey(fd) == fn }) {
			t.Errorf("%s names no function of %s; a stale entry exempts nothing", entry, file)
		}
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
		"internal/bridge/driver_agy.go": `package bridge
func (agyDriver) Launch() { deps.Runner(ctx, name, "", nil, nil, nil, nil, nil) }
`,
		"internal/z/z.go": `package z
import "os/exec"
func g1() { sysexec.Output(ctx, run, "", "claude", "--version"); sysexec.Capture(ctx, run, "", "codex"); sysexec.CombinedOutput(ctx, run, "", "agy", ""); sysexec.Output(ctx, run, "", "git") }
func g2(r Resolved) { bin := "claude"; sysexec.Command(ctx, bin); sysexec.Command(ctx, r.Manifest.Binary); var other = "git"; sysexec.Command(ctx, other) }
func g3() { p, _ := exec.LookPath("newllm"); sysexec.Command(ctx, p) }
func g4() { sysexec.Command(ctx, "sh", "-c", "claude -p hi"); sysexec.Command(ctx, "/bin/bash", "-lc", "cd /w && agy models"); sysexec.Command(ctx, "sh", "-c", "echo ~/.claude/x") }
func g5() { sysexec.Command(ctx, "tmux", "send-keys", "-t", "s", "claude --dangerously-skip-permissions"); deps.Tmux.SendKeys(ctx, s, "codex exec", true); deps.Tmux.SendKeys(ctx, s, "Enter", false) }
func g7(o opts) { o.Run(ctx, "agy", "", nil, nil, nil, nil, nil) }
func g8() { os.StartProcess("claude", nil, nil); syscall.Exec("codex", nil, nil); syscall.ForkExec("evolve", nil, nil); syscall.ForkExec("agy", nil, nil) }
const pinned = "codex"
func g9() { sysexec.Command(ctx, pinned); sysexec.Command(ctx, "sh", "-c", "ls -la"); llm := "claude"; _ = llm }
func g10() { sysexec.Command(ctx, llm); sysexec.Output(ctx, run) }
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
		"internal/bridge/b.go:7: agyDriver.Launch: Runner",
		"internal/bridge/driver_x.go:2: xLaunch: launchCmdLine",
		"internal/bridge/driver_x.go:2: xLaunch: Runner",
		"internal/modelquery/exec.go:2: d: sysexec.Command",
		"internal/x/x.go:3: a: sysexec.Command claude",
		"internal/x/x.go:3: a: exec.Command newllm",
		"internal/x/x.go:3: a: exec.CommandContext agy",
		"internal/x/x.go:4: b: RunFunc call codex",
		"internal/x/x.go:6: e: sysexec.Command claude",
		"internal/y/y.go:3: g: exec.Command claude",
		"internal/z/z.go:3: g1: sysexec.Output claude",
		"internal/z/z.go:3: g1: sysexec.Capture codex",
		"internal/z/z.go:3: g1: sysexec.CombinedOutput agy",
		"internal/z/z.go:4: g2: sysexec.Command bin",
		"internal/z/z.go:4: g2: sysexec.Command .Binary",
		"internal/z/z.go:5: g3: sysexec.Command p",
		"internal/z/z.go:6: g4: sysexec.Command sh claude",
		"internal/z/z.go:6: g4: sysexec.Command /bin/bash agy",
		"internal/z/z.go:7: g5: sysexec.Command tmux claude",
		"internal/z/z.go:7: g5: SendKeys codex",
		"internal/z/z.go:8: g7: RunFunc call agy",
		"internal/z/z.go:9: g8: os.StartProcess claude",
		"internal/z/z.go:9: g8: syscall.Exec codex",
		"internal/z/z.go:9: g8: syscall.ForkExec agy",
		"internal/z/z.go:11: g9: sysexec.Command pinned",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("spawns =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if _, err := llmSpawns(root, []string{"absent.go"}, bins); err == nil {
		t.Fatal("a file that does not parse must fail, not report no spawn")
	}
}
