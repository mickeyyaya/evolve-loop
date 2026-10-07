//go:build acs

package cycle1821

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const sizeRatchetOffenders = "go/internal/sizeratchet/offenders.json"

func llmroutePackage(t *testing.T) []*ast.File {
	t.Helper()
	dir := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "llmroute")
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("listing %s: %v (found %d files)", dir, err, len(paths))
	}
	var files []*ast.File
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parsing %s: %v", p, err)
		}
		files = append(files, f)
	}
	return files
}

func funcDecls(files []*ast.File) map[string]*ast.FuncDecl {
	decls := map[string]*ast.FuncDecl{}
	for _, f := range files {
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Body != nil {
				decls[fn.Name.Name] = fn
			}
		}
	}
	return decls
}

func isBlankDiscardOf(stmt ast.Node, name string) bool {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return false
	}
	lhs, lok := assign.Lhs[0].(*ast.Ident)
	rhs, rok := assign.Rhs[0].(*ast.Ident)
	return lok && rok && lhs.Name == "_" && rhs.Name == name
}

func paramIsUsed(body *ast.BlockStmt, name string) bool {
	used := false
	ast.Inspect(body, func(n ast.Node) bool {
		if used || isBlankDiscardOf(n, name) {
			return false
		}
		if id, ok := n.(*ast.Ident); ok && id.Name == name {
			used = true
		}
		return !used
	})
	return used
}

func unusedParameters(decls map[string]*ast.FuncDecl) []string {
	var unused []string
	for name, fn := range decls {
		for _, field := range fn.Type.Params.List {
			for _, p := range field.Names {
				if p.Name != "_" && !paramIsUsed(fn.Body, p.Name) {
					unused = append(unused, name+"("+p.Name+")")
				}
			}
		}
	}
	sort.Strings(unused)
	return unused
}

func universalFallbackArgs(fnType reflect.Type, plan llmroute.Plan, discovered []string) []reflect.Value {
	args := make([]reflect.Value, fnType.NumIn())
	for i := range args {
		switch fnType.In(i) {
		case reflect.TypeOf(llmroute.Plan{}):
			args[i] = reflect.ValueOf(plan)
		case reflect.TypeOf([]string(nil)):
			args[i] = reflect.ValueOf(discovered)
		default:
			args[i] = reflect.Zero(fnType.In(i))
		}
	}
	return args
}

func TestC1821_004_UniversalFallbackPathHasNoUnusedParameter(t *testing.T) {
	fn := reflect.ValueOf(llmroute.ApplyUniversalFallback)
	fnType := fn.Type()
	lookPathType := reflect.TypeOf(func(string) (string, error) { return "", nil })
	for i := 0; i < fnType.NumIn(); i++ {
		if fnType.In(i) == lookPathType {
			t.Errorf("ApplyUniversalFallback still takes a %v parameter (position %d) it never uses", lookPathType, i)
		}
	}
	if fnType.NumIn() != 2 || fnType.In(0) != reflect.TypeOf(llmroute.Plan{}) || fnType.In(1) != reflect.TypeOf([]string(nil)) {
		t.Errorf("ApplyUniversalFallback signature is %v, want func(llmroute.Plan, []string) llmroute.Plan", fnType)
	}

	plan := llmroute.Plan{Candidates: []string{"codex-tmux", "claude-tmux"}, Triggers: []int{80}, Model: "deep"}
	out := fn.Call(universalFallbackArgs(fnType, plan, []string{"claude-tmux", "agy-tmux"}))[0].Interface().(llmroute.Plan)
	if !slices.Equal(out.Candidates, []string{"codex-tmux", "claude-tmux", "agy-tmux"}) {
		t.Errorf("discovered tail: Candidates=%v, want [codex-tmux claude-tmux agy-tmux] (deduped, appended)", out.Candidates)
	}
	if out.Model != "deep" || !slices.Equal(out.Triggers, []int{80}) || !slices.Equal(plan.Candidates, []string{"codex-tmux", "claude-tmux"}) {
		t.Errorf("ApplyUniversalFallback must carry other fields and leave its input alone: out=%+v in=%+v", out, plan)
	}
	unchanged := fn.Call(universalFallbackArgs(fnType, plan, nil))[0].Interface().(llmroute.Plan)
	if !slices.Equal(unchanged.Candidates, plan.Candidates) {
		t.Errorf("no discovered CLIs must leave the chain as configured, got %v", unchanged.Candidates)
	}

	if unused := unusedParameters(funcDecls(llmroutePackage(t))); len(unused) > 0 {
		t.Errorf("llmroute functions with a parameter they never use: %v", unused)
	}
}

type benchCase struct {
	name       string
	apply      func(llmroute.Plan, map[string]time.Time) llmroute.Plan
	candidates []string
	benched    map[string]time.Time
	want       []string
}

func benchCases() []benchCase {
	older, newer := time.Unix(1_700_000_000, 0), time.Unix(1_700_000_600, 0)
	return []benchCase{
		{"family bench demotes every driver of the family", llmroute.ApplyBench, []string{"codex-tmux", "claude-tmux", "agy-tmux"}, map[string]time.Time{"codex": newer}, []string{"claude-tmux", "agy-tmux", "codex-tmux"}},
		{"family bench keys a bare family by itself", llmroute.ApplyBench, []string{"codex", "claude-tmux"}, map[string]time.Time{"codex": newer}, []string{"claude-tmux", "codex"}},
		{"family bench all benched runs least recent first", llmroute.ApplyBench, []string{"codex-tmux", "claude-tmux"}, map[string]time.Time{"codex": newer, "claude": older}, []string{"claude-tmux", "codex-tmux"}},
		{"family bench ignores a driver-name key", llmroute.ApplyBench, []string{"codex-tmux", "claude-tmux"}, map[string]time.Time{"codex-tmux": newer}, []string{"codex-tmux", "claude-tmux"}},
		{"driver bench demotes only that driver", llmroute.ApplyDriverBench, []string{"codex-tmux", "claude-tmux", "agy-tmux"}, map[string]time.Time{"codex-tmux": newer}, []string{"claude-tmux", "agy-tmux", "codex-tmux"}},
		{"driver bench never demotes by family", llmroute.ApplyDriverBench, []string{"codex", "claude-tmux"}, map[string]time.Time{"codex-tmux": newer}, []string{"codex", "claude-tmux"}},
		{"driver bench all benched runs least recent first", llmroute.ApplyDriverBench, []string{"codex-tmux", "claude-tmux"}, map[string]time.Time{"codex-tmux": newer, "claude-tmux": older}, []string{"claude-tmux", "codex-tmux"}},
		{"family bench single candidate is a no-op", llmroute.ApplyBench, []string{"codex-tmux"}, map[string]time.Time{"codex": newer}, []string{"codex-tmux"}},
		{"driver bench empty bench is a no-op", llmroute.ApplyDriverBench, []string{"codex-tmux", "claude-tmux"}, nil, []string{"codex-tmux", "claude-tmux"}},
	}
}

func requireBenchBehaviour(t *testing.T) {
	t.Helper()
	for _, tc := range benchCases() {
		in := llmroute.Plan{Candidates: slices.Clone(tc.candidates), Triggers: []int{80}, Model: "deep"}
		out := tc.apply(in, tc.benched)
		if !slices.Equal(out.Candidates, tc.want) {
			t.Errorf("%s: Candidates=%v, want %v", tc.name, out.Candidates, tc.want)
		}
		if out.Model != "deep" || !slices.Equal(out.Triggers, []int{80}) || !slices.Equal(in.Candidates, tc.candidates) {
			t.Errorf("%s: must carry other Plan fields and leave its input alone: out=%+v in=%+v", tc.name, out, in)
		}
	}
}

var sortCalls = map[string][]string{
	"sort":   {"Slice", "SliceStable", "Sort", "Stable"},
	"slices": {"Sort", "SortFunc", "SortStableFunc"},
}

func performsSort(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return !found
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return !found
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && slices.Contains(sortCalls[pkg.Name], sel.Sel.Name) {
			found = true
		}
		return !found
	})
	return found
}

func hasRangeLoop(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if _, ok := n.(*ast.RangeStmt); ok {
			found = true
		}
		return !found
	})
	return found
}

func directCallees(fn *ast.FuncDecl, decls map[string]*ast.FuncDecl) []string {
	var callees []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && decls[id.Name] != nil && !slices.Contains(callees, id.Name) {
				callees = append(callees, id.Name)
			}
		}
		return true
	})
	return callees
}

func sortersReachableFrom(root string, decls map[string]*ast.FuncDecl) []string {
	reached := []string{root}
	frontier := []string{root}
	for hop := 0; hop < 2; hop++ {
		var next []string
		for _, name := range frontier {
			for _, callee := range directCallees(decls[name], decls) {
				if !slices.Contains(reached, callee) {
					reached = append(reached, callee)
					next = append(next, callee)
				}
			}
		}
		frontier = next
	}
	var sorters []string
	for _, name := range reached {
		if performsSort(decls[name]) {
			sorters = append(sorters, name)
		}
	}
	sort.Strings(sorters)
	return sorters
}

func TestC1821_005_BenchDemotionIsOneImplementationKeyedByStrategy(t *testing.T) {
	requireBenchBehaviour(t)

	decls := funcDecls(llmroutePackage(t))
	for _, exported := range []string{"ApplyBench", "ApplyDriverBench"} {
		if decls[exported] == nil {
			t.Fatalf("llmroute.%s is gone; both exported bench entry points must stay for bridgechain and usageprobe", exported)
		}
	}
	familySorters := sortersReachableFrom("ApplyBench", decls)
	driverSorters := sortersReachableFrom("ApplyDriverBench", decls)
	if len(familySorters) != 1 || !slices.Equal(familySorters, driverSorters) {
		t.Fatalf("ApplyBench reaches sort logic in %v and ApplyDriverBench in %v; both must delegate to one shared bench function parameterized by its key", familySorters, driverSorters)
	}
	shared := familySorters[0]
	for _, exported := range []string{"ApplyBench", "ApplyDriverBench"} {
		if exported != shared && hasRangeLoop(decls[exported]) {
			t.Errorf("%s still carries its own demotion loop beside the shared %s; the healthy/demoted partition must live in one function", exported, shared)
		}
	}
}

type rootedGit struct{ root string }

func (g rootedGit) run(args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("git", append([]string{"-C", g.root}, args...)...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func (g rootedGit) ChangedFiles(base string) ([]string, error) {
	tracked, err := g.run("diff", "--name-only", "--no-renames", base)
	if err != nil {
		return nil, err
	}
	untracked, err := g.run("ls-files", "--others", "--exclude-standard", "--full-name", ":/")
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(tracked) + "\n" + string(untracked)), nil
}

func (g rootedGit) Show(base, path string) ([]byte, error) {
	return commentaudit.ReadAtBase(g.run, base)(path)
}

func (g rootedGit) Root() (string, error) {
	out, err := g.run("rev-parse", "--show-toplevel")
	return strings.TrimSpace(string(out)), err
}

func cycleBase(t *testing.T, g rootedGit) string {
	t.Helper()
	out, err := g.run("merge-base", "main", "HEAD")
	if err != nil {
		t.Fatalf("resolving the cycle's base against main: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func TestC1821_006_TheCycleAddsNoComments(t *testing.T) {
	g := rootedGit{root: acsassert.RepoRoot(t)}
	base := cycleBase(t, g)
	changed, err := g.ChangedFiles(base)
	if err != nil {
		t.Fatalf("listing the cycle's changed files: %v", err)
	}
	if !slices.ContainsFunc(changed, func(f string) bool {
		return strings.HasPrefix(f, "go/internal/llmroute/") && strings.HasSuffix(f, ".go") && !strings.HasSuffix(f, "_test.go")
	}) {
		t.Errorf("no production Go file under go/internal/llmroute changed since %s; the trigger-aliasing fix has not landed", base)
	}

	var stdout, stderr bytes.Buffer
	code := commentaudit.Main([]string{"comments", "-base", base}, &stdout, &stderr, g)
	if code != 0 {
		t.Errorf("commentaudit comments -base %s exit=%d (want 0: the change adds no comments)\nstdout:\n%s\nstderr:\n%s", base, code, stdout.String(), stderr.String())
	}
}

func TestC1821_007_SizeRatchetOffendersStayUntouched(t *testing.T) {
	g := rootedGit{root: acsassert.RepoRoot(t)}
	base := cycleBase(t, g)
	changed, err := g.ChangedFiles(base)
	if err != nil {
		t.Fatalf("listing the cycle's changed files: %v", err)
	}
	if slices.Contains(changed, sizeRatchetOffenders) {
		t.Errorf("%s changed since %s; a size-ratchet exemption is not part of this fix", sizeRatchetOffenders, base)
	}
}
