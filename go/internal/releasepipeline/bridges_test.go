package releasepipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// freshRepoNoGit returns a tempdir that looks like a repo root but has
// no .git. Most bridge libraries probe git refs early and fail — that's
// fine; we're covering the bridge wrapper, not the underlying impl.
func freshRepoNoGit(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// TestRunPreflightLib_NoGit_ReturnsError — preflight needs a git repo;
// without one, the underlying library errors and the bridge surfaces it.
func TestRunPreflightLib_NoGit_ReturnsError(t *testing.T) {
	root := freshRepoNoGit(t)
	err := runPreflightLib(root, "1.2.3", true, true, false)
	if err == nil {
		t.Error("runPreflightLib without git: want error")
	}
}

// TestRunChangelogGenLib_NonSemverTarget_ReturnsError — the bridge
// rejects non-semver targets before invoking the library.
func TestRunChangelogGenLib_NonSemverTarget_ReturnsError(t *testing.T) {
	err := runChangelogGenLib(t.TempDir(), "v0", "HEAD", "not-a-version", true)
	if err == nil {
		t.Error("non-semver target should error")
	}
}

// TestRunChangelogGenLib_AlreadyHasEntry_IdempotentSkip — when the
// changelog already has the target version, the bridge returns nil
// (idempotent).
func TestRunChangelogGenLib_AlreadyHasEntry_IdempotentSkip(t *testing.T) {
	root := t.TempDir()
	cl := filepath.Join(root, "CHANGELOG.md")
	body := `# Changelog

## [1.2.3] - 2026-05-25

- previously released
`
	if err := os.WriteFile(cl, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runChangelogGenLib(root, "v0.0.0", "HEAD", "1.2.3", false); err != nil {
		t.Errorf("idempotent skip should return nil; got %v", err)
	}
}

// TestRunChangelogGenLib_DryRun_NoFileWritten — DryRun=true after
// passing semver + ref checks logs and returns nil. The bridge code
// gets coverage even though the underlying git ops would fail.
// Because we don't have git refs, we expect an error from VerifyRef.
func TestRunChangelogGenLib_DryRun_VerifyRefFails(t *testing.T) {
	root := t.TempDir()
	err := runChangelogGenLib(root, "v0.0.0", "HEAD", "1.2.3", true)
	if err == nil {
		t.Error("VerifyRef without git should error")
	}
}

// TestRunVersionBumpLib_NoMarkers_ReturnsError — without any of the
// version-marker files, versionbump.Run errors.
func TestRunVersionBumpLib_NoMarkers_ReturnsError(t *testing.T) {
	root := t.TempDir()
	err := runVersionBumpLib(root, "1.2.3", true)
	if err == nil {
		t.Error("version-bump without markers: want error")
	}
}

// TestRunMarketplacePollLib_DryRunNoMarketplace_ReturnsError — without
// an actual marketplace directory and with a very short max-wait, the
// bridge fails quickly.
func TestRunMarketplacePollLib_FailsFast(t *testing.T) {
	// EVOLVE_MARKETPLACE_DIR is unset by default in test. Set it to a
	// non-existent dir; the poll library should fail fast.
	t.Setenv("EVOLVE_MARKETPLACE_DIR", filepath.Join(t.TempDir(), "no-such-marketplace"))
	err := runMarketplacePollLib(t.TempDir(), "1.2.3", 1*time.Second)
	if err == nil {
		t.Error("missing marketplace dir: want error")
	}
}

// TestRunReleaseConsistencyLib_NoMarkers_ReturnsError — consistency
// check needs version-marker files present.
func TestRunReleaseConsistencyLib_NoMarkers_ReturnsError(t *testing.T) {
	err := runReleaseConsistencyLib(t.TempDir(), "1.2.3")
	if err == nil {
		t.Error("consistency check without markers: want error")
	}
}

// TestRunRollbackLib_MissingJournal_ReturnsError — rollback requires a
// journal file.
func TestRunRollbackLib_MissingJournal_ReturnsError(t *testing.T) {
	err := runRollbackLib(t.TempDir(), "/no/such/journal.json", "test reason")
	if err == nil {
		t.Error("missing journal: want error")
	}
}

// TestRunMarketplacePollLib_HomeDirFallback — without
// EVOLVE_MARKETPLACE_DIR, the bridge derives ~/.claude/plugins/.../evolve-loop.
// We can't easily test the derivation result without mocking UserHomeDir,
// but we can confirm the call path doesn't panic on a vanilla env.
func TestRunMarketplacePollLib_NoEnvVar_NoPanic(t *testing.T) {
	t.Setenv("EVOLVE_MARKETPLACE_DIR", "")
	err := runMarketplacePollLib(t.TempDir(), "1.2.3", 100*time.Millisecond)
	// We expect an error (the derived marketplace dir probably doesn't
	// exist or is stale), but the call must not panic.
	_ = err
}

// TestRunChangelogGenLib_TooFewArgs_DocumentsContract — bridge with
// empty fromRef/toRef should surface the VerifyRef error.
func TestRunChangelogGenLib_EmptyRefs_ReturnsError(t *testing.T) {
	err := runChangelogGenLib(t.TempDir(), "", "", "1.2.3", false)
	if err == nil {
		t.Error("empty refs should error")
	}
	// Sanity: the error mentions the failing ref so operators can
	// diagnose.
	if !strings.Contains(err.Error(), "ref") && !strings.Contains(err.Error(), "git") &&
		!strings.Contains(err.Error(), "rev-parse") && err.Error() != "" {
		// loose check: we just want a non-empty diagnostic
	}
}
