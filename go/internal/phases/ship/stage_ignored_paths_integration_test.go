//go:build integration

// stage_ignored_paths_integration_test.go — real-git half of the cycle-1101
// gitignored-declared-path contract. The unit half (stage_ignored_paths_test.go)
// pins the git ARGUMENTS via a capture runner; this pins the OBSERVABLE EFFECT
// against a genuine repository — required by the adversarial review of the
// first fix attempt, whose `check-ignore -z` (stdin-mode-only flag) made the
// probe rc=128 on every ship: the capture-runner unit tests stayed green while
// the production feature was dead on arrival. Only a real git can catch that
// class.
package ship

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestShipFromWorktree_DropsGitignoredDeclaredEvalPath — a worktree whose
// phase reports declare BOTH a real changed file and the cycle's gitignored
// eval file (the eval-quality contract does this on every cycle) must ship
// cleanly: the eval path is dropped from staging (real `git add` refuses
// ignored paths with rc=1), the declared source file lands in the commit, and
// the eval file stays on disk untouched.
func TestShipFromWorktree_DropsGitignoredDeclaredEvalPath(t *testing.T) {
	repo, wt := makeWorktreeScenario(t)

	// Un-stage so the ship's own staging step is what decides the commit.
	runGit(t, wt, "reset", "HEAD", "wt-change.txt")

	// Gitignore the eval dir IN THE COMMITTED TREE (mirrors .evolve/* in the
	// real repo) and create the eval file the reports will declare.
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
	// Dropped from staging ≠ deleted: the runtime eval file must survive.
	if _, err := os.Stat(filepath.Join(wt, filepath.FromSlash(evalRel))); err != nil {
		t.Errorf("eval file must remain on disk after being excluded from staging: %v", err)
	}
	// And the drop is LOUD in the ship log.
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
