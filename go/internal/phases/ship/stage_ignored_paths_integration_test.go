//go:build integration

package ship

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShipFromWorktree_DropsGitignoredDeclaredEvalPath(t *testing.T) {
	t.Parallel()
	repo, wt := makeWorktreeScenario(t)

	runGit(t, wt, "reset", "HEAD", "wt-change.txt")

	mustWrite(t, filepath.Join(wt, ".gitignore"), ".evolve/*\n")
	runGit(t, wt, "add", ".gitignore")
	evalRel := ".evolve/evals/persona-budget-inlane-gate.md"
	mustWrite(t, filepath.Join(wt, filepath.FromSlash(evalRel)), "# eval\n")

	ws := t.TempDir()
	mustWrite(t, filepath.Join(ws, "build-report.md"),
		"# Build Report\n\n## Files Changed\n\n- `wt-change.txt`\n")
	mustWrite(t, filepath.Join(ws, "test-report.md"),
		"# TDD Report\n\n| File | Evals |\n|---|---|\n| "+evalRel+" | 4 score_caps |\n")

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: eval-declaring cycle ships despite gitignored eval path",
		ProjectRoot:   repo,
		PluginRoot:    repo,
		WorkspacePath: ws,
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
	res := &RunResult{}
	if err := shipFromWorktree(context.Background(), opts, res, "main", wt); err != nil {
		t.Fatalf("shipFromWorktree with gitignored declared eval path: %v (cycle-1101 rc=1 class)", err)
	}

	files := commitFileList(t, wt, "cycle-1")
	if !strings.Contains(files, "wt-change.txt") {
		t.Errorf("declared source path absent from the ship commit; files=%q", files)
	}
	if strings.Contains(files, evalRel) {
		t.Errorf("gitignored eval path rode into the ship commit; files=%q", files)
	}
	if _, err := os.Stat(filepath.Join(wt, filepath.FromSlash(evalRel))); err != nil {
		t.Errorf("eval file must remain on disk after being excluded from staging: %v", err)
	}
	var logged bool
	for _, l := range res.Logs {
		if strings.Contains(l, "gitignored") && strings.Contains(l, evalRel) {
			logged = true
		}
	}
	if !logged {
		t.Errorf("dropped ignored path not logged; logs=%v", res.Logs)
	}
}
