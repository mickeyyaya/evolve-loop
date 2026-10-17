//go:build acs

package cycle1934

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	coveredBaseSource = "package p\n\nfunc F(x int) int {\n\treturn x\n}\n"
	branchedSource    = "package p\n\nfunc F(x int) int {\n\tif x > 0 {\n\t\treturn 1\n\t}\n\treturn x\n}\n"
	zeroOnlyTest      = "package p\n\nimport \"testing\"\n\nfunc TestF(t *testing.T) {\n\tif F(0) != 0 {\n\t\tt.Fatal(\"F(0)\")\n\t}\n}\n"
	bothBranchesTest  = "package p\n\nimport \"testing\"\n\nfunc TestF(t *testing.T) {\n\tif F(0) != 0 || F(1) != 1 {\n\t\tt.Fatal(\"F\")\n\t}\n}\n"
	gSource           = "package p\n\nfunc G() int {\n\treturn 2\n}\n"
	gTest             = "package p\n\nimport \"testing\"\n\nfunc TestG(t *testing.T) {\n\tif G() != 2 {\n\t\tt.Fatal(\"G\")\n\t}\n}\n"
)

type cycleTree struct {
	repo *gittest.Repo
	base string
}

func newCycleTree(t *testing.T, baseFiles map[string]string) cycleTree {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("go/go.mod", "module example.com/m\n\ngo 1.23\n")
	for rel, body := range baseFiles {
		repo.Write(rel, body)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	return cycleTree{repo: repo, base: repo.Git("rev-parse", "HEAD")}
}

func (c cycleTree) build(t *testing.T, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		c.repo.Write(rel, body)
	}
	c.repo.Git("add", "-A")
	c.repo.Git("commit", "-q", "-m", "build")
}

func projectRootWithPolicy(t *testing.T, policyJSON string) string {
	t.Helper()
	root := t.TempDir()
	if policyJSON == "" {
		return root
	}
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(policyJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func buildFloor(t *testing.T, c cycleTree, projectRoot string) (failures []string, stderr string) {
	t.Helper()
	var errOut bytes.Buffer
	failures = core.DefaultBuildFloorChecks(context.Background(), core.ReviewInput{
		Cycle:           1934,
		Phase:           "build",
		Worktree:        c.repo.Dir,
		WorktreeBaseSHA: c.base,
		ProjectRoot:     projectRoot,
		Stderr:          &errOut,
	})
	return failures, errOut.String()
}

func coverChangedRefusals(failures []string) []string {
	var out []string
	for _, f := range failures {
		if strings.Contains(f, "below 100% statement coverage") {
			out = append(out, f)
		}
	}
	return out
}

func TestC1934_001_CoverChangedCompiledDefaultIsEnforce(t *testing.T) {
	if got := (policy.Policy{}).CoverChangedConfig().Stage; got != "enforce" {
		t.Errorf("policy.Policy{}.CoverChangedConfig().Stage = %q, want %q: a cycle with no cover_changed policy block must refuse a changed function below 100%%, as CI does", got, "enforce")
	}
	if got := (policy.Policy{CoverChanged: &policy.CoverChangedPolicy{Stage: "shadow"}}).CoverChangedConfig().Stage; got != "shadow" {
		t.Errorf("an explicit cover_changed.stage=shadow resolved to %q, want the operator's shadow kept", got)
	}
}

func TestC1934_002_BuildFloorRefusesUncoveredChangedFunctionByName(t *testing.T) {
	tree := newCycleTree(t, map[string]string{"go/p/a.go": coveredBaseSource, "go/p/a_test.go": zeroOnlyTest})
	tree.build(t, map[string]string{"go/p/a.go": branchedSource})

	failures, stderr := buildFloor(t, tree, projectRootWithPolicy(t, ""))

	refusals := coverChangedRefusals(failures)
	if len(refusals) != 1 {
		t.Fatalf("the build floor returned %d cover-changed refusal(s), want 1 for the uncovered branch of p.F\nfailures=%q\nstderr=%s", len(refusals), failures, stderr)
	}
	for _, want := range []string{"REFUSED", "p/a.go:3", "p.F"} {
		if !strings.Contains(refusals[0], want) {
			t.Errorf("the refusal does not name %q:\n%s", want, refusals[0])
		}
	}
}

func TestC1934_003_BuildFloorPassesFullyCoveredChange(t *testing.T) {
	tree := newCycleTree(t, map[string]string{"go/p/a.go": coveredBaseSource, "go/p/a_test.go": zeroOnlyTest})
	tree.build(t, map[string]string{"go/p/a.go": branchedSource, "go/p/a_test.go": bothBranchesTest})

	failures, stderr := buildFloor(t, tree, projectRootWithPolicy(t, ""))

	if refusals := coverChangedRefusals(failures); len(refusals) != 0 {
		t.Errorf("a fully covered change was refused: %q\nstderr=%s", refusals, stderr)
	}
}

func TestC1934_004_BuildFloorPassesEmptyDiff(t *testing.T) {
	tree := newCycleTree(t, map[string]string{"go/p/a.go": branchedSource, "go/p/a_test.go": zeroOnlyTest})

	failures, stderr := buildFloor(t, tree, projectRootWithPolicy(t, ""))

	if refusals := coverChangedRefusals(failures); len(refusals) != 0 {
		t.Errorf("an empty cycle diff was refused: %q\nstderr=%s", refusals, stderr)
	}
}

func TestC1934_005_BuildFloorIgnoresUncoveredFunctionOnlyBaseChanged(t *testing.T) {
	tree := newCycleTree(t, map[string]string{"go/p/a.go": branchedSource, "go/p/a_test.go": zeroOnlyTest})
	tree.build(t, map[string]string{"go/p/g.go": gSource, "go/p/g_test.go": gTest})

	failures, stderr := buildFloor(t, tree, projectRootWithPolicy(t, ""))

	if refusals := coverChangedRefusals(failures); len(refusals) != 0 {
		t.Errorf("the floor refused a function the cycle never changed (p.F is uncovered only on the base): %q\nstderr=%s", refusals, stderr)
	}
}

func TestC1934_006_ExplicitShadowPolicyDoesNotRefuse(t *testing.T) {
	tree := newCycleTree(t, map[string]string{"go/p/a.go": coveredBaseSource, "go/p/a_test.go": zeroOnlyTest})
	tree.build(t, map[string]string{"go/p/a.go": branchedSource})

	failures, stderr := buildFloor(t, tree, projectRootWithPolicy(t, `{"cover_changed":{"stage":"shadow"}}`))

	if refusals := coverChangedRefusals(failures); len(refusals) != 0 {
		t.Errorf("an explicit cover_changed.stage=shadow policy refused the change: %q\nstderr=%s", refusals, stderr)
	}
}
