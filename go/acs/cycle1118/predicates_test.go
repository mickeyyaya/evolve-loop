//go:build acs

package cycle1118

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const bridgePkg = "./internal/bridge/"

func goDir(t *testing.T) string { return filepath.Join(acsassert.RepoRoot(t), "go") }

func runGoTests(t *testing.T, pkg string, names ...string) {
	t.Helper()
	filter := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir(t), "-count=1", "-v", "-run", filter, pkg)
	out := stdout + stderr
	if err != nil {
		t.Fatalf("go test failed to launch (not a test failure): %v\n%s", err, out)
	}
	if code != 0 {
		t.Fatalf("%s -run %s exited %d\n%s", pkg, filter, code, out)
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Fatalf("no PASS line for %s in %s (renamed, skipped, or never ran?)\n%s", name, pkg, out)
		}
	}
}

func TestC1118_001_TransientFatalPaneNeverFastFails(t *testing.T) {
	runGoTests(t, bridgePkg,
		"TestFatalPaneGate_TransientMatchDoesNotFastFail",
		"TestFatalPaneGate_BusyObservationResetsStreak",
		"TestFatalPaneGate_DisabledPathsNeverAccumulate",
	)
}

func TestC1118_002_PersistentFatalPaneStillFastFailsBounded(t *testing.T) {
	runGoTests(t, bridgePkg,
		"TestFatalPaneGate_PersistentFatalPaneStillFastFails",
		"TestFatalPaneGate_ThresholdMirrorsExhaustionGuard",
	)
}

func TestC1118_003_ShadowEvidenceTracksGatedEnforce(t *testing.T) {
	runGoTests(t, bridgePkg, "TestFatalPaneGate_ShadowEvidenceOnlyAfterPersistence")
}

func TestC1118_004_CheckpointLoopOwnsExactlyOneGate(t *testing.T) {
	path := filepath.Join(goDir(t), "internal", "bridge", "driver_tmux_repl.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	var owner *ast.FuncDecl
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if countCalls(fn, isIdentCall("newExhaustionGate")) > 0 {
			if owner != nil {
				t.Fatalf("two functions construct newExhaustionGate() — cannot identify the checkpoint loop owner")
			}
			owner = fn
		}
	}
	if owner == nil {
		t.Fatalf("no function in %s constructs newExhaustionGate() — the checkpoint-loop precedent moved; this predicate needs re-anchoring", filepath.Base(path))
	}

	if n := countCalls(owner, isIdentCall("newFatalPaneGate")); n != 1 {
		t.Errorf("%s constructs newFatalPaneGate() %d time(s), want exactly 1 — the gate must be loop-scoped state (a per-checkpoint gate never accumulates a streak, so a transient match still kills)", owner.Name.Name, n)
	}
	if n := countCalls(owner, isIdentCall("fatalPaneVerdict")); n != 0 {
		t.Errorf("%s still calls fatalPaneVerdict directly %d time(s) — the checkpoint must reach the seam through the gate, or production stays un-gated regardless of the gate's unit tests", owner.Name.Name, n)
	}
	if n := countCalls(owner, isMethodCall("verdict")); n != 1 {
		t.Errorf("%s makes %d gate .verdict(...) call(s), want exactly 1 — fatal-pane evidence is recorded per matching call, so a second call would inflate the soak's C2 counts silently", owner.Name.Name, n)
	}
}

func TestC1118_005_ExistingFatalPaneContractUnchanged(t *testing.T) {
	runGoTests(t, bridgePkg,
		"TestFatalPaneVerdict_EnforcePreemptsWithStop",
		"TestFatalPaneVerdict_ShadowLogsButDoesNotPreempt",
		"TestFatalPaneVerdict_BusyPaneNeverPreempted",
		"TestFatalPaneVerdict_OffSkipsDetection",
		"TestFatalPaneVerdict_ShadowRecordsDurableEvidence",
		"TestFatalPaneVerdict_EnforceRecordsFastFailed",
		"TestFatalPaneVerdict_BusyAndOffRecordNothing",
	)
}

func isIdentCall(name string) func(*ast.CallExpr) bool {
	return func(call *ast.CallExpr) bool {
		id, ok := call.Fun.(*ast.Ident)
		return ok && id.Name == name
	}
}

func isMethodCall(name string) func(*ast.CallExpr) bool {
	return func(call *ast.CallExpr) bool {
		sel, ok := call.Fun.(*ast.SelectorExpr)
		return ok && sel.Sel != nil && sel.Sel.Name == name
	}
}

func countCalls(fn *ast.FuncDecl, match func(*ast.CallExpr) bool) int {
	n := 0
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok && match(call) {
			n++
		}
		return true
	})
	return n
}
