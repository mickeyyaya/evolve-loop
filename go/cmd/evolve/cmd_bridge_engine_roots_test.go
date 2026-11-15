package main

import (
	"bytes"
	"context"
	"go/ast"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/launchoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const bridgeImportPath = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"

var bridgeEngineRoots = map[string]struct {
	calls  int
	wiring string
}{
	"cmd/evolve/cmd_bridge.go":           {2, "center-less: `evolve bridge launch` through bridgeLaunchDeps (TokenResolver, the policy sandbox mode and credential opt-ins, and the bypass recorder), and the other bridge verbs through Deps{TokenResolver}"},
	"cmd/evolve/cli_routing_root.go":     {1, "center-less: the routing table's discovered-tail Doctor probe, Deps{TokenResolver}"},
	"cmd/evolve/cmd_models_live.go":      {1, "center-less: the models-live probe, Deps{TokenResolver}"},
	"internal/adapters/bridge/bridge.go": {3, "center-bearing: New()'s default factory (Env and TokenResolver), NewDefault's factory and Launch's onStopReview branch through productionEngineDeps (Deps.Signals = the Adapter's Center; nil when NewDefault was given nil)"},
	"internal/setup/setup.go":            {1, "center-less: the setup Doctor default, Deps{TokenResolver}"},
	"internal/subagent/bridgeadapter.go": {1, "center-bearing: the `evolve subagent run` root (ADR-0103 unit 16) — execAdapterDepsWith(env, signals) hands the Dispatcher's Center to the engine"},
}

var depsBuildersThatWireTokens = map[string]bool{"bridgeLaunchDeps": true, "productionEngineDeps": true, "execAdapterDepsWith": true, "execAdapterDeps": true}

var depsForwarders = map[string]string{
	"internal/subagent/bridgeadapter.go": "newAdapterBridge forwards the Deps that execAdapterDepsWith builds",
}

func bridgeNewEngineCalls(t *testing.T, f *ast.File) int {
	t.Helper()
	calls := 0
	forEachBridgeNewEngine(f, func(*ast.CallExpr, ast.Decl) { calls++ })
	return calls
}

func forEachBridgeNewEngine(f *ast.File, visit func(*ast.CallExpr, ast.Decl)) {
	alias := ""
	for _, imp := range f.Imports {
		if strings.Trim(imp.Path.Value, `"`) != bridgeImportPath {
			continue
		}
		alias = "bridge"
		if imp.Name != nil {
			alias = imp.Name.Name
		}
	}
	if alias == "" {
		return
	}
	for _, decl := range f.Decls {
		ast.Inspect(decl, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "NewEngine" {
				if x, ok := sel.X.(*ast.Ident); ok && x.Name == alias {
					visit(call, decl)
				}
			}
			return true
		})
	}
}

func TestBridgeEngineRoots_EachWiresATokenResolver(t *testing.T) {
	t.Parallel()
	byRel := map[string]*moduleSourceFile{}
	for _, f := range sharedModuleFiles(t) {
		byRel[f.Rel] = f
	}
	for rel := range bridgeEngineRoots {
		_, forwards := depsForwarders[rel]
		f, ok := byRel[rel]
		if !ok {
			t.Fatalf("missing module file %s", rel)
		}
		forEachBridgeNewEngine(f.AST(t), func(call *ast.CallExpr, decl ast.Decl) {
			if !forwards && (len(call.Args) != 1 || !depsWireTokens(call.Args[0], decl)) {
				t.Errorf("%s: a NewEngine call builds Deps without a TokenResolver, so its launches record no token usage and the build WARNs BRIDGE_TOKEN_RESOLVER_MISSING", rel)
			}
		})
	}
}

func depsWireTokens(arg ast.Expr, scope ast.Node) bool {
	switch a := arg.(type) {
	case *ast.CompositeLit:
		return setsNonNil(a, "TokenResolver")
	case *ast.CallExpr:
		name := calleeName(a.Fun)
		return depsBuildersThatWireTokens[name[strings.LastIndex(name, ".")+1:]]
	case *ast.Ident:
		return identBuiltByAWiringBuilder(a.Name, scope)
	}
	return false
}

func identBuiltByAWiringBuilder(name string, scope ast.Node) bool {
	wired := false
	ast.Inspect(scope, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
			return true
		}
		if id, ok := as.Lhs[0].(*ast.Ident); ok && id.Name == name && depsWireTokens(as.Rhs[0], scope) {
			wired = true
		}
		return !wired
	})
	return wired
}

func setsNonNil(lit *ast.CompositeLit, key string) bool {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if id, isKey := kv.Key.(*ast.Ident); isKey && id.Name == key {
			value, isIdent := kv.Value.(*ast.Ident)
			return !isIdent || value.Name != "nil"
		}
	}
	return false
}

func TestBridgeEngineRootsAreEnumerated(t *testing.T) {
	t.Parallel()
	found := map[string]int{}
	for _, f := range sharedModuleFiles(t) {
		if n := bridgeNewEngineCalls(t, f.AST(t)); n > 0 {
			found[f.Rel] = n
		}
	}
	for rel, n := range found {
		want, ok := bridgeEngineRoots[rel]
		if !ok {
			t.Errorf("%s builds a bridge Engine (%d call(s)) but is not in the root allowlist — classify it center-bearing or center-less", rel, n)
		} else if want.calls != n {
			t.Errorf("%s: %d NewEngine calls, the allowlist says %d", rel, n, want.calls)
		}
	}
	for rel, want := range bridgeEngineRoots {
		if _, ok := found[rel]; !ok {
			t.Errorf("%s is in the allowlist (%s) but builds no Engine any more — drop the row", rel, want.wiring)
		}
	}
}

func TestWireSimulateOrchestrator_BridgeExitWarningRenders(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var console bytes.Buffer
	d := wireSimulateOrchestrator(root, evolveDir, &console)
	eng := bridge.NewEngine(bridge.Deps{Signals: d.Signals, Stderr: io.Discard, LookupEnv: func(string) (string, bool) { return "", false }})
	ws := filepath.Join(root, "ws")
	_, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: filepath.Join(root, "no-such-profile.json"), Workspace: ws,
		ArtifactPath: filepath.Join(ws, "a.md"), Prompt: "x", Agent: "build", Cycle: 3,
	})
	if err == nil || !strings.HasPrefix(err.Error(), "bridge: launch exit=10:") {
		t.Fatalf("a missing profile is exit 10: %v", err)
	}
	if out := console.String(); !strings.Contains(out, "[bridge]") || !strings.Contains(out, string(launchoutcome.CodeExitBadFlags)) || !strings.Contains(out, "origin=Engine.Launch") {
		t.Fatalf("the console sink renders the unit's WARN under --simulate: %q", out)
	}
	data, rerr := os.ReadFile(filepath.Join(core.RunWorkspacePath(root, 3), "signals.ndjson"))
	if rerr != nil || !strings.Contains(string(data), `"code":"BRIDGE_EXIT_BAD_FLAGS"`) || !strings.Contains(string(data), `"step":"classify"`) {
		t.Errorf("the cycle-stamped signal is durable in the cycle workspace: %v %s", rerr, data)
	}
}
