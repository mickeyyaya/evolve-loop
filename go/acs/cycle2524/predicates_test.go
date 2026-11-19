//go:build acs

// Package cycle2524 holds the ACS predicates of the shadow-batch overlap test fix.
package cycle2524

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	testFileRel      = "go/internal/core/evaluate_batch_shadow_test.go"
	dispatchFileRel  = "go/internal/core/evaluate_batch.go"
	serialTestName   = "TestShadowDispatchesTheBatchSerially"
	minRendezvousBnd = 5 * time.Second
)

func goTestRun(t *testing.T, env []string, overlay, count, race, verbose, run string) (string, int) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-overlay", overlay, "-count", count, "-race="+race, "-v="+verbose, "-run", run, "./internal/core/")
	cmd.Dir = filepath.Join(root, "go")
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		code = 1
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
	}
	return string(out), code
}

func emptyOverlay(t *testing.T) string {
	t.Helper()
	overlay := filepath.Join(t.TempDir(), "overlay.json")
	if err := os.WriteFile(overlay, []byte(`{"Replace":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return overlay
}

func mutantOverlay(t *testing.T, old, replacement string) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	src := filepath.Join(root, dispatchFileRel)
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", dispatchFileRel, err)
	}
	if strings.Count(string(raw), old) != 1 {
		t.Fatalf("mutation anchor %q must occur exactly once in %s", old, dispatchFileRel)
	}
	dir := t.TempDir()
	mutated := filepath.Join(dir, "evaluate_batch_mutant.go")
	if err := os.WriteFile(mutated, []byte(strings.Replace(string(raw), old, replacement, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(dir, "overlay.json")
	body := `{"Replace":{` + strconv.Quote(src) + `:` + strconv.Quote(mutated) + `}}`
	if err := os.WriteFile(overlay, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return overlay
}

func TestC2524_001_EnforceControlSurvivesASlowScheduler(t *testing.T) {
	overlay := mutantOverlay(t,
		"defer wg.Done()\n\t\t\tsem <- struct{}{}",
		"defer wg.Done()\n\t\t\tif i > 0 {\n\t\t\t\ttime.Sleep(2 * time.Second)\n\t\t\t}\n\t\t\tsem <- struct{}{}")

	out, code := goTestRun(t, nil, overlay, "1", "false", "false", "^"+serialTestName+"$")

	if code != 0 {
		t.Errorf("a peer that enters Run 2s late made the enforce control fail: the test proves overlap with timing, not a rendezvous\n%s", acsassert.OutputTail(out, 15))
	}
}

func TestC2524_002_NoSleepAndRendezvousBoundAtLeastFiveSeconds(t *testing.T) {
	root := acsassert.RepoRoot(t)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join(root, testFileRel), nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", testFileRel, err)
	}

	var sleeps int
	var longest time.Duration
	ast.Inspect(file, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.CallExpr:
			if sel, ok := v.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Sleep" {
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "time" {
					sleeps++
				}
			}
		case *ast.BinaryExpr, *ast.SelectorExpr:
			if d, ok := durationOf(n.(ast.Expr)); ok && d > longest {
				longest = d
			}
		}
		return true
	})

	if sleeps != 0 {
		t.Errorf("%s calls time.Sleep %d time(s): overlap must be proven by a rendezvous", testFileRel, sleeps)
	}
	if longest < minRendezvousBnd {
		t.Errorf("longest duration literal in %s is %v, want a rendezvous bound of at least %v", testFileRel, longest, minRendezvousBnd)
	}
}

func durationOf(e ast.Expr) (time.Duration, bool) {
	switch v := e.(type) {
	case *ast.SelectorExpr:
		return timeUnit(v)
	case *ast.BinaryExpr:
		if v.Op != token.MUL {
			return 0, false
		}
		if n, ok := intLit(v.X); ok {
			if u, ok := durationOf(v.Y); ok {
				return time.Duration(n) * u, true
			}
		}
		if n, ok := intLit(v.Y); ok {
			if u, ok := durationOf(v.X); ok {
				return time.Duration(n) * u, true
			}
		}
	}
	return 0, false
}

func timeUnit(sel *ast.SelectorExpr) (time.Duration, bool) {
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "time" {
		return 0, false
	}
	units := map[string]time.Duration{"Nanosecond": time.Nanosecond, "Microsecond": time.Microsecond, "Millisecond": time.Millisecond, "Second": time.Second, "Minute": time.Minute}
	d, ok := units[sel.Sel.Name]
	return d, ok
}

func intLit(e ast.Expr) (int64, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return 0, false
	}
	n, err := strconv.ParseInt(lit.Value, 0, 64)
	return n, err == nil
}

func TestC2524_003_ConcurrentShadowDispatchStillTurnsTheTestRed(t *testing.T) {
	overlay := mutantOverlay(t,
		"cr.emitParallelEvaluateShadow(batch)\n\t\treturn nil",
		"cr.emitParallelEvaluateShadow(batch)\n\t\treturn batch")

	out, code := goTestRun(t, nil, overlay, "1", "false", "true", "^"+serialTestName+"$")

	if code == 0 {
		t.Fatalf("a shadow that dispatches the batch concurrently left %s green", serialTestName)
	}
	if !strings.Contains(out, "--- FAIL: "+serialTestName) {
		t.Errorf("the run failed without %s failing (compile error or other cause):\n%s", serialTestName, acsassert.OutputTail(out, 15))
	}
}

func TestC2524_004_StaysGreenRepeatedlyUnderOneAndTwoProcs(t *testing.T) {
	for _, procs := range []string{"1", "2"} {
		out, code := goTestRun(t, []string{"GOMAXPROCS=" + procs}, emptyOverlay(t), "50", "true", "false", "TestShadowDispatchesTheBatchSerially|TestShadowIsNotOff")
		if code != 0 {
			t.Errorf("GOMAXPROCS=%s: -count=50 -race failed:\n%s", procs, acsassert.OutputTail(out, 20))
		}
	}
}

func TestC2524_005_ChangeTouchesNoGoSourceBesideTheShadowTestFile(t *testing.T) {
	root := acsassert.RepoRoot(t)
	base, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", "origin/main")
	if err != nil || code != 0 {
		t.Fatalf("git merge-base HEAD origin/main: code=%d err=%v", code, err)
	}
	tracked, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", strings.TrimSpace(base))
	if err != nil || code != 0 {
		t.Fatalf("git diff against %s: code=%d err=%v", strings.TrimSpace(base), code, err)
	}
	untracked, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--others", "--exclude-standard")
	if err != nil || code != 0 {
		t.Fatalf("git ls-files: code=%d err=%v", code, err)
	}
	pipelineOwned := []string{"go/acs/cycle2524/"}
	guarded := []string{"go/", "agents/", "skills/", "guidance/", "scripts/"}
	for _, path := range strings.Fields(tracked + "\n" + untracked) {
		if path == testFileRel || hasAnyPrefix(path, pipelineOwned) || !hasAnyPrefix(path, guarded) {
			continue
		}
		t.Errorf("changed path %s is outside %s", path, testFileRel)
	}
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
