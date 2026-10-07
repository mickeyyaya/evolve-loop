//go:build acs

package cycle1823

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runNamedFleetTest(t *testing.T, root, name string) {
	t.Helper()
	cmd := exec.Command("go", "test", "-count=1", "-v", "-run", "^"+name+"$", "./internal/fleet")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "--- PASS: "+name) {
		t.Errorf("go test -run ^%s$ ./internal/fleet did not report a pass (err=%v):\n%s", name, err, out)
	}
}

func TestC1823_001_RunPoolNilLauncherFailsEveryLaneLikeSupervisor(t *testing.T) {
	backlog := []fleet.Todo{
		{ID: "A", Files: []string{"a.go"}},
		{ID: "B", Files: []string{"b.go"}},
		{ID: "C", Files: []string{"a.go"}},
	}
	supervisorShape := (&fleet.Supervisor{}).Run(context.Background(), make([]fleet.CycleSpec, len(backlog)))
	if len(supervisorShape) != len(backlog) || supervisorShape[0].Err == nil {
		t.Fatalf("Supervisor.Run no longer fails a nil LaunchFn (%+v); the reference RunPool must mirror is gone", supervisorShape)
	}

	res := fleet.RunPool(context.Background(), fleet.PoolConfig{Target: 2}, backlog, nil, nil)
	if len(res) != len(backlog) {
		t.Fatalf("RunPool(nil launcher) returned %d results, want %d", len(res), len(backlog))
	}
	for i, r := range res {
		if r.Status() != fleet.LaneFailed {
			t.Errorf("RunPool(nil launcher) result[%d]=%+v reads as %q, want %q", i, r, r.Status(), fleet.LaneFailed)
		}
		if r.Index != i || r.ExitCode != supervisorShape[i].ExitCode || !errors.Is(r.Err, supervisorShape[i].Err) {
			t.Errorf("RunPool(nil launcher) result[%d]=%+v, want the Supervisor.Run shape {Index:%d ExitCode:%d Err:%v}", i, r, i, supervisorShape[i].ExitCode, supervisorShape[i].Err)
		}
	}

	if empty := fleet.RunPool(context.Background(), fleet.PoolConfig{Target: 2}, nil, nil, nil); len(empty) != 0 {
		t.Errorf("RunPool(empty backlog, nil launcher) = %+v, want no results", empty)
	}

	succeed := func(context.Context, fleet.CycleSpec) (int, error) { return 0, nil }
	for i, r := range fleet.RunPool(context.Background(), fleet.PoolConfig{Target: 2}, backlog, succeed, nil) {
		if r.Status() != fleet.LaneOK || r.Index != i {
			t.Errorf("RunPool(succeeding launcher) result[%d]=%+v, want an ok lane: the nil-launcher failure must not leak into real launches", i, r)
		}
	}
}

func TestC1823_002_GlobalZoneFilesMatchDocumentedGlobalZone(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc, err := os.ReadFile(filepath.Join(root, "docs", "architecture", "packages", "internal-fleet.md"))
	if err != nil {
		t.Fatal(err)
	}
	clause := regexp.MustCompile(`global-zone file \(([^)]*)\)`).FindSubmatch(doc)
	if clause == nil {
		t.Fatal("internal-fleet.md no longer documents the global zone as \"global-zone file (...)\"")
	}
	var documented []string
	for _, m := range regexp.MustCompile("`([^`]+)`").FindAllSubmatch(clause[1], -1) {
		documented = append(documented, string(m[1]))
	}
	if len(documented) == 0 {
		t.Fatalf("the documented global-zone clause %q names no file", clause[1])
	}
	got := fleet.GlobalZoneFiles()
	sort.Strings(documented)
	sort.Strings(got)
	if !reflect.DeepEqual(got, documented) {
		t.Errorf("fleet.GlobalZoneFiles() = %v, documented global zone = %v", got, documented)
	}
	for _, f := range documented {
		if !fleet.IsGlobalZone("./" + f) {
			t.Errorf("IsGlobalZone(%q) = false for a documented global-zone file", "./"+f)
		}
	}
	for _, ordinary := range []string{"go/internal/fleet/pool.go", "internal/fleet/packagegraph.go", "go.mod.bak", ""} {
		if fleet.IsGlobalZone(ordinary) {
			t.Errorf("IsGlobalZone(%q) = true, want false for a file outside the documented global zone", ordinary)
		}
	}
	runNamedFleetTest(t, root, "TestGlobalZoneFiles_MatchesDocumentedGlobalZone")
}

func fleetTestFuncs(t *testing.T, fset *token.FileSet, dir string) map[string]*ast.FuncDecl {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no fleet test files under %s (err=%v)", dir, err)
	}
	funcs := map[string]*ast.FuncDecl{}
	for _, p := range paths {
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Body != nil {
				funcs[fd.Name.Name] = fd
			}
		}
	}
	return funcs
}

func isPkgCall(n ast.Node, pkg, name string) bool {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg
}

func isBusyWait(body *ast.BlockStmt) bool {
	for _, s := range body.List {
		es, ok := s.(*ast.ExprStmt)
		if !ok || !(isPkgCall(es.X, "runtime", "Gosched") || isPkgCall(es.X, "time", "Sleep")) {
			return false
		}
	}
	return true
}

func receiveOperand(comm ast.Stmt) ast.Expr {
	var e ast.Expr
	switch c := comm.(type) {
	case *ast.ExprStmt:
		e = c.X
	case *ast.AssignStmt:
		if len(c.Rhs) == 1 {
			e = c.Rhs[0]
		}
	}
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.ARROW {
		return u.X
	}
	return nil
}

func isTimeoutSource(x ast.Expr, deadlineCtx bool) bool {
	if isPkgCall(x, "time", "After") {
		return true
	}
	if sel, ok := x.(*ast.SelectorExpr); ok && sel.Sel.Name == "C" {
		return true
	}
	if call, ok := x.(*ast.CallExpr); ok && deadlineCtx {
		sel, ok := call.Fun.(*ast.SelectorExpr)
		return ok && sel.Sel.Name == "Done"
	}
	return false
}

func TestC1823_003_BoundedConcurrencyTestWaitsOnATimeoutNotASpin(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	const name = "TestSupervisor_BoundedConcurrency"
	fset := token.NewFileSet()
	funcs := fleetTestFuncs(t, fset, filepath.Join(root, "go", "internal", "fleet"))
	target, ok := funcs[name]
	if !ok {
		t.Fatalf("%s no longer exists in the fleet tests", name)
	}
	bodies := []*ast.BlockStmt{target.Body}
	ast.Inspect(target.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && funcs[id.Name] != nil && id.Name != name {
				bodies = append(bodies, funcs[id.Name].Body)
			}
		}
		return true
	})
	deadlineCtx := false
	for _, b := range bodies {
		ast.Inspect(b, func(n ast.Node) bool {
			if isPkgCall(n, "context", "WithTimeout") || isPkgCall(n, "context", "WithDeadline") {
				deadlineCtx = true
			}
			return true
		})
	}
	var spins []string
	timeoutWait := false
	for _, b := range bodies {
		ast.Inspect(b, func(n ast.Node) bool {
			switch s := n.(type) {
			case *ast.ForStmt:
				if isBusyWait(s.Body) {
					spins = append(spins, fset.Position(s.Pos()).String())
				}
			case *ast.SelectStmt:
				cases := 0
				hasTimeout := false
				for _, c := range s.Body.List {
					cc, ok := c.(*ast.CommClause)
					if !ok || cc.Comm == nil {
						continue
					}
					cases++
					if op := receiveOperand(cc.Comm); op != nil && isTimeoutSource(op, deadlineCtx) {
						hasTimeout = true
					}
				}
				if hasTimeout && cases >= 2 {
					timeoutWait = true
				}
			}
			return true
		})
	}
	if len(spins) > 0 {
		t.Errorf("%s still busy-waits at %v; wait on a channel instead", name, spins)
	}
	if !timeoutWait {
		t.Errorf("%s has no select that waits for the lanes alongside a timeout case (time.After, a timer's .C, or a deadline context's Done)", name)
	}
	runNamedFleetTest(t, root, name)
}

func TestC1823_004_QuotaBenchWarnKeepsTheLoopwaveContractGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	var warn bytes.Buffer
	if got := fleet.QuotaAwareCount(3, map[string]string{"codex": "rate_limit"}, 1, &warn); got != 2 {
		t.Fatalf("QuotaAwareCount(3, one bench, min 1) = %d, want 2", got)
	}
	if !strings.Contains(warn.String(), `quota bench on CLI family "codex" (rate_limit): wave count 3 -> 2 (min 1)`) {
		t.Errorf("QuotaAwareCount no longer warns about the shrink; got %q", warn.String())
	}
	cmd := exec.Command("go", "test", "-count=1", "./internal/loopwave")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil || !regexp.MustCompile(`(?m)^ok\s+\S*/internal/loopwave\s`).Match(out) {
		t.Errorf("go test ./internal/loopwave is red (err=%v): loopwave pins fleet's quota-bench and freshness-gate WARN text under the protected control plane, so a fleet lane must keep that text unchanged:\n%s", err, out)
	}
}

func launchNilComparisons(t *testing.T, fleetDir string) map[string][]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(fleetDir, "*.go"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no fleet sources under %s (err=%v)", fleetDir, err)
	}
	fset := token.NewFileSet()
	byFunc := map[string][]string{}
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				be, ok := n.(*ast.BinaryExpr)
				if !ok || (be.Op != token.EQL && be.Op != token.NEQ) {
					return true
				}
				for _, pair := range [][2]ast.Expr{{be.X, be.Y}, {be.Y, be.X}} {
					sel, isSel := pair[0].(*ast.SelectorExpr)
					nilID, isIdent := pair[1].(*ast.Ident)
					if isSel && sel.Sel.Name == "Launch" && isIdent && nilID.Name == "nil" {
						byFunc[fd.Name.Name] = append(byFunc[fd.Name.Name], fset.Position(be.Pos()).String())
					}
				}
				return true
			})
		}
	}
	return byFunc
}

func TestC1823_005_SupervisorChecksANilLauncherOnlyInValidate(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	nilLaunch := &fleet.Supervisor{Concurrency: 2}
	wantErr := nilLaunch.Validate()
	if wantErr == nil {
		t.Fatal("Supervisor.Validate accepts a nil LaunchFn; Run's only nil guard is gone")
	}
	res := nilLaunch.Run(context.Background(), make([]fleet.CycleSpec, 3))
	if len(res) != 3 {
		t.Fatalf("Supervisor.Run(nil launcher, 3 specs) returned %d results, want 3", len(res))
	}
	for i, r := range res {
		if r.Index != i || r.ExitCode != -1 || !errors.Is(r.Err, wantErr) || r.Status() != fleet.LaneFailed {
			t.Errorf("Supervisor.Run(nil launcher) result[%d]=%+v, want {Index:%d ExitCode:-1 Err:%v} reading as %q", i, r, i, wantErr, fleet.LaneFailed)
		}
	}
	launched := 0
	realLaunch := &fleet.Supervisor{Launch: func(_ context.Context, spec fleet.CycleSpec) (int, error) {
		launched++
		if spec.Env[ipcenv.FleetKey] != "1" {
			return 1, errors.New("lane launched without fleet mode")
		}
		return 0, nil
	}}
	if err := realLaunch.Validate(); err != nil {
		t.Fatalf("Supervisor.Validate rejects a real LaunchFn: %v", err)
	}
	for i, r := range realLaunch.Run(context.Background(), make([]fleet.CycleSpec, 1)) {
		if r.Status() != fleet.LaneOK {
			t.Errorf("Supervisor.Run(real launcher) result[%d]=%+v, want an ok lane", i, r)
		}
	}
	if launched != 1 {
		t.Errorf("Supervisor.Run(real launcher, 1 spec) launched %d lanes, want 1", launched)
	}

	guards := launchNilComparisons(t, filepath.Join(root, "go", "internal", "fleet"))
	if len(guards["Validate"]) == 0 {
		t.Errorf("no Launch nil check left in Supervisor.Validate; found %v", guards)
	}
	for fn, at := range guards {
		if fn != "Validate" {
			t.Errorf("%s still compares Launch with nil at %v; Run rejects a nil LaunchFn through Validate before any launch, so that branch is dead", fn, at)
		}
	}
	runNamedFleetTest(t, root, "TestSupervisor_NilLaunch_ErrorsPerSpec")
}

func TestC1823_006_FleetPackageDocNamesOnlyTestsThatExist(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	doc, err := os.ReadFile(filepath.Join(root, "docs", "architecture", "packages", "internal-fleet.md"))
	if err != nil {
		t.Fatal(err)
	}
	named := map[string]bool{}
	for _, m := range regexp.MustCompile(`\bTest[A-Z][A-Za-z0-9_]*`).FindAll(doc, -1) {
		named[string(m)] = true
	}
	if len(named) == 0 {
		t.Fatal("internal-fleet.md names no test; the doc's test pins are gone")
	}
	declared := map[string]bool{}
	testFunc := regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]*)\(`)
	walkErr := filepath.WalkDir(filepath.Join(root, "go"), func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, m := range testFunc.FindAllSubmatch(src, -1) {
			declared[string(m[1])] = true
		}
		return nil
	})
	if walkErr != nil {
		t.Fatal(walkErr)
	}
	var stale []string
	for name := range named {
		if !declared[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("internal-fleet.md cites tests no package declares: %v; a doc claim pinned by a missing test is unpinned", stale)
	}
}

func warnPrefix(t *testing.T, line, marker string) string {
	t.Helper()
	at := strings.Index(line, marker)
	if at < 0 {
		t.Fatalf("WARN line %q no longer contains %q", line, marker)
	}
	return strings.TrimSpace(line[:at])
}

func TestC1823_007_FleetDocPrefixClaimMatchesPrintedWarnings(t *testing.T) {
	root := acsassert.RepoRoot(t)
	var quotaWarn bytes.Buffer
	fleet.QuotaAwareCount(3, map[string]string{"codex": "rate_limit"}, 1, &quotaWarn)
	quotaPrefix := warnPrefix(t, quotaWarn.String(), "quota bench on CLI family")

	var freshnessWarn bytes.Buffer
	staleProbe := func(string) fleet.TaskFreshness { return fleet.TaskFreshness{Reason: "consumed"} }
	emptyBacklog := func(map[string]bool) (fleet.CycleSpec, bool) { return fleet.CycleSpec{}, false }
	fleet.FreshenSpecs([]fleet.CycleSpec{{Scope: []string{"stale-id"}}}, staleProbe, emptyBacklog, &freshnessWarn)
	freshnessPrefix := warnPrefix(t, freshnessWarn.String(), "freshness gate skipped stale-id")

	doc, err := os.ReadFile(filepath.Join(root, "docs", "architecture", "packages", "internal-fleet.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, claim := range regexp.MustCompile("shares? (?:the|one) `([^`]+)` prefix").FindAllSubmatch(doc, -1) {
		claimed := strings.TrimSpace(string(claim[1]))
		if quotaPrefix != claimed || freshnessPrefix != claimed {
			t.Errorf("internal-fleet.md claims the fleet WARNs share the %q prefix, but QuotaAwareCount prints %q and FreshenSpecs prints %q", claimed, quotaPrefix, freshnessPrefix)
		}
	}
}
