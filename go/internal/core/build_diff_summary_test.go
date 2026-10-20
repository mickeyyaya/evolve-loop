package core

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const noOpBuildReport = "# Build Report\n\n## Worktree\n- **Files changed:** 2\n\n## Changes\n```tsv\nAction\tFile\tDescription\nMODIFY\tprior.go\tthe prior cycle\n```\n\n| files_changed | pending <!-- POSTHOC: git show HEAD --numstat | wc -l --> |\n"

func noOpCycle(t *testing.T) (worktree, base, workspace string) {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("seed.txt", "seed\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "seed")
	repo.Write("prior.go", "package prior\n")
	repo.Write("prior_two.go", "package prior\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "the ship commit of the prior cycle")
	workspace = t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "build-report.md"), []byte(noOpBuildReport), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo.Dir, repo.Git("rev-parse", "HEAD"), workspace
}

func TestWriteBuildDiffSummary_ANoOpCycleReportsZeroFilesFromItsBase(t *testing.T) {
	t.Parallel()
	worktree, base, workspace := noOpCycle(t)
	var warn bytes.Buffer

	writeBuildDiffSummary(context.Background(), worktree, base, workspace, &warn)

	body, _ := os.ReadFile(filepath.Join(workspace, "build-report.md"))
	if !strings.Contains(string(body), "| files_changed | 0 <!-- host-derived") || !strings.Contains(string(body), "- **Files changed:** 0 (host-derived") {
		t.Fatalf("a no-op cycle must report 0 files from its base, not the tip commit's 2:\n%s", body)
	}
	if !strings.Contains(string(body), "Declared by the build but not in the diff: `prior.go`") || warn.Len() != 0 {
		t.Errorf("the declared row of another commit must be disclosed; warn=%q\n%s", warn.String(), body)
	}
}

func TestWriteBuildDiffSummary_ANoBaseWarnsAndStatesAbsent(t *testing.T) {
	t.Parallel()
	worktree, _, workspace := noOpCycle(t)
	var warn bytes.Buffer

	writeBuildDiffSummary(context.Background(), worktree, "", workspace, &warn)

	body, _ := os.ReadFile(filepath.Join(workspace, "build-report.md"))
	if strings.Contains(string(body), "| files_changed | pending") || !strings.Contains(string(body), "| files_changed | absent") {
		t.Errorf("a missing base must state absent, never pending:\n%s", body)
	}
	if !strings.Contains(warn.String(), "[orchestrator] WARN build-diff-summary") {
		t.Errorf("warn = %q, want a loud line", warn.String())
	}
}

func TestWriteBuildDiffSummary_NoReportIsANoOpAndAFailedWriteWarns(t *testing.T) {
	t.Parallel()
	worktree, base, workspace := noOpCycle(t)
	var warn bytes.Buffer
	writeBuildDiffSummary(context.Background(), worktree, base, t.TempDir(), &warn)
	if warn.Len() != 0 {
		t.Errorf("no report must stay quiet: %q", warn.String())
	}
	report := filepath.Join(workspace, "build-report.md")
	if err := os.Chmod(report, 0o444); err != nil {
		t.Fatal(err)
	}
	writeBuildDiffSummary(context.Background(), worktree, base, workspace, &warn)
	if !strings.Contains(warn.String(), "build-diff-summary: write") {
		t.Errorf("a failed write must warn: %q", warn.String())
	}
}

func TestNormalizeBuildWorktree_WritesTheHostDiffIntoTheBuildReport(t *testing.T) {
	t.Parallel()
	worktree, base, workspace := noOpCycle(t)
	o := &Orchestrator{stderr: &bytes.Buffer{}}

	o.normalizeBuildWorktree(context.Background(), PhaseBuild, CycleState{ActiveWorktree: worktree, WorktreeBaseSHA: base, WorkspacePath: workspace}, t.TempDir())

	body, _ := os.ReadFile(filepath.Join(workspace, "build-report.md"))
	if !strings.Contains(string(body), "| files_changed | 0 <!-- host-derived") {
		t.Fatalf("the build normalize must write the host diff into the report:\n%s", body)
	}
}
