package core

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
)

func goFunc(name string, lines int) string {
	return "package p\n\nfunc " + name + "() {\n" + strings.Repeat("\t_ = 0\n", lines-2) + "}\n"
}

func sizeFloorRepo(t *testing.T, module string, listed int) (*gittest.Repo, string) {
	t.Helper()
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, module, "p", "a.go"), goFunc("F", 55))
	if listed > 0 {
		writeFile(t, filepath.Join(repo.Dir, module, filepath.FromSlash(sizeratchet.OffendersRelPath)), `{"p.F": 60}`+"\n")
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	return repo, repo.Git("rev-parse", "HEAD")
}

func TestSizeFloorFailures_RefusesAListedFunctionThatRegrowsIntoItsSlack(t *testing.T) {
	t.Parallel()
	repo, base := sizeFloorRepo(t, "go", 60)
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), goFunc("F", 58))
	in := ReviewInput{Phase: string(PhaseBuild), Worktree: repo.Dir, WorktreeBaseSHA: base}

	failures := sizeFloorFailures(context.Background(), in)

	if len(failures) != 1 || !strings.Contains(failures[0], "p.F grew to 58 lines > allowance 55") || !strings.Contains(failures[0], "do not edit "+sizeratchet.OffendersRelPath) || strings.Contains(failures[0], "ratchet tighten") {
		t.Fatalf("the floor must refuse the regrowth against the base size and tell the lane to shrink, never to edit the list or to run the writer: %v", failures)
	}
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), goFunc("F", 52))
	if failures := sizeFloorFailures(context.Background(), in); failures != nil {
		t.Errorf("a shrink is slack, never a failure: %v", failures)
	}
}

func TestSizeFloorFailures_JudgesAnUncleanWorktreePathAgainstItsModule(t *testing.T) {
	t.Parallel()
	repo, base := sizeFloorRepo(t, "go", 60)
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), goFunc("F", 58))
	in := ReviewInput{Phase: string(PhaseBuild), Worktree: repo.Dir + string(filepath.Separator) + ".", WorktreeBaseSHA: base}

	failures := sizeFloorFailures(context.Background(), in)

	if len(failures) != 1 || !strings.Contains(failures[0], "p.F grew to 58 lines > allowance 55") {
		t.Fatalf("an unclean worktree path must still map the module's paths, never pass silently: %v", failures)
	}
	if prefix, err := moduleRelPrefix("relative", string(filepath.Separator)+"absolute"); err == nil || prefix != "" {
		t.Errorf("a module dir that cannot be made relative to the worktree = %q, %v, want an error and no prefix", prefix, err)
	}
}

func TestSizeFloorFailures_IsSilentWithoutAWorktreeOrAListAndFailsOpenOnAnUnreadableBase(t *testing.T) {
	t.Parallel()
	if failures := sizeFloorFailures(context.Background(), ReviewInput{}); failures != nil {
		t.Errorf("no worktree: %v", failures)
	}
	repo, base := sizeFloorRepo(t, ".", 0)
	writeFile(t, filepath.Join(repo.Dir, "p", "a.go"), goFunc("F", 70))
	if failures := sizeFloorFailures(context.Background(), ReviewInput{Worktree: repo.Dir, WorktreeBaseSHA: base}); failures != nil {
		t.Errorf("a module without a list has no ratchet: %v", failures)
	}
	var stderr bytes.Buffer
	in := ReviewInput{Worktree: repo.Dir, WorktreeBaseSHA: strings.Repeat("0", 40), Stderr: &stderr}
	if failures := sizeFloorFailures(context.Background(), in); failures != nil || !strings.Contains(stderr.String(), "[build-floor] WARN size floor") {
		t.Errorf("an unreadable base fails open with a WARN: failures=%v stderr=%q", failures, stderr.String())
	}
}

func TestDefaultBuildFloorChecks_RunsTheSizeFloor(t *testing.T) {
	t.Parallel()
	repo, base := sizeFloorRepo(t, "go", 60)
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), goFunc("F", 58))
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".evolve", "policy.json"), `{"comment_floor":{"stage":"off"}}`)

	failures := DefaultBuildFloorChecks(context.Background(), ReviewInput{Phase: string(PhaseBuild), ProjectRoot: root, Worktree: repo.Dir, WorktreeBaseSHA: base})

	if !strings.Contains(strings.Join(failures, "\n"), "p.F grew to 58 lines > allowance 55") {
		t.Fatalf("the production build floor does not run the size floor: %v", failures)
	}
}
