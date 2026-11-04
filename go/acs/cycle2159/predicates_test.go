//go:build acs

package cycle2159

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const ciparityPkg = "./internal/ciparity"

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

type located struct {
	decl        *ast.FuncDecl
	integration bool
	source      string
}

func locateTest(t *testing.T, name string) (located, bool) {
	t.Helper()
	dir := filepath.Join(goDir(t), "internal", "ciparity")
	paths, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("glob ciparity tests: %v (%d files)", err, len(paths))
	}
	fset := token.NewFileSet()
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		f, err := parser.ParseFile(fset, p, raw, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if ok && fn.Recv == nil && fn.Name.Name == name {
				return located{decl: fn, integration: strings.Contains(string(raw), "//go:build integration"), source: string(raw[fset.Position(fn.Pos()).Offset:fset.Position(fn.End()).Offset])}, true
			}
		}
	}
	return located{}, false
}

func requirePasses(t *testing.T, name string, wantIntegration bool) located {
	t.Helper()
	loc, found := locateTest(t, name)
	if !found {
		t.Fatalf("RED: %s is not declared in %s", name, ciparityPkg)
	}
	if loc.integration != wantIntegration {
		t.Fatalf("%s integration-tagged = %v, want %v: the normal CI suite must run it", name, loc.integration, wantIntegration)
	}
	args := []string{"-C", goDir(t), "test", "-count=1", "-v", "-run", "^" + name + "$"}
	if wantIntegration {
		args = append(args, "-tags", "integration")
	}
	args = append(args, ciparityPkg)
	out, errOut, code, err := acsassert.SubprocessOutput("go", args...)
	if err != nil || code != 0 {
		t.Fatalf("%s exit=%d err=%v\n%s\n%s", name, code, err, acsassert.OutputTail(out, 30), acsassert.OutputTail(errOut, 30))
	}
	if !strings.Contains(out, "--- PASS: "+name+" ") {
		t.Fatalf("%s did not run to a PASS:\n%s", name, acsassert.OutputTail(out, 30))
	}
	return loc
}

func requireSourceMentions(t *testing.T, loc located, name string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(loc.source, n) {
			t.Errorf("%s never mentions %q: it does not evaluate that case", name, n)
		}
	}
}

func TestC2159_001_PullRequestGroupsByNumberAndCancelsInProgress(t *testing.T) {
	const name = "TestCIConcurrency_PullRequestGroupsByNumberAndCancels"
	loc := requirePasses(t, name, false)
	requireSourceMentions(t, loc, name, "pull_request", "pr-", "RequiredWorkflow")
}

func TestC2159_002_PushGroupsByRefAndNeverCancels(t *testing.T) {
	const name = "TestCIConcurrency_PushGroupsByRefAndNeverCancels"
	loc := requirePasses(t, name, false)
	requireSourceMentions(t, loc, name, "push", "refs/heads/", "RequiredWorkflow")
}

func TestC2159_003_DispatchGroupsByRunIDAndNeverCancels(t *testing.T) {
	const name = "TestCIConcurrency_DispatchGroupsByRunID"
	loc := requirePasses(t, name, false)
	requireSourceMentions(t, loc, name, "workflow_dispatch", "dispatch-", "RequiredWorkflow")
}

func TestC2159_004_RequiredAndGoWorkflowsAreBothEvaluatedForEveryEvent(t *testing.T) {
	const name = "TestCIConcurrency_RequiredAndGoWorkflowsAreEvaluatedForEveryEvent"
	loc := requirePasses(t, name, false)
	requireSourceMentions(t, loc, name, "go.yml", "RequiredWorkflow", "pull_request", "push", "workflow_dispatch")
}

func TestC2159_005_DistinctPullRequestsRefsAndRunsNeverShareAGroup(t *testing.T) {
	const name = "TestCIConcurrency_DistinctPullRequestsRefsAndRunsNeverShareAGroup"
	loc := requirePasses(t, name, false)
	requireSourceMentions(t, loc, name, "pull_request", "push", "workflow_dispatch")
}

func TestC2159_006_InboxOnlyDiffTakesTheLightRoute(t *testing.T) {
	const name = "TestRequiredRouting_InboxOnlyChangesSkipTheGoMatrix"
	loc := requirePasses(t, name, true)
	requireSourceMentions(t, loc, name, "inbox_only", "inbox_deleted_only", "parked_inbox_is_not_the_inbox")
}
