package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/publisher"
)

func parsedModuleSources(t *testing.T) map[string]*ast.File {
	t.Helper()
	moduleRoot := filepath.Join("..", "..")
	files := map[string]*ast.File{}
	err := filepath.WalkDir(moduleRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if name == "vendor" || name == "bin" || name == "testdata" || strings.HasPrefix(name, ".") && path != moduleRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		files[filepath.ToSlash(rel)] = file
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func calleeName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		if id, ok := f.X.(*ast.Ident); ok {
			return id.Name + "." + f.Sel.Name
		}
	}
	return ""
}

func functionsOf(file *ast.File) []*ast.FuncDecl {
	var fns []*ast.FuncDecl
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Body != nil {
			fns = append(fns, fd)
		}
	}
	return fns
}

func callSitesOf(files map[string]*ast.File, callees ...string) map[string][]string {
	sites := map[string][]string{}
	for rel, file := range files {
		for _, fd := range functionsOf(file) {
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok && slices.Contains(callees, calleeName(call.Fun)) {
					site := rel + ":" + fd.Name.Name
					sites[calleeName(call.Fun)] = append(sites[calleeName(call.Fun)], site)
				}
				return true
			})
		}
	}
	for callee := range sites {
		slices.Sort(sites[callee])
		sites[callee] = slices.Compact(sites[callee])
	}
	return sites
}

func TestRootSignalCenter_EveryRootSubscribesThePublisher(t *testing.T) {
	t.Parallel()
	files := parsedModuleSources(t)
	want := map[string][]string{
		"signalcenter.New": {
			"cmd/evolve/cmd_cycle.go:newRootSignalCenter",
			"internal/cli/phasecmd/phase_observer.go:observerSignalCenter",
		},
		"newRootSignalCenter": {"cmd/evolve/cmd_cycle.go:newRoleSignalCenter"},
		"newRoleSignalCenter": {
			"cmd/evolve/cmd_cycle.go:wireOrchestratorDeps",
			"cmd/evolve/cmd_cycle_simulate.go:wireSimulateOrchestrator",
			"cmd/evolve/cmd_loop_chain.go:maybeRefreshChainBoundary",
			"cmd/evolve/cmd_loop_chain.go:runLoopChain",
			"cmd/evolve/cmd_ship.go:shipRootSignals",
			"cmd/evolve/cmd_subagent.go:runSubagentRun",
		},
		"attachRootEventsFn": {"cmd/evolve/cmd_cycle.go:newRoleSignalCenter"},
		"rootwire.Attach":    {"internal/cli/phasecmd/phase_observer.go:observerSignalCenter"},
	}

	got := callSitesOf(files, "signalcenter.New", "newRootSignalCenter", "newRoleSignalCenter", "attachRootEventsFn", "rootwire.Attach")

	for callee, sites := range want {
		if !slices.Equal(got[callee], sites) {
			t.Errorf("%s is called from %v, want exactly the roots %v: a root that builds a Center without the Publisher, or a new root that is not in this list, is a lane whose events reach no channel", callee, got[callee], sites)
		}
	}
	if !seamDefaultsTo(files["cmd/evolve/cmd_cycle.go"], "attachRootEventsFn", "rootwire.Attach") {
		t.Error("attachRootEventsFn must default to rootwire.Attach: the seam is for tests, and production keeps the real Publisher")
	}
}

func seamDefaultsTo(file *ast.File, name, value string) bool {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)
			if len(vs.Names) == 1 && vs.Names[0].Name == name && len(vs.Values) == 1 && calleeName(vs.Values[0]) == value {
				return true
			}
		}
	}
	return false
}

func TestRootSignalCenter_EveryRootRunsUnderARoleOfTheClosedSet(t *testing.T) {
	t.Parallel()
	roles := []string{roleCycle, roleLoop, roleLoopChain, roleShip, roleSubagent, roleSimulate}

	for _, role := range roles {
		if !slices.Contains(publisher.Roles(), role) {
			t.Errorf("role %q is not one of publisher.Roles() %v", role, publisher.Roles())
		}
	}
	distinct := slices.Clone(roles)
	slices.Sort(distinct)
	if len(slices.Compact(distinct)) != len(roles) {
		t.Errorf("roles %v repeat: each Center of a process needs its own role, because the reader keys gaps on (source, pid)", roles)
	}
}

type closerFlow struct {
	relays  map[string]bool
	aliases map[string]string
	roots   map[string]*ast.FuncDecl
}

func returnsACloser(fd *ast.FuncDecl) bool {
	if fd.Type.Results == nil {
		return false
	}
	for _, field := range fd.Type.Results.List {
		switch typ := field.Type.(type) {
		case *ast.Ident:
			if typ.Name == "orchDeps" {
				return true
			}
		case *ast.FuncType:
			if typ.Params.NumFields() == 0 && typ.Results.NumFields() == 0 {
				return true
			}
		}
	}
	return false
}

func seamAliases(files map[string]*ast.File) map[string]string {
	aliases := map[string]string{}
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				if target := calleeName(firstValue(vs)); len(vs.Names) == 1 && target != "" {
					aliases[vs.Names[0].Name] = target
				}
			}
		}
	}
	return aliases
}

func firstValue(vs *ast.ValueSpec) ast.Expr {
	if len(vs.Values) != 1 {
		return nil
	}
	return vs.Values[0]
}

func (f closerFlow) isRelayCall(call *ast.CallExpr) bool {
	name := calleeName(call.Fun)
	if target, ok := f.aliases[name]; ok {
		name = target
	}
	return f.relays[name]
}

func (f closerFlow) obtainsACloser(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		if call, ok := x.(*ast.CallExpr); ok && f.isRelayCall(call) {
			found = true
		}
		return !found
	})
	return found
}

func deriveCloserFlow(files map[string]*ast.File) closerFlow {
	flow := closerFlow{relays: map[string]bool{"rootwire.Attach": true}, aliases: seamAliases(files), roots: map[string]*ast.FuncDecl{}}
	for changed := true; changed; {
		changed = false
		for rel, file := range files {
			for _, fd := range functionsOf(file) {
				switch {
				case !flow.obtainsACloser(fd.Body):
				case !returnsACloser(fd):
					flow.roots[rel+":"+fd.Name.Name] = fd
				case !flow.relays[fd.Name.Name]:
					flow.relays[fd.Name.Name] = true
					changed = true
				}
			}
		}
	}
	return flow
}

func namesTheCloser(fun ast.Expr) bool {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name == "closeSignals"
	case *ast.SelectorExpr:
		return f.Sel.Name == "closeSignals"
	}
	return false
}

func containsReturn(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		switch x.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			found = true
		}
		return !found
	})
	return found
}

func closerDeferProblem(fd *ast.FuncDecl, flow closerFlow) string {
	stmts := fd.Body.List
	first := slices.IndexFunc(stmts, func(s ast.Stmt) bool { return flow.obtainsACloser(s) })
	if first < 0 {
		return "obtains no closer"
	}
	if containsReturn(stmts[first]) {
		return "can return in the statement that obtains the closer, before it defers the closer"
	}
	for _, s := range stmts[first+1:] {
		if d, ok := s.(*ast.DeferStmt); ok && namesTheCloser(d.Call.Fun) {
			return ""
		}
		if containsReturn(s) {
			return "can return between obtaining the closer and deferring it"
		}
	}
	return "never defers the closer at the top level of its body"
}

func TestRootSignalCenter_EveryRootDefersItsCloser(t *testing.T) {
	t.Parallel()
	flow := deriveCloserFlow(parsedModuleSources(t))

	for _, floor := range []string{
		"cmd/evolve/cmd_cycle.go:runCycleRun",
		"cmd/evolve/cmd_loop.go:runLoopBatch",
		"cmd/evolve/cmd_loop_chain.go:runLoopChain",
		"cmd/evolve/cmd_loop_chain.go:maybeRefreshChainBoundary",
		"cmd/evolve/cmd_ship.go:runShipCmd",
		"cmd/evolve/cmd_subagent.go:runSubagentRun",
		"internal/cli/phasecmd/phase_observer.go:RunPhaseObserver",
	} {
		if flow.roots[floor] == nil {
			t.Errorf("%s is not among the roots derived from the call sites %v: the derivation lost a root that the lane relies on", floor, slices.Sorted(maps.Keys(flow.roots)))
		}
	}
	for site, fd := range flow.roots {
		if problem := closerDeferProblem(fd, flow); problem != "" {
			t.Errorf("%s %s: its lossless events are on disk, but the best-effort queue is not drained at exit", site, problem)
		}
	}
}

func snippetFlow(t *testing.T, sources ...string) closerFlow {
	t.Helper()
	files := map[string]*ast.File{}
	for i, src := range sources {
		file, err := parser.ParseFile(token.NewFileSet(), "snippet.go", src, 0)
		if err != nil {
			t.Fatal(err)
		}
		files["snippet"+strconv.Itoa(i)+".go"] = file
	}
	return deriveCloserFlow(files)
}

func TestDeriveCloserFlow_AFunctionThatReturnsTheCloserRelaysItAndItsCallersAreTheRoots(t *testing.T) {
	t.Parallel()
	flow := snippetFlow(t, `package p
func run() int { d := wrap(); defer d.closeSignals(); return 0 }
func unrelated() int { return 1 }
func wrap() orchDeps { _ = attach(); return orchDeps{} }
func attach() func() { return rootwire.Attach(c, "r", "d", w) }
`)

	if got := slices.Sorted(maps.Keys(flow.roots)); !slices.Equal(got, []string{"snippet0.go:run"}) {
		t.Errorf("roots = %v, want only run: it is the one function that obtains a closer and passes none on", got)
	}
	for _, relay := range []string{"rootwire.Attach", "attach", "wrap"} {
		if !flow.relays[relay] {
			t.Errorf("%s is not a relay: a function that returns the closer hands the duty to its callers", relay)
		}
	}
	if flow.relays["run"] || flow.relays["unrelated"] {
		t.Errorf("relays = %v, want neither run nor unrelated", flow.relays)
	}
}

func TestDeriveCloserFlow_ASeamVariableIsFollowedToItsDefault(t *testing.T) {
	t.Parallel()
	flow := snippetFlow(t, `package p
var buildFn = build
var unrelatedFn, otherFn = 1, 2
var literalFn = func() int { return 1 }
func build() orchDeps { _ = rootwire.Attach(c, "r", "d", w); return orchDeps{} }
func run() int { d := buildFn(); defer d.closeSignals(); return 0 }
`)

	if got := slices.Sorted(maps.Keys(flow.roots)); !slices.Equal(got, []string{"snippet0.go:run"}) {
		t.Errorf("roots = %v, want run: a call through the seam variable buildFn is a call of build", got)
	}
}

func TestCloserDeferProblem_NamesWhatLetsARootSkipItsCloser(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, body, problem string
	}{
		{"defers at once", `c := rootwire.Attach(); defer closeSignals(); return 0`, ""},
		{"defers a method closer after other work", `d := rootwire.Attach(); work(); defer d.closeSignals(); if d.failed { return 1 }; return 0`, ""},
		{"returns before it obtains the closer", `if bad { return 1 }; c := rootwire.Attach(); defer closeSignals(); return 0`, ""},
		{"a closure may return", `c := rootwire.Attach(); f := func() int { return 1 }; _ = f; defer closeSignals(); return 0`, ""},
		{"returns before the defer", `c := rootwire.Attach(); if bad { return 1 }; defer closeSignals(); return 0`, "can return between"},
		{"returns in the obtaining statement", `if c := rootwire.Attach(); c == nil { return 1 }; defer closeSignals(); return 0`, "can return in the statement that obtains the closer"},
		{"defers only in a branch", `c := rootwire.Attach(); if ok { defer closeSignals() }`, "never defers the closer at the top level"},
		{"defers only in a closure", `c := rootwire.Attach(); go func() { defer closeSignals() }()`, "never defers the closer at the top level"},
		{"flushes but does not close", `c := rootwire.Attach(); defer signals.Flush()`, "never defers the closer at the top level"},
		{"defers nothing", `c := rootwire.Attach(); work()`, "never defers the closer at the top level"},
		{"returns without deferring", `c := rootwire.Attach(); return 0`, "can return between"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			flow := snippetFlow(t, "package p\nfunc run() int { "+tc.body+" }\n")
			fd := flow.roots["snippet0.go:run"]
			if fd == nil {
				t.Fatalf("run is not a derived root: %v", flow.roots)
			}
			got := closerDeferProblem(fd, flow)
			if tc.problem == "" && got != "" || tc.problem != "" && !strings.Contains(got, tc.problem) {
				t.Errorf("closerDeferProblem = %q, want %q", got, tc.problem)
			}
		})
	}
}

func TestRootSignalCenter_NoRootDropsItsCloserAndEveryDepsLiteralCarriesIt(t *testing.T) {
	t.Parallel()
	files := parsedModuleSources(t)
	flow := deriveCloserFlow(files)
	for rel, file := range files {
		for _, fd := range functionsOf(file) {
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.AssignStmt:
					if call, ok := x.Rhs[0].(*ast.CallExpr); ok && flow.isRelayCall(call) {
						if id, ok := x.Lhs[len(x.Lhs)-1].(*ast.Ident); ok && id.Name == "_" {
							t.Errorf("%s:%s drops the closer of %s", rel, fd.Name.Name, calleeName(call.Fun))
						}
					}
				case *ast.CompositeLit:
					if id, ok := x.Type.(*ast.Ident); ok && id.Name == "orchDeps" && keyedWith(x, "Signals") && !keyedWith(x, "CloseSignals") {
						t.Errorf("%s:%s builds an orchDeps with a Center and no CloseSignals", rel, fd.Name.Name)
					}
				}
				return true
			})
		}
	}
}

func keyedWith(lit *ast.CompositeLit, key string) bool {
	for _, el := range lit.Elts {
		if kv, ok := el.(*ast.KeyValueExpr); ok {
			if id, ok := kv.Key.(*ast.Ident); ok && id.Name == key {
				return true
			}
		}
	}
	return false
}
