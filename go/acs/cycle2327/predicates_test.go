//go:build acs

package cycle2327

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const gcPackage = "./internal/gc"

const pipelineOnlyTests = "TestRebuildableIgnored_PipelineOutputAtAnyDepth|TestPlanDevWorktrees_ATreeHoldingOnlyPipelineOutputIsArchived|TestPlanDevWorktrees_APipelineOnlyCommitGateTreeIsArchived"

const keepTests = "TestPlanDevWorktrees_AnEnvFileBesidePipelineOutputKeepsTheTree|TestPlanDevWorktrees_AnIgnoredFileInsidePipelineOutputDirectoryStaysRebuildable|TestPlanDevWorktrees_ANestedRepositoryBesidePipelineOutputKeepsTheTree"

func runNamedTests(t *testing.T, names string, extra ...string) (string, int) {
	t.Helper()
	args := append([]string{"-C", filepath.Join(acsassert.RepoRoot(t), "go"), "test", "-count=1", "-v", "-run", "^(" + names + ")$"}, extra...)
	args = append(args, gcPackage)
	out, errOut, code, _ := acsassert.SubprocessOutput("go", args...)
	return out + errOut, code
}

func requirePassed(t *testing.T, out string, code int, names string) {
	t.Helper()
	if code != 0 {
		t.Fatalf("go test %s exit=%d:\n%s", gcPackage, code, acsassert.OutputTail(out, 25))
	}
	for _, name := range strings.Split(names, "|") {
		if !regexp.MustCompile(`--- PASS: ` + regexp.QuoteMeta(name) + ` `).MatchString(out) {
			t.Errorf("%s did not run and pass:\n%s", name, acsassert.OutputTail(out, 25))
		}
	}
}

func TestC2327_001_ATreeHoldingOnlyPipelineOutputPassesTheGate(t *testing.T) {
	out, code := runNamedTests(t, pipelineOnlyTests)
	requirePassed(t, out, code, pipelineOnlyTests)
}

func TestC2327_002_AnEnvFileOrANestedRepositoryStillKeepsTheTree(t *testing.T) {
	out, code := runNamedTests(t, keepTests)
	requirePassed(t, out, code, keepTests)
}

func TestC2327_003_ThePreExistingRebuildableAndNestedGuardsStayGreen(t *testing.T) {
	preserved := "TestRebuildableIgnored|TestPlanDevWorktrees_ANestedGitRepositoryBlocksTheArchive|TestPlanDevWorktrees_AnIgnoredFileBlocksTheArchiveAndIsNamed|TestPlanDevWorktrees_OnlyRebuildableIgnoredOutputsAllowTheArchive"
	out, code := runNamedTests(t, preserved)
	requirePassed(t, out, code, preserved)
}

func TestC2327_004_EveryChangedFunctionKeepsFullCoverage(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "cover.out")
	out, code := runNamedTests(t, pipelineOnlyTests+"|"+keepTests+"|TestRebuildableIgnored", "-coverprofile="+profile)
	if code != 0 {
		t.Fatalf("coverage run exit=%d:\n%s", code, acsassert.OutputTail(out, 25))
	}
	funcOut, errOut, fcode, _ := acsassert.SubprocessOutput("go", "-C", filepath.Join(acsassert.RepoRoot(t), "go"), "tool", "cover", "-func="+profile)
	if fcode != 0 {
		t.Fatalf("go tool cover exit=%d:\n%s", fcode, errOut)
	}
	for _, fn := range []string{"rebuildableIgnored"} {
		pct, err := acsassert.FuncCoverage(funcOut, fn)
		if err != nil {
			t.Errorf("%v", err)
			continue
		}
		if pct < 100.0 {
			t.Errorf("%s coverage %.1f%%, want 100%%", fn, pct)
		}
	}
}

// acs-predicate: config-check
func TestC2327_005_TheBuildExplanationNamesDecisionsD3AndX3(t *testing.T) {
	root := acsassert.RepoRoot(t)
	matches, err := filepath.Glob(filepath.Join(root, "docs", "explain", "builds", "cycle-2327-*.md"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("want exactly one docs/explain/builds/cycle-2327-*.md, got %v (err %v)", matches, err)
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"D3", "X3"} {
		if !regexp.MustCompile(`\b` + id + `\b`).Match(raw) {
			t.Errorf("%s does not name decision id %s", filepath.Base(matches[0]), id)
		}
	}
	rel, _ := filepath.Rel(root, matches[0])
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("%s untracked — may be gitignored (dropped at ship)", rel)
	}
}
