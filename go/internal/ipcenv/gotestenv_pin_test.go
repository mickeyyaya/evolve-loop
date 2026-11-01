package ipcenv

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

var (
	spawnCallee        = regexp.MustCompile(`(?i)command|capture|output|run|exec`)
	programSpawnCallee = regexp.MustCompile(`(?i)command|capture|output|exec`)
)

var skippedTreeDirs = []string{"acs", "testdata", "vendor", "bin", ".git"}

var exemptSpawners = map[string]string{
	"internal/core/ship_recovery.go:worktreeGenerator":               "runs the worktree's own evolve source with go run; WorktreeEvolveInvocation owns that env",
	"internal/gotooltest/cache.go:ExecRunner":                        "runs go build only; the Cache composes every argv from build",
	"internal/acssuite/acssuite.go:defaultGoExec":                    "receives the env predicateEnv built, operator keys and suite exports included; predicateEnv sits on a protected control-plane path (ADR-0064) and moves to ipcenv.GoTestEnv as console work",
	"internal/commitgate/lanes.go:laneGo":                            "protected control-plane path (ADR-0064): a lane cannot edit it; console work routes its go test through sysexec.WithEnv(ipcenv.GoTestEnv())",
	"internal/core/build_floor_reviewer.go:runCoverPass":             "protected control-plane path (ADR-0064): console work swaps ipcenv.Scrub(os.Environ()) for ipcenv.GoTestEnv()",
	"internal/core/phase_bindings_selfcheck.go:realGoUnitTest":       "protected control-plane path (ADR-0064): console work swaps ipcenv.Scrub(os.Environ()) for ipcenv.GoTestEnv()",
	"internal/core/phase_bindings_selfcheck.go:realGoUnitTestTagged": "protected control-plane path (ADR-0064): console work swaps ipcenv.Scrub(os.Environ()) for ipcenv.GoTestEnv()",
	"internal/phases/audit/ciparitygate/command.go:ACSDurable":       "protected control-plane path (ADR-0064): console work wraps its runner in scrubbedRun like the other gates",
	"internal/phases/audit/ciparitygate/tier.go:runAttempt":          "runs the runner IntegrationTier wrapped once in scrubbedRun, so both attempts carry one captured ciparity.CIEnv",
}

type parsedFile struct {
	path string
	pkg  string
	fset *token.FileSet
	file *ast.File
}

type spawner struct {
	path      string
	function  string
	line      int
	projected bool
}

func parseSources(t *testing.T, sources map[string]string) []parsedFile {
	t.Helper()
	var files []parsedFile
	for path, src := range sources {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		files = append(files, parsedFile{path: path, pkg: file.Name.Name, fset: fset, file: file})
	}
	return files
}

func parseTree(t *testing.T, root string) []parsedFile {
	t.Helper()
	var files []parsedFile
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && slices.Contains(skippedTreeDirs, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, parsedFile{path: filepath.ToSlash(rel), pkg: file.Name.Name, fset: fset, file: file})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func calleeName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	}
	return ""
}

func qualifier(call *ast.CallExpr) string {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok {
			return id.Name
		}
	}
	return ""
}

func isProjectionCall(call *ast.CallExpr, pkg string) bool {
	switch name, qual := calleeName(call), qualifier(call); {
	case name == "GoTestEnv" && (qual == "ipcenv" || (qual == "" && pkg == "ipcenv")):
		return true
	case name == "CIEnv" && (qual == "ciparity" || (qual == "" && pkg == "ciparity")):
		return true
	}
	return false
}

func containsCall(node ast.Node, match func(*ast.CallExpr) bool) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && match(call) {
			found = true
		}
		return !found
	})
	return found
}

func stringValue(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	return value, err == nil
}

func isStringSlice(e ast.Expr) bool {
	arr, ok := e.(*ast.ArrayType)
	if !ok || arr.Len != nil {
		return false
	}
	id, ok := arr.Elt.(*ast.Ident)
	return ok && id.Name == "string"
}

func stringSliceLiteral(e ast.Expr) (*ast.CompositeLit, bool) {
	lit, ok := e.(*ast.CompositeLit)
	return lit, ok && isStringSlice(lit.Type)
}

func returnsStringSlice(fn *ast.FuncDecl) bool {
	results := fn.Type.Results
	return results != nil && len(results.List) == 1 && isStringSlice(results.List[0].Type)
}

func testArgBuilders(files []parsedFile) map[string]bool {
	builders := map[string]bool{}
	for _, f := range files {
		for _, decl := range f.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || !returnsStringSlice(fn) {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				lit, ok := stringSliceLiteral(asExpr(n))
				if ok && len(lit.Elts) > 0 {
					if first, ok := stringValue(lit.Elts[0]); ok && first == "test" {
						builders[fn.Name.Name] = true
					}
				}
				return true
			})
		}
	}
	return builders
}

func asExpr(n ast.Node) ast.Expr {
	e, _ := n.(ast.Expr)
	return e
}

func projectionFuncs(files []parsedFile) map[string]bool {
	funcs := map[string]bool{}
	for _, f := range files {
		for _, decl := range f.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Body != nil && containsCall(fn.Body, func(c *ast.CallExpr) bool { return isProjectionCall(c, f.pkg) }) {
				funcs[fn.Name.Name] = true
			}
		}
	}
	return funcs
}

func localDefinitions(node ast.Node) map[string]ast.Expr {
	defs := map[string]ast.Expr{}
	ast.Inspect(node, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			if len(s.Lhs) == len(s.Rhs) {
				for i, lhs := range s.Lhs {
					if id, ok := lhs.(*ast.Ident); ok && defs[id.Name] == nil {
						defs[id.Name] = s.Rhs[i]
					}
				}
			}
		case *ast.ValueSpec:
			if len(s.Names) == len(s.Values) {
				for i, id := range s.Names {
					if defs[id.Name] == nil {
						defs[id.Name] = s.Values[i]
					}
				}
			}
		}
		return true
	})
	return defs
}

type classifier struct {
	builders map[string]bool
	defs     map[string]ast.Expr
}

func (c classifier) subcommand(e ast.Expr, depth int) (string, bool) {
	if depth > 6 {
		return "", false
	}
	switch v := e.(type) {
	case *ast.ParenExpr:
		return c.subcommand(v.X, depth+1)
	case *ast.BasicLit:
		return stringValue(v)
	case *ast.CompositeLit:
		if _, ok := stringSliceLiteral(v); ok && len(v.Elts) > 0 {
			return stringValue(v.Elts[0])
		}
	case *ast.CallExpr:
		name := calleeName(v)
		switch {
		case c.builders[name]:
			return "test", true
		case (name == "append" || (name == "Args" && qualifier(v) == "lanebudget")) && len(v.Args) > 0:
			return c.subcommand(v.Args[0], depth+1)
		}
	case *ast.Ident:
		if def, ok := c.defs[v.Name]; ok {
			return c.subcommand(def, depth+1)
		}
	}
	return "", false
}

func goComposite(e ast.Expr) ([]ast.Expr, bool) {
	if call, ok := e.(*ast.CallExpr); ok && calleeName(call) == "append" && len(call.Args) > 0 {
		if lit, ok := stringSliceLiteral(call.Args[0]); ok && len(lit.Elts) > 0 {
			if first, ok := stringValue(lit.Elts[0]); ok && first == "go" {
				return call.Args[1:], true
			}
		}
		return nil, false
	}
	if lit, ok := stringSliceLiteral(e); ok && len(lit.Elts) > 0 {
		if first, ok := stringValue(lit.Elts[0]); ok && first == "go" {
			return lit.Elts[1:], true
		}
	}
	return nil, false
}

func isSubcommandCandidate(call *ast.CallExpr, arg ast.Expr, last bool) bool {
	switch v := arg.(type) {
	case *ast.BasicLit:
		value, ok := stringValue(v)
		return ok && value != ""
	case *ast.CompositeLit, *ast.CallExpr:
		return true
	case *ast.Ident, *ast.SelectorExpr:
		return last && call.Ellipsis.IsValid()
	}
	return false
}

func (c classifier) firstSubcommand(call *ast.CallExpr, rest []ast.Expr) (string, bool, bool) {
	for i, arg := range rest {
		if isSubcommandCandidate(call, arg, i == len(rest)-1) {
			sub, known := c.subcommand(arg, 0)
			return sub, known, true
		}
	}
	return "", false, false
}

func followsFlag(args []ast.Expr, i int) bool {
	if i == 0 {
		return false
	}
	prev, ok := stringValue(args[i-1])
	return ok && strings.HasPrefix(prev, "-")
}

func (c classifier) spawnsGoTest(call *ast.CallExpr) bool {
	if !spawnCallee.MatchString(calleeName(call)) {
		return false
	}
	for i, arg := range call.Args {
		if value, ok := stringValue(arg); ok && value == "go" && !followsFlag(call.Args, i) {
			sub, known, found := c.firstSubcommand(call, call.Args[i+1:])
			return found && (!known || sub == "test")
		}
		if rest, ok := goComposite(arg); ok {
			sub, known, found := c.firstSubcommand(call, rest)
			return found && (!known || sub == "test")
		}
	}
	if !programSpawnCallee.MatchString(calleeName(call)) {
		return false
	}
	for i, arg := range call.Args {
		if _, literal := arg.(*ast.BasicLit); literal {
			continue
		}
		if sub, known := c.subcommand(arg, 0); known && sub == "test" && isSubcommandCandidate(call, arg, i == len(call.Args)-1) {
			return true
		}
	}
	return false
}

func findSpawners(files []parsedFile) []spawner {
	builders := testArgBuilders(files)
	projectors := projectionFuncs(files)
	var found []spawner
	for _, f := range files {
		for _, decl := range f.file.Decls {
			name, body := declBody(decl)
			if body == nil {
				continue
			}
			c := classifier{builders: builders, defs: localDefinitions(body)}
			line := 0
			ast.Inspect(body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok && line == 0 && c.spawnsGoTest(call) {
					line = f.fset.Position(call.Pos()).Line
				}
				return true
			})
			if line == 0 {
				continue
			}
			projected := projectsWithoutAmbientEnv(body, func(call *ast.CallExpr) bool {
				return isProjectionCall(call, f.pkg) || projectors[calleeName(call)]
			})
			found = append(found, spawner{path: f.path, function: name, line: line, projected: projected})
		}
	}
	return found
}

func isAmbientEnv(call *ast.CallExpr) bool {
	return calleeName(call) == "Environ" && qualifier(call) == "os"
}

func projectsWithoutAmbientEnv(body ast.Node, projects func(*ast.CallExpr) bool) bool {
	projected, ambient := false, false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		switch {
		case !ok:
		case projects(call):
			projected = true
			return false
		case isAmbientEnv(call):
			ambient = true
		}
		return true
	})
	return projected && !ambient
}

func declBody(decl ast.Decl) (string, ast.Node) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if d.Body == nil {
			return "", nil
		}
		return d.Name.Name, d.Body
	case *ast.GenDecl:
		if d.Tok != token.VAR || len(d.Specs) == 0 {
			return "", nil
		}
		if spec, ok := d.Specs[0].(*ast.ValueSpec); ok && len(spec.Names) > 0 {
			return spec.Names[0].Name, d
		}
	}
	return "", nil
}

func spawnerViolations(files []parsedFile) []string {
	var out []string
	for _, s := range findSpawners(files) {
		if s.projected {
			continue
		}
		if _, exempt := exemptSpawners[s.path+":"+s.function]; exempt {
			continue
		}
		out = append(out, s.path+":"+s.function+" spawns go test without building its env from ipcenv.GoTestEnv or ciparity.CIEnv")
	}
	slices.Sort(out)
	return out
}

const plantedFixture = "package planted\n"

func plantedViolations(t *testing.T, sources map[string]string) []string {
	t.Helper()
	return spawnerViolations(parseSources(t, sources))
}

func TestGoTestSpawners_APlantedSpawnerThatBuildsItsEnvAnyOtherWayIsFlagged(t *testing.T) {
	cases := map[string]string{
		"inherits the process env": plantedFixture + `
import ("os/exec")
func plant() { cmd := exec.Command("go", "test", "./..."); _ = cmd.Run() }`,
		"passes os.Environ": plantedFixture + `
import ("os"; "os/exec")
func plant() { cmd := exec.Command("go", "test", "./..."); cmd.Env = os.Environ(); _ = cmd.Run() }`,
		"scrubs with the bare Scrub projection": plantedFixture + `
import ("os"; "github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"; "github.com/mickeyyaya/evolve-loop/go/internal/sysexec")
func plant(ctx context.Context) { cmd := sysexec.Command(ctx, "go", "test", "./..."); cmd.Env = ipcenv.Scrub(os.Environ()); _ = cmd.Run() }`,
		"runner capture with a parameter argv": plantedFixture + `
import ("github.com/mickeyyaya/evolve-loop/go/internal/sysexec")
func plant(ctx context.Context, run sysexec.RunFunc, args []string) { _, _, _, _ = sysexec.Capture(ctx, run, dir, "go", args...) }`,
		"argv from a test-args builder": plantedFixture + `
import ("github.com/mickeyyaya/evolve-loop/go/internal/sysexec")
func testArgs() []string { return []string{"test", "-count=1"} }
func plant(ctx context.Context) { cmd := sysexec.Command(ctx, "go", testArgs()...); _ = cmd.Run() }`,
		"program resolved at run time": plantedFixture + `
import ("github.com/mickeyyaya/evolve-loop/go/internal/sysexec")
func plant(ctx context.Context, bin string, extra []string) { cmd := sysexec.Command(ctx, bin, append([]string{"test"}, extra...)...); _ = cmd.Run() }`,
		"go composite argv": plantedFixture + `
func plant(ctx context.Context) { execCommand(ctx, append([]string{"go"}, lanebudget.Args([]string{"test", "-count=1"})...)...) }`,
		"argv from a local test slice": plantedFixture + `
import ("github.com/mickeyyaya/evolve-loop/go/internal/sysexec")
func plant(ctx context.Context, pkgs []string) { args := append([]string{"test", "-count=1"}, pkgs...); cmd := sysexec.Command(ctx, "go", args...); _ = cmd.Run() }`,
		"decoy projection beside an ambient env": plantedFixture + `
import ("os"; "os/exec"; "github.com/mickeyyaya/evolve-loop/go/internal/ipcenv")
func plant() { cmd := exec.Command("go", "test", "./..."); cmd.Env = os.Environ(); _ = ipcenv.GoTestEnv(); _ = cmd.Run() }`,
		"ambient env appended back to the projection": plantedFixture + `
import ("os"; "os/exec"; "github.com/mickeyyaya/evolve-loop/go/internal/ipcenv")
func plant() { cmd := exec.Command("go", "test", "./..."); cmd.Env = append(ipcenv.GoTestEnv(), os.Environ()...); _ = cmd.Run() }`,
		"package level runner var": plantedFixture + `
var plantedRun = func(ctx context.Context) { cmd := sysexec.Command(ctx, "go", "test", "./..."); _ = cmd.Run() }`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			got := plantedViolations(t, map[string]string{"internal/planted/planted.go": src})
			if len(got) != 1 {
				t.Fatalf("a planted spawner that %s was reported as %q, want exactly one violation", name, got)
			}
		})
	}
}

func TestGoTestSpawners_ASpawnerThatUsesANamedProjectionIsAccepted(t *testing.T) {
	cases := map[string]map[string]string{
		"ipcenv.GoTestEnv": {"internal/planted/a.go": plantedFixture + `
import ("github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"; "github.com/mickeyyaya/evolve-loop/go/internal/sysexec")
func plant(ctx context.Context) { cmd := sysexec.Command(ctx, "go", "test", "./..."); cmd.Env = ipcenv.GoTestEnv("EVOLVE_X=1"); _ = cmd.Run() }`},
		"ciparity.CIEnv": {"internal/planted/a.go": plantedFixture + `
import ("os"; "github.com/mickeyyaya/evolve-loop/go/internal/ciparity"; "github.com/mickeyyaya/evolve-loop/go/internal/sysexec")
func plant(ctx context.Context) { cmd := sysexec.Command(ctx, "go", "test", "./..."); cmd.Env = ciparity.CIEnv(os.Environ()); _ = cmd.Run() }`},
		"a wrapper that projects": {
			"internal/planted/a.go": plantedFixture + `
func scrubbedRun(run sysexec.RunFunc) sysexec.RunFunc { clean := ciparity.CIEnv(os.Environ()); return func() { _ = clean } }`,
			"internal/planted/b.go": plantedFixture + `
func plant(ctx context.Context, run sysexec.RunFunc) { _, _, _, _ = sysexec.Capture(ctx, scrubbedRun(run), dir, "go", "test", "./...") }`,
		},
		"the projection inside ipcenv itself": {"internal/ipcenv/a.go": "package ipcenv\n" + `
func plant(ctx context.Context) { cmd := sysexec.Command(ctx, "go", "test", "./..."); cmd.Env = GoTestEnv(); _ = cmd.Run() }`},
	}
	for name, sources := range cases {
		t.Run(name, func(t *testing.T) {
			if got := plantedViolations(t, sources); len(got) != 0 {
				t.Fatalf("%s was flagged: %q", name, got)
			}
		})
	}
}

func TestGoTestSpawners_ACommandThatIsNotGoTestIsLeftAlone(t *testing.T) {
	cases := map[string]string{
		"go list":           `func plant(ctx context.Context) { _ = sysexec.Command(ctx, "go", "list", "./...") }`,
		"go vet":            `func plant(ctx context.Context) { _ = sysexec.Command(ctx, "go", "vet", "./...") }`,
		"go tool cover":     `func plant(ctx context.Context) { _ = sysexec.Command(ctx, "go", "tool", "cover", "-func=x") }`,
		"go env by runner":  `func plant(ctx context.Context) { _, _ = sysexec.DefaultRunner(ctx, "go", "", []string{"env", "GOCACHE"}, nil, nil, nil, nil) }`,
		"git test word":     `func plant(ctx context.Context) { _ = sysexec.Command(ctx, "git", "test") }`,
		"make test target":  `func plant(ctx context.Context) { _ = sysexec.Command(ctx, "make", "-C", "go", "test") }`,
		"a path with go":    `func plant(root string) string { return filepath.Join(root, "go", "bin", "evolve") }`,
		"slash join":        `func plant(rel string) string { return path.Join("go", filepath.ToSlash(rel)) }`,
		"local list slice":  `func plant(ctx context.Context, p []string) { a := append([]string{"list", "-e"}, p...); _ = sysexec.Command(ctx, "go", a...) }`,
		"lane budget table": `func plant(args []string) []string { return lanebudget.Args([]string{"test", "-count=1"}) }`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if got := plantedViolations(t, map[string]string{"internal/planted/a.go": plantedFixture + body}); len(got) != 0 {
				t.Fatalf("%s was flagged: %q", name, got)
			}
		})
	}
}

func TestGoTestSpawners_ATreeScanReadsProductionFilesOnly(t *testing.T) {
	root := t.TempDir()
	plant := plantedFixture + `
import ("os/exec")
func plant() { _ = exec.Command("go", "test", "./...").Run() }`
	for _, rel := range []string{
		"internal/p/planted.go",
		"internal/p/planted_test.go",
		"acs/cycle1/planted.go",
		"testdata/planted.go",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(plant), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := spawnerViolations(parseTree(t, root))
	want := []string{"internal/p/planted.go:plant spawns go test without building its env from ipcenv.GoTestEnv or ciparity.CIEnv"}
	if !slices.Equal(got, want) {
		t.Fatalf("tree scan = %q, want %q", got, want)
	}
}

func TestGoTestSpawners_TheTreeScanFindsEveryKnownProductionSpawner(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, s := range findSpawners(parseTree(t, root)) {
		seen[s.path] = true
	}
	for _, path := range []string{
		"internal/core/task_contract.go",
		"internal/core/build_floor_reviewer.go",
		"internal/core/phase_bindings_selfcheck.go",
		"internal/acssuite/acssuite.go",
		"internal/acsrunner/runner.go",
		"internal/commitgate/lanes.go",
		"internal/phases/ship/repocontract.go",
		"internal/phases/audit/ciparitygate/command.go",
		"internal/phases/audit/ciparitygate/tier.go",
		"internal/releasepreflight/releasepreflight.go",
		"pkg/acsassert/go_tests.go",
	} {
		if !seen[path] {
			t.Errorf("the scan does not see %s as a go test spawner; its env would go unchecked", path)
		}
	}
}

func TestGoTestSpawners_EveryExemptionStillNamesAnUnprojectedSpawner(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	live := map[string]bool{}
	for _, s := range findSpawners(parseTree(t, root)) {
		if !s.projected {
			live[s.path+":"+s.function] = true
		}
	}
	for key, reason := range exemptSpawners {
		if reason == "" || !live[key] {
			t.Errorf("exemption %s is stale or has no reason; remove it", key)
		}
	}
}

func TestGoTestSpawners_EveryProductionSpawnerUsesTheNamedProjection(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, violation := range spawnerViolations(parseTree(t, root)) {
		t.Error(violation)
	}
}
