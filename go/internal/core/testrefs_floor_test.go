package core

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/testrefs"
)

func TestTestrefsFloorFailures_RefusesABaselineRowThatTheBaseDidNotHold(t *testing.T) {
	t.Parallel()
	repo, base := sizeFloorRepo(t, "go", 60)
	path := filepath.Join(repo.Dir, "go", filepath.FromSlash(testrefs.BaselineRelPath))
	writeFile(t, path, `{"docs/architecture/a.md":["TestGhost","TestOld"]}`+"\n")
	in := ReviewInput{Phase: string(PhaseBuild), Worktree: repo.Dir, WorktreeBaseSHA: base}

	failures := testrefsFloorFailures(context.Background(), in)

	if len(failures) != 1 || !strings.Contains(failures[0], "docs/architecture/a.md: baseline row TestGhost") || !strings.Contains(failures[0], "do not edit "+testrefs.BaselineRelPath) {
		t.Fatalf("the floor must refuse the grown row and name the rule: %v", failures)
	}
	writeFile(t, path, `{}`+"\n")
	if failures := testrefsFloorFailures(context.Background(), in); failures != nil {
		t.Errorf("a shrink is never a failure: %v", failures)
	}
}

func TestTestrefsFloorFailures_IsSilentWithoutAWorktreeAndFailsOpenOnAnUnreadableBase(t *testing.T) {
	t.Parallel()
	if failures := testrefsFloorFailures(context.Background(), ReviewInput{}); failures != nil {
		t.Errorf("no worktree: %v", failures)
	}
	repo, _ := sizeFloorRepo(t, ".", 0)
	var stderr bytes.Buffer
	in := ReviewInput{Worktree: repo.Dir, WorktreeBaseSHA: strings.Repeat("0", 40), Stderr: &stderr}
	if failures := testrefsFloorFailures(context.Background(), in); failures != nil || !strings.Contains(stderr.String(), "[build-floor] WARN named-test floor") {
		t.Errorf("an unreadable base fails open with a WARN: failures=%v stderr=%q", failures, stderr.String())
	}
}

func TestStaticBuildFloorChecks_RunsTheNamedTestFloor(t *testing.T) {
	t.Parallel()
	repo, base := sizeFloorRepo(t, "go", 60)
	writeFile(t, filepath.Join(repo.Dir, "go", filepath.FromSlash(testrefs.BaselineRelPath)), `{"docs/architecture/a.md":["TestGhost","TestOld"]}`+"\n")

	failures := StaticBuildFloorChecks(context.Background(), ReviewInput{Phase: string(PhaseBuild), Worktree: repo.Dir, WorktreeBaseSHA: base})

	if !strings.Contains(strings.Join(failures, "\n"), "baseline row TestGhost") {
		t.Fatalf("the production build floor does not run the named-test floor: %v", failures)
	}
}
