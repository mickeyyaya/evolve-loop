package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestCommentFloorResult_EnforceRejectsEveryAddedCommentByName(t *testing.T) {
	added := []commentaudit.Added{{File: "go/internal/x/a.go", Line: "// restates the helper."}, {File: "go/internal/x/b.go", Line: "// and another."}}

	failures, warning := commentFloorResult(config.StageEnforce, added)

	if warning != "" || len(failures) != 1 {
		t.Fatalf("enforce: failures=%v warning=%q, want one failure and no warning", failures, warning)
	}
	for _, want := range []string{"2 comment line(s)", "go/internal/x/a.go: // restates the helper.", "go/internal/x/b.go: // and another.", "docs/conventions/code-comments.md"} {
		if !strings.Contains(failures[0], want) {
			t.Errorf("the failure does not name %q:\n%s", want, failures[0])
		}
	}
}

func TestCommentFloorResult_ShadowOnlyWarnsAndOffOrACleanDiffIsSilent(t *testing.T) {
	added := []commentaudit.Added{{File: "a.go", Line: "// restates the helper."}}

	if failures, warning := commentFloorResult(config.StageShadow, added); failures != nil || !strings.Contains(warning, "a.go: // restates the helper.") || !strings.Contains(warning, "shadow") {
		t.Errorf("shadow: failures=%v warning=%q, want a warning naming the line and no failure", failures, warning)
	}
	for stage, in := range map[config.Stage][]commentaudit.Added{config.StageOff: added, config.StageEnforce: nil, config.StageShadow: nil} {
		if failures, warning := commentFloorResult(stage, in); failures != nil || warning != "" {
			t.Errorf("stage %s with %d added: failures=%v warning=%q, want silence", stage, len(in), failures, warning)
		}
	}
}

func TestCommentFloorFailures_JudgesTheWorktreeAgainstItsBaseAndSparesAMovedComment(t *testing.T) {
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), "package p\n\n// keeps the lease across resume.\nfunc a() {}\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "old.go"), "package p\n\n// renamed along with its file.\nfunc o() {}\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), "package p\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "lease.go"), "package p\n\n// keeps the lease across resume.\nfunc a() {}\n\n// restates the helper.\nfunc h() {}\n")
	repo.Git("mv", "go/p/old.go", "go/p/renamed.go")
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".evolve", "policy.json"), `{"comment_floor":{"stage":"enforce"}}`)
	in := ReviewInput{Phase: string(PhaseBuild), ProjectRoot: root, Worktree: repo.Dir, WorktreeBaseSHA: base}

	failures := commentFloorFailures(context.Background(), in)

	if len(failures) != 1 || !strings.Contains(failures[0], "go/p/lease.go: // restates the helper.") || strings.Contains(failures[0], "keeps the lease") || strings.Contains(failures[0], "renamed along") {
		t.Fatalf("the floor must name only the new comment, sparing the moved and the renamed ones: %v", failures)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, "go", "p", "lease.go")); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultBuildFloorChecks_RunsTheCommentFloor(t *testing.T) {
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "notes", "a.go"), "package notes\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(repo.Dir, "notes", "a.go"), "package notes\n\n// restates the helper.\nfunc h() {}\n")
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".evolve", "policy.json"), `{"comment_floor":{"stage":"enforce"}}`)

	failures := DefaultBuildFloorChecks(context.Background(), ReviewInput{Phase: string(PhaseBuild), ProjectRoot: root, Worktree: repo.Dir, WorktreeBaseSHA: base})

	if !strings.Contains(strings.Join(failures, "\n"), "notes/a.go: // restates the helper.") {
		t.Fatalf("the production build floor does not run the comment floor: %v", failures)
	}
}
