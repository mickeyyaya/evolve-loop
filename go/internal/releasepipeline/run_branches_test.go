package releasepipeline

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// === Run — nil Now uses real time.Now ========================================

// TestRun_NilNowUsesRealClock: when opts.Now is nil, Run wires time.Now
// internally. The journal file must still be written with a non-zero StartedAt.
func TestRun_NilNowUsesRealClock(t *testing.T) {
	res, err := Run(Options{
		Target:      "2.0.0",
		RepoRoot:    t.TempDir(),
		FromTag:     "v1.9.9",
		MaxPollWait: time.Second,
		Steps:       allOkSteps(),
		Now:         nil, // must not panic
	})
	if err != nil {
		t.Fatalf("Run with nil Now: %v", err)
	}
	if res.JournalPath == "" {
		t.Error("JournalPath must be set even with nil Now")
	}
}

// === Run — nil Steps fields default to DefaultSteps wrappers =================

// TestRun_NilStepsAreFilled: when opts.Steps has nil function fields,
// Run fills them from DefaultSteps() before executing. We verify by providing
// a nil Preflight and confirming no nil-dereference panic; the fallback calls
// the real library but we short-circuit with the remaining injected steps.
//
// Because the real Preflight hits a live git repo, we inject only the important
// branches (non-nil Preflight is required for a real pipeline), but we test the
// nil-overlay path for a step that is safe: FullDryRunPreflight (step 0 is
// skipped when RequirePreflight==false, so the nil value is never called).
func TestRun_NilFullDryRunPreflightNotCalled(t *testing.T) {
	steps := allOkSteps()
	steps.FullDryRunPreflight = nil // nil, but RequirePreflight=false so never invoked

	res, err := Run(Options{
		Target:      "3.0.0",
		RepoRoot:    t.TempDir(),
		FromTag:     "v2.9.9",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if err != nil {
		t.Fatalf("Run with nil FullDryRunPreflight (unused) = %v", err)
	}
	if !contains(res.StepsCompleted, "preflight") {
		t.Errorf("StepsCompleted = %v, want 'preflight'", res.StepsCompleted)
	}
}

// === Run — fromTag resolution via resolvePrevTag (no FromTag provided) =======

// TestRun_FromTagAutoResolved_ValidRepo: when opts.FromTag is empty, Run calls
// resolvePrevTag. If the repo has at least one tag it picks the previous tag;
// if not, it falls through to resolveInitCommit. Either way the pipeline must
// still complete without error (the resolved fromTag may be a SHA or a tag).
func TestRun_FromTagAutoResolved_ValidRepo(t *testing.T) {
	// We use the actual repo root which has real tags.
	res, err := Run(Options{
		Target:      "99.0.0",
		RepoRoot:    findRepoRoot(t),
		FromTag:     "", // force auto-resolution
		MaxPollWait: time.Second,
		Steps:       allOkSteps(),
		Now:         fixedNow(t),
	})
	if err != nil {
		t.Fatalf("Run with auto-resolved FromTag: %v", err)
	}
	if res.JournalPath == "" {
		t.Error("JournalPath not set")
	}
}

// TestRun_FromTagAutoResolved_NonGitDir: when opts.FromTag is empty and
// resolvePrevTag fails (non-git dir), Run falls through to resolveInitCommit,
// which also fails, so fromTag stays empty. The pipeline still proceeds with
// an empty fromTag (changelog range will be "..HEAD").
func TestRun_FromTagAutoResolved_NonGitDir(t *testing.T) {
	dir := t.TempDir() // not a git repo

	res, err := Run(Options{
		Target:      "99.0.0",
		RepoRoot:    dir,
		FromTag:     "", // force auto-resolution; both git calls fail
		MaxPollWait: time.Second,
		Steps:       allOkSteps(),
		Now:         fixedNow(t),
	})
	if err != nil {
		t.Fatalf("Run with unresolvable fromTag: %v", err)
	}
	if res.JournalPath == "" {
		t.Error("JournalPath not set")
	}
}

// === Run — ChangelogGen failure halts at pre-publish =========================

// TestRun_ChangelogGenFails verifies that a changelog-gen failure returns
// ErrPrePublishFailed and stops the pipeline before version-bump.
func TestRun_ChangelogGenFails(t *testing.T) {
	steps := allOkSteps()
	steps.ChangelogGen = func(string, string, string, string, bool) error {
		return errors.New("simulated changelog error")
	}
	versionBumpCalled := 0
	steps.VersionBump = func(string, string, bool) error {
		versionBumpCalled++
		return nil
	}

	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    t.TempDir(),
		FromTag:     "v1.2.2",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if !errors.Is(err, ErrPrePublishFailed) {
		t.Fatalf("err = %v, want ErrPrePublishFailed", err)
	}
	if versionBumpCalled != 0 {
		t.Errorf("VersionBump called %d times after changelog-gen failure (want 0)", versionBumpCalled)
	}
	if !contains(res.StepsFailed, "changelog-gen") {
		t.Errorf("StepsFailed = %v, want contains 'changelog-gen'", res.StepsFailed)
	}
}

// === Run — VersionBump failure halts at pre-publish ==========================

// TestRun_VersionBumpFails verifies that a version-bump failure returns
// ErrPrePublishFailed and stops the pipeline before rebuild-binary.
func TestRun_VersionBumpFails(t *testing.T) {
	steps := allOkSteps()
	steps.VersionBump = func(string, string, bool) error {
		return errors.New("simulated version-bump error")
	}
	rebuildCalled := 0
	steps.RebuildBinary = func(string, bool) error {
		rebuildCalled++
		return nil
	}

	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    t.TempDir(),
		FromTag:     "v1.2.2",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if !errors.Is(err, ErrPrePublishFailed) {
		t.Fatalf("err = %v, want ErrPrePublishFailed", err)
	}
	if rebuildCalled != 0 {
		t.Errorf("RebuildBinary called %d times after version-bump failure (want 0)", rebuildCalled)
	}
	if !contains(res.StepsFailed, "version-bump") {
		t.Errorf("StepsFailed = %v, want contains 'version-bump'", res.StepsFailed)
	}
}

// === Run — ReleaseSh failure halts at pre-publish ============================

// TestRun_ReleaseShFails verifies that a release-sh-check failure returns
// ErrPrePublishFailed and stops the pipeline before ship.
func TestRun_ReleaseShFails(t *testing.T) {
	steps := allOkSteps()
	steps.ReleaseSh = func(string, string) error {
		return errors.New("consistency check failed")
	}
	shipCalled := 0
	steps.Ship = func(string, string, string) (string, error) {
		shipCalled++
		return "sha", nil
	}

	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    t.TempDir(),
		FromTag:     "v1.2.2",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if !errors.Is(err, ErrPrePublishFailed) {
		t.Fatalf("err = %v, want ErrPrePublishFailed", err)
	}
	if shipCalled != 0 {
		t.Errorf("Ship called %d times after release-sh failure (want 0)", shipCalled)
	}
	if !contains(res.StepsFailed, "release-sh-check") {
		t.Errorf("StepsFailed = %v, want contains 'release-sh-check'", res.StepsFailed)
	}
}

// === Run — journal init failure returns ErrPrePublishFailed ==================

// TestRun_JournalInitFails_UnwritableDir: when the journal directory cannot be
// created (opts.JournalDir points to a file, not a directory), initJournal
// returns an error and Run wraps it as ErrPrePublishFailed.
func TestRun_JournalInitFails_UnwritableDir(t *testing.T) {
	dir := t.TempDir()
	// Create a FILE at the path where MkdirAll would need to create a directory.
	// MkdirAll will fail trying to create a directory over an existing file.
	blockingFile := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blockingFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// The journal dir is set to be a path under the blocking file (impossible dir).
	impossibleDir := filepath.Join(blockingFile, "release-journal")

	_, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    dir,
		FromTag:     "v1.2.2",
		MaxPollWait: time.Second,
		JournalDir:  impossibleDir,
		Steps:       allOkSteps(),
		Now:         fixedNow(t),
	})
	if !errors.Is(err, ErrPrePublishFailed) {
		t.Fatalf("err = %v, want ErrPrePublishFailed on journal init failure", err)
	}
	if !strings.Contains(err.Error(), "journal init") {
		t.Errorf("err = %q, want contains 'journal init'", err.Error())
	}
}

// === Run — dry-run with release.sh also skipped (non-DryRun branches) =======

// TestRun_DryRun_JournalHasDryRunSteps: in dry-run mode, rebuild-binary and
// release-sh-check journal entries should have status "skipped-dry-run".
func TestRun_DryRun_JournalHasDryRunSteps(t *testing.T) {
	res, err := Run(Options{
		Target:      "1.2.3",
		RepoRoot:    t.TempDir(),
		FromTag:     "v1.2.2",
		MaxPollWait: time.Second,
		DryRun:      true,
		Steps:       allOkSteps(),
		Now:         fixedNow(t),
	})
	if err != nil {
		t.Fatalf("dry-run err = %v", err)
	}
	// Journal must record that rebuild-binary and release-sh-check were skipped.
	body, err := os.ReadFile(res.JournalPath)
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	if !strings.Contains(string(body), "skipped-dry-run") {
		t.Error("dry-run journal must contain 'skipped-dry-run' status records")
	}
}
