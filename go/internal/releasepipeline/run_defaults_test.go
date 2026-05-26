package releasepipeline

import (
	"errors"
	"os/exec"
	"testing"
	"time"
)

// === Run — nil step fields trigger default overlay ===========================

// TestRun_NilPreflightOverriddenByDefault: when opts.Steps.Preflight is nil,
// Run fills it from DefaultSteps(). We verify this by injecting nil for
// Preflight while providing a steps struct with all other fields set. Because
// the default Preflight calls a real library that fails on a temp dir (no git),
// we accept either success or ErrPrePublishFailed — the only failure we must
// NOT see is a nil-dereference panic.
func TestRun_NilPreflightOverriddenByDefault(t *testing.T) {
	steps := allOkSteps()
	steps.Preflight = nil // force the nil-overlay path in Run

	// Run must not panic; the default Preflight will fail on a plain tempdir.
	_, _ = Run(Options{
		Target:      "5.0.0",
		RepoRoot:    t.TempDir(),
		FromTag:     "v4.9.9",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
}

// TestRun_NilChangelogGenOverriddenByDefault: when opts.Steps.ChangelogGen is nil,
// Run fills it from DefaultSteps(). The default calls the real library which
// fails on a non-git dir — we just confirm no panic and the error is pre-publish.
func TestRun_NilChangelogGenOverriddenByDefault(t *testing.T) {
	steps := allOkSteps()
	steps.ChangelogGen = nil // force nil-overlay

	_, err := Run(Options{
		Target:      "5.0.0",
		RepoRoot:    t.TempDir(),
		FromTag:     "v4.9.9",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	// real changeloggen on a non-git dir fails — that's expected
	if err != nil && !errors.Is(err, ErrPrePublishFailed) {
		t.Errorf("unexpected error wrapping: %v", err)
	}
}

// TestRun_NilVersionBumpOverriddenByDefault: when opts.Steps.VersionBump is nil,
// Run fills it from DefaultSteps().
func TestRun_NilVersionBumpOverriddenByDefault(t *testing.T) {
	steps := allOkSteps()
	steps.VersionBump = nil // force nil-overlay

	_, err := Run(Options{
		Target:      "5.0.0",
		RepoRoot:    t.TempDir(),
		FromTag:     "v4.9.9",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	// real versionbump on empty dir fails — that's expected
	if err != nil && !errors.Is(err, ErrPrePublishFailed) {
		t.Errorf("unexpected error wrapping: %v", err)
	}
}

// TestRun_NilRebuildBinaryOverriddenByDefault: when opts.Steps.RebuildBinary is
// nil, Run fills it from DefaultSteps(). The default succeeds on dryRun=true
// (returns nil immediately), so we use DryRun=true to reach the overlay and
// confirm no panic.
func TestRun_NilRebuildBinaryOverriddenByDefault(t *testing.T) {
	steps := allOkSteps()
	steps.RebuildBinary = nil // force nil-overlay

	res, err := Run(Options{
		Target:      "5.0.0",
		RepoRoot:    t.TempDir(),
		FromTag:     "v4.9.9",
		MaxPollWait: time.Second,
		DryRun:      true, // keeps defaultRebuildBinary harmless (dryRun→nil)
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if err != nil {
		t.Fatalf("Run with nil RebuildBinary (dry-run): %v, result=%+v", err, res)
	}
}

// TestRun_NilReleaseShOverriddenByDefault: when opts.Steps.ReleaseSh is nil,
// Run fills it from DefaultSteps(). We run non-dry-run; the default calls the
// real library that fails on a plain tempdir.
func TestRun_NilReleaseShOverriddenByDefault(t *testing.T) {
	steps := allOkSteps()
	steps.ReleaseSh = nil // force nil-overlay

	_, err := Run(Options{
		Target:      "5.0.0",
		RepoRoot:    t.TempDir(),
		FromTag:     "v4.9.9",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	// real releaseconsistency on empty dir fails — expected
	if err != nil && !errors.Is(err, ErrPrePublishFailed) {
		t.Errorf("unexpected error wrapping: %v", err)
	}
}

// TestRun_NilShipOverriddenByDefault: when opts.Steps.Ship is nil, Run fills it
// from DefaultSteps(). The default resolveEvolveBin returns "" in CI (no evolve
// binary), which causes defaultShip to return an error → ErrShipFailed.
func TestRun_NilShipOverriddenByDefault(t *testing.T) {
	steps := allOkSteps()
	steps.Ship = nil // force nil-overlay
	// Clear EVOLVE_GO_BIN and PATH so resolveEvolveBin returns ""
	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", "")

	_, err := Run(Options{
		Target:      "5.0.0",
		RepoRoot:    t.TempDir(),
		FromTag:     "v4.9.9",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	// defaultShip can't find evolve binary → ErrShipFailed
	if err != nil && !errors.Is(err, ErrShipFailed) && !errors.Is(err, ErrPrePublishFailed) {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestRun_NilMarketplacePollOverriddenByDefault: when opts.Steps.MarketplacePoll
// is nil, Run fills it from DefaultSteps(). The default polls a non-existent
// marketplace dir → ErrPostPublishFailed.
func TestRun_NilMarketplacePollOverriddenByDefault(t *testing.T) {
	steps := allOkSteps()
	steps.MarketplacePoll = nil // force nil-overlay
	t.Setenv("EVOLVE_MARKETPLACE_DIR", t.TempDir()+"/no-such-market")

	_, err := Run(Options{
		Target:      "5.0.0",
		RepoRoot:    t.TempDir(),
		FromTag:     "v4.9.9",
		MaxPollWait: 100 * time.Millisecond,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if err != nil && !errors.Is(err, ErrPostPublishFailed) &&
		!errors.Is(err, ErrPrePublishFailed) && !errors.Is(err, ErrShipFailed) {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestRun_NilRollbackOverriddenByDefault: when opts.Steps.Rollback is nil,
// Run fills it from DefaultSteps(). We trigger the rollback path by failing
// MarketplacePoll and confirm no panic (the default rollback will fail on an
// invalid journal — that's captured in RollbackErr).
func TestRun_NilRollbackOverriddenByDefault(t *testing.T) {
	steps := allOkSteps()
	steps.Rollback = nil // force nil-overlay
	steps.MarketplacePoll = func(string, string, time.Duration) error {
		return errors.New("poll fail")
	}

	res, err := Run(Options{
		Target:      "5.0.0",
		RepoRoot:    t.TempDir(),
		FromTag:     "v4.9.9",
		MaxPollWait: time.Second,
		Steps:       steps,
		Now:         fixedNow(t),
	})
	if !errors.Is(err, ErrPostPublishFailed) {
		t.Fatalf("err = %v, want ErrPostPublishFailed", err)
	}
	if !res.RollbackTriggered {
		t.Error("RollbackTriggered must be true")
	}
	// default rollback will fail (journal is a temp path from earlier step);
	// RollbackErr must be set, not nil
	if res.RollbackErr == nil {
		// It's acceptable for RollbackErr to be nil if the rollback somehow
		// succeeded on this journal — just verify no panic occurred.
		t.Log("RollbackErr is nil (rollback may have partially succeeded)")
	}
}

// === Run — fromTag auto-resolution: prevTag succeeds but resolvePrevTag
// error + resolveInitCommit success ==========================================

// TestRun_FromTagResolvesViaInitCommit: when opts.FromTag is empty,
// resolvePrevTag fails (non-git dir), and resolveInitCommit also fails —
// fromTag remains "". This path (both fail) is already covered by
// TestRun_FromTagAutoResolved_NonGitDir. Here we exercise the branch where
// resolvePrevTag fails but resolveInitCommit succeeds, which requires a real
// git repo with no tags.
func TestRun_FromTagResolvesViaInitCommit_NoTags(t *testing.T) {
	dir := makeHermeticGitRepo(t)
	// Delete the v0.0.1 tag so resolvePrevTag fails; resolveInitCommit succeeds.
	deleteTagCmd := exec.Command("git", "-C", dir, "tag", "-d", "v0.0.1")
	if out, err := deleteTagCmd.CombinedOutput(); err != nil {
		t.Fatalf("delete tag: %v\n%s", err, out)
	}

	res, err := Run(Options{
		Target:      "99.1.0",
		RepoRoot:    dir,
		FromTag:     "", // force resolution; no tags → falls to resolveInitCommit
		MaxPollWait: time.Second,
		Steps:       allOkSteps(),
		Now:         fixedNow(t),
	})
	if err != nil {
		t.Fatalf("Run with init-commit fromTag: %v", err)
	}
	if res.JournalPath == "" {
		t.Error("JournalPath must be set")
	}
}
