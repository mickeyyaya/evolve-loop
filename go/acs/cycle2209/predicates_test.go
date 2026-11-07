//go:build acs

package cycle2209

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	bridgeImportPath = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	minWallWaitSecs  = 30
)

var harnessFiles = []string{"channel_e2e_test.go", "channel_e2e_load_test.go"}

func bridgeDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go", "internal", "bridge")
}

func runBridgeRace(t *testing.T, runFilter string) (string, int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-run", runFilter, bridgeImportPath,
	)
	return acsassert.OutputTail(stdout+stderr, 40), code
}

func parseHarness(t *testing.T) map[string]*ast.File {
	t.Helper()
	dir := bridgeDir(t)
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, name := range harnessFiles {
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files[name] = f
	}
	return files
}

func eachFunc(files map[string]*ast.File, visit func(file string, fn *ast.FuncDecl)) {
	for name, f := range files {
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Body != nil {
				visit(name, fn)
			}
		}
	}
}

func isSel(e ast.Expr, pkg, name string) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg && sel.Sel.Name == name
}

func goStmtCallsRun(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		g, ok := n.(*ast.GoStmt)
		if !ok {
			return true
		}
		ast.Inspect(g.Call, func(m ast.Node) bool {
			c, ok := m.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := c.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Run" {
				found = true
			}
			return true
		})
		return true
	})
	return found
}

func cleanupWaitsOnChannel(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok || !isSel(c.Fun, "t", "Cleanup") || len(c.Args) != 1 {
			return true
		}
		lit, ok := c.Args[0].(*ast.FuncLit)
		if !ok {
			return true
		}
		ast.Inspect(lit.Body, func(m ast.Node) bool {
			if u, ok := m.(*ast.UnaryExpr); ok && u.Op == token.ARROW {
				found = true
			}
			return true
		})
		return true
	})
	return found
}

func secondsLiteral(e ast.Expr) (int, bool) {
	b, ok := e.(*ast.BinaryExpr)
	if !ok || b.Op != token.MUL || !isSel(b.Y, "time", "Second") {
		return 0, false
	}
	lit, ok := b.X.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return 0, false
	}
	n, err := strconv.Atoi(lit.Value)
	return n, err == nil
}

func TestC2209_001_ClaudeSpanSurvivesHostLagAtBothTransitions(t *testing.T) {
	tail, code := runBridgeRace(t, "^TestChannelE2E_RealFixtures_ClaudeSpan_UnderHostLag$")
	if code != 0 {
		t.Errorf("C2209_001: lagged claude-span e2e exit=%d (the harness fails when the host is slow)\n%s", code, tail)
	}
}

func TestC2209_002_ClaudeSpanStillPassesOnIdleHost(t *testing.T) {
	tail, code := runBridgeRace(t, "^TestChannelE2E_RealFixtures_ClaudeSpan$")
	if code != 0 {
		t.Errorf("C2209_002: original claude-span e2e exit=%d\n%s", code, tail)
	}
}

func TestC2209_003_ReproducerLagsAreNotShrunkToGoGreen(t *testing.T) {
	files := parseHarness(t)
	floors := map[string]int{"beforeIdle": 8, "afterIdle": 4}
	seen := map[string]int{}
	ast.Inspect(files["channel_e2e_load_test.go"], func(n ast.Node) bool {
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			return true
		}
		floor, tracked := floors[key.Name]
		if !tracked {
			return true
		}
		secs, ok := secondsLiteral(kv.Value)
		if !ok {
			return true
		}
		if secs < floor {
			t.Errorf("C2209_003: %s lag is %ds, floor is %ds: the reproducer models a loaded host", key.Name, secs, floor)
		}
		if secs > seen[key.Name] {
			seen[key.Name] = secs
		}
		return true
	})
	for key := range floors {
		if seen[key] == 0 {
			t.Errorf("C2209_003: no %s lag case left in channel_e2e_load_test.go", key)
		}
	}
}

func TestC2209_004_TempDirCleanupJoinsProducerAndDriver(t *testing.T) {
	files := parseHarness(t)
	harnesses := 0
	eachFunc(files, func(file string, fn *ast.FuncDecl) {
		if !goStmtCallsRun(fn) {
			return
		}
		harnesses++
		if !cleanupWaitsOnChannel(fn) {
			t.Errorf("C2209_004: %s %s starts a producer goroutine with no t.Cleanup that waits on a channel: the TempDir removal races the producer", file, fn.Name.Name)
		}
	})
	if harnesses == 0 {
		t.Errorf("C2209_004: no harness function starts the producer goroutine in %v", harnessFiles)
	}
}

func TestC2209_005_DriverRunsUnderCancellableContext(t *testing.T) {
	files := parseHarness(t)
	calls := 0
	eachFunc(files, func(file string, fn *ast.FuncDecl) {
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := c.Fun.(*ast.Ident)
			if !ok || id.Name != "runTmuxREPL" || len(c.Args) == 0 {
				return true
			}
			calls++
			if inner, ok := c.Args[0].(*ast.CallExpr); ok && isSel(inner.Fun, "context", "Background") {
				t.Errorf("C2209_005: %s %s runs the driver on context.Background(): a failed test cannot stop it", file, fn.Name.Name)
			}
			return true
		})
	})
	if calls == 0 {
		t.Errorf("C2209_005: no harness function drives runTmuxREPL in %v", harnessFiles)
	}
}

func TestC2209_006_DriverWaitIsNotAShortWallClockDeadline(t *testing.T) {
	files := parseHarness(t)
	eachFunc(files, func(file string, fn *ast.FuncDecl) {
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok || !isSel(c.Fun, "time", "After") || len(c.Args) != 1 {
				return true
			}
			if secs, ok := secondsLiteral(c.Args[0]); ok && secs < minWallWaitSecs {
				t.Errorf("C2209_006: %s %s waits on time.After(%d s): a loaded host needs the event, not a wall-clock guess", file, fn.Name.Name, secs)
			}
			return true
		})
	})
}

// acs-predicate: config-check
func TestC2209_007_PackageNoteNamesTheRaceAndItsReproducer(t *testing.T) {
	note := filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "packages", "internal-bridge.md")
	if _, err := os.Stat(note); err != nil {
		t.Fatalf("C2209_007: package note missing: %v", err)
	}
	if !acsassert.LineContainsAll(note, "ClaudeSpan_UnderHostLag", "producer", "driver", "supervisor") {
		t.Errorf("C2209_007: no line of %s names the driver, producer and supervisor race together with TestChannelE2E_RealFixtures_ClaudeSpan_UnderHostLag", note)
	}
}
