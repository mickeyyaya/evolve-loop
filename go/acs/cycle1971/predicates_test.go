//go:build acs

package cycle1971

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/phasecmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const (
	cycleNumber      = 1971
	predicateRelPath = "go/acs/cycle1971/predicates_test.go"
	enforcePolicy    = `{"comment_floor":{"stage":"enforce"}}`
	narrativeComment = "// pins the retry budget."
	predicateBody    = "//go:build acs\n\npackage cycle1971\n\n// acs-predicate: config-check\nfunc TestC1971_001_RetryBudget() {}\n"
)

func writeTree(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func baseRepo(t *testing.T) (*gittest.Repo, string) {
	t.Helper()
	repo := gittest.Fixture(t)
	writeTree(t, repo.Dir, "go/p/a.go", "package p\n\nfunc a() {}\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	return repo, repo.Git("rev-parse", "HEAD")
}

func projectWithPolicy(t *testing.T, policyJSON string) string {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, ".evolve/policy.json", policyJSON)
	return root
}

func tddWorkspace(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	writeTree(t, ws, "test-report.md", "# TDD Report\n\n## AC-Materialization\n\n| Criterion | Test | Status |\n|---|---|---|\n| retry budget | TestC1971_001_RetryBudget | RED |\n\n## RED Run Output\n\n```\nFAIL\n```\n\n## Handoff to Builder\n\n```json\n{\n  \"testFiles\": [\""+predicateRelPath+"\"],\n  \"redRunConfirmed\": true,\n  \"doNotModifyTests\": false\n}\n```\n")
	return ws
}

func verifyTdd(t *testing.T, workspace, worktree string) (int, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := phasecmd.NewRunPhase(nil, nil)([]string{"verify", "tdd", "--workspace=" + workspace, "--worktree=" + worktree}, strings.NewReader(""), &out, &errb)
	return code, errb.String()
}

func TestC1971_001_TddExitCorrectsANarrativeCommentInItsOwnPredicateFile(t *testing.T) {
	repo, _ := baseRepo(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", projectWithPolicy(t, enforcePolicy))
	ws := tddWorkspace(t)
	writeTree(t, repo.Dir, predicateRelPath, strings.Replace(predicateBody, "func ", narrativeComment+"\nfunc ", 1))

	code, stderr := verifyTdd(t, ws, repo.Dir)
	if code != 1 || !strings.Contains(stderr, predicateRelPath+": "+narrativeComment) {
		t.Fatalf("RED: evolve phase verify tdd exit=%d, want 1 naming the narrative comment line; stderr=%s", code, stderr)
	}

	writeTree(t, repo.Dir, predicateRelPath, predicateBody)
	if code, stderr := verifyTdd(t, ws, repo.Dir); code != 0 {
		t.Fatalf("RED: the corrected predicate file must pass the TDD exit, exit=%d stderr=%s", code, stderr)
	}
}

func TestC1971_002_TddExitDoesNotJudgeFilesTheBuilderOwns(t *testing.T) {
	repo, _ := baseRepo(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", projectWithPolicy(t, enforcePolicy))
	ws := tddWorkspace(t)
	writeTree(t, repo.Dir, predicateRelPath, predicateBody)
	writeTree(t, repo.Dir, "go/p/b.go", "package p\n\n// restates the helper.\nfunc b() {}\n")

	if code, stderr := verifyTdd(t, ws, repo.Dir); code != 0 {
		t.Fatalf("exit=%d, want 0: the TDD floor judges only the TDD-owned predicate files; stderr=%s", code, stderr)
	}
}

func TestC1971_003_BuildFloorIgnoresTheCyclesTddOwnedPredicateComments(t *testing.T) {
	repo, base := baseRepo(t)
	writeTree(t, repo.Dir, predicateRelPath, strings.Replace(predicateBody, "func ", narrativeComment+"\nfunc ", 1))
	in := core.ReviewInput{Cycle: cycleNumber, Phase: string(core.PhaseBuild), ProjectRoot: projectWithPolicy(t, enforcePolicy), Worktree: repo.Dir, WorktreeBaseSHA: base}

	for _, failure := range core.DefaultBuildFloorChecks(context.Background(), in) {
		if strings.Contains(failure, "comment line(s)") {
			t.Fatalf("RED: the build floor counts the TDD-owned predicate comment: %s", failure)
		}
	}
}

func TestC1971_004_BuildFloorStillRejectsTheBuildsOwnComment(t *testing.T) {
	repo, base := baseRepo(t)
	writeTree(t, repo.Dir, predicateRelPath, strings.Replace(predicateBody, "func ", narrativeComment+"\nfunc ", 1))
	writeTree(t, repo.Dir, "go/p/b.go", "package p\n\n// restates the helper.\nfunc b() {}\n")
	in := core.ReviewInput{Cycle: cycleNumber, Phase: string(core.PhaseBuild), ProjectRoot: projectWithPolicy(t, enforcePolicy), Worktree: repo.Dir, WorktreeBaseSHA: base}

	joined := strings.Join(core.DefaultBuildFloorChecks(context.Background(), in), "\n")

	if !strings.Contains(joined, "go/p/b.go: // restates the helper.") {
		t.Fatalf("the build's own comment must still be named by the floor: %s", joined)
	}
	if strings.Contains(joined, narrativeComment) {
		t.Fatalf("RED: the floor must not name the TDD-owned comment beside the build's own: %s", joined)
	}
}

func TestC1971_005_UnboundCycleBuildFloorIgnoresANumberedPredicateDirButCountsTheBuildsOwnComment(t *testing.T) {
	repo, base := baseRepo(t)
	writeTree(t, repo.Dir, "go/acs/cycle42/predicates_test.go", strings.Replace(strings.ReplaceAll(predicateBody, "cycle1971", "cycle42"), "func ", narrativeComment+"\nfunc ", 1))
	writeTree(t, repo.Dir, "go/p/b.go", "package p\n\n// restates the helper.\nfunc b() {}\n")
	in := core.ReviewInput{Cycle: 0, Phase: string(core.PhaseBuild), ProjectRoot: projectWithPolicy(t, enforcePolicy), Worktree: repo.Dir, WorktreeBaseSHA: base}

	joined := strings.Join(core.DefaultBuildFloorChecks(context.Background(), in), "\n")

	if !strings.Contains(joined, "go/p/b.go: // restates the helper.") {
		t.Fatalf("the build's own comment must be named for an unbound cycle: %s", joined)
	}
	if strings.Contains(joined, narrativeComment) {
		t.Fatalf("RED: an unbound cycle counts the numbered TDD predicate dir comment: %s", joined)
	}
}
