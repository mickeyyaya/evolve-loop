package core

import (
	"context"
	"errors"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

func TestCoverfixS2_AuthoritativeResumeIdentityRejectsMalformedFleetWorkspaceNames(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, tc := range []struct{ name, dir string }{
		{"no cycle prefix", "scratch"},
		{"bare number without prefix", "7"},
		{"non numeric cycle", "cycle-abc"},
		{"zero cycle", "cycle-0"},
		{"negative cycle", "cycle--3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := filepath.Join(root, ".evolve", "runs", tc.dir)
			id, err := authoritativeResumeIdentity(root, filepath.Join(ws, CycleStateFile), ws)
			if err == nil || !strings.Contains(err.Error(), "resume identity mismatch") {
				t.Fatalf("workspace %q must be refused as a fleet identity, got id=%+v err=%v", tc.dir, id, err)
			}
			if id.fleet || id.cycle != 0 {
				t.Errorf("a refused identity must carry no authority, got %+v", id)
			}
		})
	}
}

func TestCoverfixS2_IntersectingHunksYieldsNothingWhenEitherDiffIsMalformed(t *testing.T) {
	t.Parallel()
	valid := []byte("diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1,2 +1,2 @@\n-a\n+b\n c\n")
	malformed := []byte("diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ not a header @@\n-a\n+b\n")
	if got := IntersectingHunks(malformed, valid); got != nil {
		t.Errorf("malformed audited diff must scope nothing, got %q", got)
	}
	if got := IntersectingHunks(valid, malformed); got != nil {
		t.Errorf("malformed composed diff must scope nothing, got %q", got)
	}
	if got := IntersectingHunks(valid, valid); len(got) == 0 {
		t.Error("identical valid diffs must still intersect")
	}
}

func TestCoverfixS2_LastLineBreakWithinFallsBackToTheBudgetWhenNoUsableBreakExists(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		s      string
		budget int
		want   int
	}{
		{"no newline", "abcdefghij", 6, 6},
		{"only a leading newline", "\nabcdefghij", 6, 6},
		{"break inside the budget wins", "ab\ncd\nefghij", 8, 5},
	} {
		if got := lastLineBreakWithin(tc.s, tc.budget); got != tc.want {
			t.Errorf("%s: lastLineBreakWithin(%q, %d) = %d, want %d", tc.name, tc.s, tc.budget, got, tc.want)
		}
	}
}

func TestCoverfixS2_StampContinuationManifestStampsNothingWhenPreservationCannotBeProven(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	newO := func() *Orchestrator { return NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil)) }

	t.Run("snapshot_fails_outside_a_git_worktree", func(t *testing.T) {
		t.Parallel()
		ws := filepath.Join(t.TempDir(), "ws")
		if err := os.MkdirAll(ws, 0o755); err != nil {
			t.Fatal(err)
		}
		cs := CycleState{WorkspacePath: ws, ActiveWorktree: t.TempDir()}
		newO().stampContinuationManifest(ctx, cs, 91, t.TempDir())
		if _, ok, _ := continuation.ReadManifest(ws); ok {
			t.Error("a worktree that cannot be snapshotted must not be stamped")
		}
	})

	t.Run("classify_fails_when_main_does_not_exist", func(t *testing.T) {
		t.Parallel()
		repo := gittest.Fixture(t)
		repo.Git("symbolic-ref", "HEAD", "refs/heads/trunk")
		wt := repo.Dir
		if err := os.WriteFile(filepath.Join(wt, "a.txt"), []byte("work\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		ws := filepath.Join(t.TempDir(), "ws")
		if err := os.MkdirAll(ws, 0o755); err != nil {
			t.Fatal(err)
		}
		cs := CycleState{WorkspacePath: ws, ActiveWorktree: wt}
		newO().stampContinuationManifest(ctx, cs, 92, t.TempDir())
		if _, ok, _ := continuation.ReadManifest(ws); ok {
			t.Error("work that cannot be classified against main must not be stamped")
		}
	})

	t.Run("manifest_write_fails", func(t *testing.T) {
		t.Parallel()
		root, wt := initContinuationRepo(t, 94)
		blocker := filepath.Join(t.TempDir(), "blocker")
		if err := os.WriteFile(blocker, []byte("file"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(wt, "feature.go"), []byte("package y\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		ws := filepath.Join(blocker, "ws")
		cs := CycleState{WorkspacePath: ws, ActiveWorktree: wt, WorktreeBaseSHA: gitOut(t, wt, "rev-parse", "HEAD")}
		newO().stampContinuationManifest(ctx, cs, 94, root)
		if _, ok, _ := continuation.ReadManifest(ws); ok {
			t.Error("an unwritable workspace must not yield a manifest")
		}
	})

	t.Run("branch_unresolvable_on_a_detached_head", func(t *testing.T) {
		t.Parallel()
		root, wt := initContinuationRepo(t, 93)
		ws := filepath.Join(root, ".evolve", "runs", "cycle-93")
		if err := os.MkdirAll(ws, 0o755); err != nil {
			t.Fatal(err)
		}
		gitOut(t, wt, "checkout", "--detach")
		if err := os.WriteFile(filepath.Join(wt, "feature.go"), []byte("package y\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		base := gitOut(t, wt, "rev-parse", "HEAD")
		cs := CycleState{WorkspacePath: ws, ActiveWorktree: wt, WorktreeBaseSHA: base}
		newO().stampContinuationManifest(ctx, cs, 93, root)
		if _, ok, _ := continuation.ReadManifest(ws); ok {
			t.Error("a detached HEAD has no branch to resume on; nothing may be stamped")
		}
	})
}

type coverfixS2PathlessVerifier struct{}

func (coverfixS2PathlessVerifier) VerifyDeliverable(context.Context, ReviewInput) (ContractVerification, error) {
	return ContractVerification{OK: false}, nil
}

func TestCoverfixS2_SalvageDeliverableRefusesUnsafeDestinationsAndReportsRelocationFailures(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	newO := func(fv ContractVerifier) *Orchestrator {
		o, _ := ladderOrchestrator(&recordingReviewer{default_: ReviewResult{Approve: true}}, fv, config.StageEnforce)
		return o
	}

	t.Run("contract_resolves_no_artifact_path", func(t *testing.T) {
		t.Parallel()
		sr := newO(coverfixS2PathlessVerifier{}).salvageDeliverable(ctx, ReviewInput{Phase: "build", Workspace: t.TempDir()})
		if sr.Relocated || !strings.Contains(sr.Reason, "resolves no artifact path") {
			t.Errorf("a contract with no artifact path has nothing to salvage into, got %+v", sr)
		}
	})

	t.Run("basename_not_salvage_safe", func(t *testing.T) {
		t.Parallel()
		ws := t.TempDir()
		for _, dest := range []string{ws + "/..", "/"} {
			sr := newO(&fakeVerifier{absDest: dest, alwaysBad: true}).salvageDeliverable(ctx, ReviewInput{Phase: "build", Workspace: ws})
			if sr.Relocated || !strings.Contains(sr.Reason, "not salvage-safe") {
				t.Errorf("dest %q must be refused as unsafe, got %+v", dest, sr)
			}
		}
	})

	t.Run("destination_outside_run_roots", func(t *testing.T) {
		t.Parallel()
		ws, elsewhere := t.TempDir(), t.TempDir()
		if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		sr := newO(&fakeVerifier{absDest: filepath.Join(elsewhere, "build-report.md")}).salvageDeliverable(ctx, ReviewInput{Phase: "build", Workspace: ws, ProjectRoot: t.TempDir()})
		if sr.Relocated || !strings.Contains(sr.Reason, "outside the workspace/.evolve roots") {
			t.Errorf("an out-of-roots destination must never receive a relocation, got %+v", sr)
		}
		if _, err := os.Stat(filepath.Join(elsewhere, "build-report.md")); err == nil {
			t.Error("nothing may be written outside the run roots")
		}
	})

	t.Run("mkdir_of_destination_parent_fails", func(t *testing.T) {
		t.Parallel()
		ws, wt := t.TempDir(), t.TempDir()
		if err := os.WriteFile(filepath.Join(ws, "blocker"), []byte("file"), 0o644); err != nil {
			t.Fatal(err)
		}
		cand := filepath.Join(wt, "build-report.md")
		if err := os.WriteFile(cand, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		fv := &fakeVerifier{absDest: filepath.Join(ws, "blocker", "build-report.md")}
		sr := newO(fv).salvageDeliverable(ctx, ReviewInput{Phase: "build", Workspace: ws, Worktree: wt})
		if sr.Relocated || !strings.Contains(sr.Reason, "mkdir dest") {
			t.Errorf("a mkdir failure must be reported as a skip, got %+v", sr)
		}
		if _, err := os.Stat(cand); err != nil {
			t.Errorf("a failed relocation must leave the candidate in place: %v", err)
		}
	})

	t.Run("rename_onto_a_directory_fails", func(t *testing.T) {
		t.Parallel()
		ws, wt := t.TempDir(), t.TempDir()
		destDir := filepath.Join(ws, "build-report.md")
		if err := os.MkdirAll(filepath.Join(destDir, "child"), 0o755); err != nil {
			t.Fatal(err)
		}
		cand := filepath.Join(wt, "build-report.md")
		if err := os.WriteFile(cand, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		fv := &fakeVerifier{absDest: destDir, alwaysBad: true}
		sr := newO(fv).salvageDeliverable(ctx, ReviewInput{Phase: "build", Workspace: ws, Worktree: wt})
		if sr.Relocated || !strings.Contains(sr.Reason, "rename") {
			t.Errorf("a rename failure must be reported as a skip, got %+v", sr)
		}
	})
}

func TestCoverfixS2_UnresolvablePathsSurfaceAsRefusalNotSuccess(t *testing.T) {
	root := t.TempDir()
	boom := errors.New("injected path resolution failure")

	t.Run("checkpoint_path", func(t *testing.T) {
		orig := absPath
		t.Cleanup(func() { absPath = orig })
		absPath = func(string) (string, error) { return "", boom }
		_, err := authoritativeResumeIdentity(root, filepath.Join(root, CycleStateFile), "")
		if !errors.Is(err, boom) || !strings.Contains(err.Error(), "resolve checkpoint path") {
			t.Errorf("an unresolvable checkpoint path must be refused, got %v", err)
		}
	})

	t.Run("runs_root", func(t *testing.T) {
		orig := absPath
		t.Cleanup(func() { absPath = orig })
		calls := 0
		absPath = func(p string) (string, error) {
			calls++
			if calls == 2 {
				return "", boom
			}
			return orig(p)
		}
		_, err := authoritativeResumeIdentity(root, filepath.Join(root, CycleStateFile), "")
		if !errors.Is(err, boom) || !strings.Contains(err.Error(), "resolve runs root") {
			t.Errorf("an unresolvable runs root must be refused, got %v", err)
		}
	})

	t.Run("salvage_cwd", func(t *testing.T) {
		orig := workingDir
		t.Cleanup(func() { workingDir = orig })
		workingDir = func() (string, error) { return "", boom }
		ws, wt := t.TempDir(), t.TempDir()
		dest := filepath.Join(ws, "deliverables", "build-report.md")
		o, _ := ladderOrchestrator(&recordingReviewer{default_: ReviewResult{Approve: true}}, &fakeVerifier{absDest: dest, alwaysBad: true}, config.StageEnforce)
		sr := o.salvageDeliverable(context.Background(), ReviewInput{Phase: "build", Workspace: ws, Worktree: wt})
		if sr.Relocated || !strings.Contains(sr.Reason, "cwd: unreadable: "+boom.Error()) {
			t.Errorf("an unreadable cwd must be recorded as a skip, got %+v", sr)
		}
	})
}
