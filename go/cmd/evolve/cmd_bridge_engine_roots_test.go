// cmd_bridge_engine_roots_test.go — ADR-0103 unit 10 (design §6 tests 49-50):
// every non-test bridge.NewEngine call in the module is enumerated with its
// Signal Center wiring, so the roots where the unit's BRIDGE_EXIT_* and step
// codes fall into the nil Null Object are on record rather than implicit; and
// the --simulate root renders a launch death on the console and in the
// cycle-workspace signals.ndjson.
package main

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
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

// bridgeEngineRoots is the allowlist: module-relative file → the number of
// bridge.NewEngine CALL EXPRESSIONS it holds and how each is wired.
// "center-bearing" roots thread a Signal Center into Deps (nil only when the
// caller passed nil — the eight phase-registry NewDefault(root, nil) defaults
// and cmd_campaign); "center-less" roots build a bare Deps{} and drop every
// bridge signal (operator question 1 — the S4 sink topology, follow-up F2).
var bridgeEngineRoots = map[string]struct {
	calls  int
	wiring string
}{
	"cmd/evolve/cmd_bridge.go":             {1, "center-less: `evolve bridge launch`, bare Deps{}"},
	"cmd/evolve/cmd_cycle.go":              {1, "center-less: the pre-cycle Doctor probe, bare Deps{}"},
	"cmd/evolve/cmd_models_live.go":        {1, "center-less: the models-live probe, bare Deps{}"},
	"internal/adapters/bridge/bridge.go":   {3, "center-bearing: New()'s default factory (Env only), NewDefault's factory and Launch's onStopReview branch through productionEngineDeps (Deps.Signals = the Adapter's Center; nil when NewDefault was given nil)"},
	"internal/setup/setup.go":              {1, "center-less: the setup Doctor default, bare Deps{}"},
	"internal/subagent/validateprofile.go": {1, "center-less: the subagent runner (execAdapterDeps)"},
}

// bridgeNewEngineCalls counts the bridge.NewEngine call expressions in one
// file, resolving the package through its import alias (never a comment, a
// doc string or a same-named local).
func bridgeNewEngineCalls(t *testing.T, path string) int {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
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
		return 0
	}
	calls := 0
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "NewEngine" {
			if x, ok := sel.X.(*ast.Ident); ok && x.Name == alias {
				calls++
			}
		}
		return true
	})
	return calls
}

// Test 49 — every non-test NewEngine root is in the allowlist with the right
// call count; a new root fails until it is classified center-bearing or
// center-less.
func TestBridgeEngineRootsAreEnumerated(t *testing.T) {
	moduleRoot := filepath.Join("..", "..")
	found := map[string]int{}
	err := filepath.WalkDir(moduleRoot, func(path string, entry os.DirEntry, err error) error {
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
		if n := bridgeNewEngineCalls(t, path); n > 0 {
			rel, _ := filepath.Rel(moduleRoot, path)
			found[filepath.ToSlash(rel)] = n
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
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

// Test 50 — the --simulate root renders a launch death: the console sink
// prints the module tag and the BRIDGE_EXIT_* code, and the cycle-stamped
// event is durable in the cycle workspace's signals.ndjson.
func TestWireSimulateOrchestrator_BridgeExitWarningRenders(t *testing.T) {
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
