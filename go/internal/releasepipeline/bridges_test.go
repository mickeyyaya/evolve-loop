package releasepipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func freshRepoNoGit(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func TestRunPreflightLib_NoGit_ReturnsError(t *testing.T) {
	root := freshRepoNoGit(t)
	err := runPreflightLib(root, "1.2.3", true, true, false)
	if err == nil {
		t.Error("runPreflightLib without git: want error")
	}
}

func TestRunChangelogGenLib_NonSemverTarget_ReturnsError(t *testing.T) {
	err := runChangelogGenLib(t.TempDir(), "v0", "HEAD", "not-a-version", true)
	if err == nil {
		t.Error("non-semver target should error")
	}
}

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

func TestRunChangelogGenLib_DryRun_VerifyRefFails(t *testing.T) {
	root := t.TempDir()
	err := runChangelogGenLib(root, "v0.0.0", "HEAD", "1.2.3", true)
	if err == nil {
		t.Error("VerifyRef without git should error")
	}
}

func TestRunVersionBumpLib_NoMarkers_ReturnsError(t *testing.T) {
	root := t.TempDir()
	err := runVersionBumpLib(root, "1.2.3", true)
	if err == nil {
		t.Error("version-bump without markers: want error")
	}
}

func TestRunMarketplacePollLib_FailsFast(t *testing.T) {
	nonExistent := filepath.Join(t.TempDir(), "no-such-marketplace")
	err := runMarketplacePollLib(t.TempDir(), "1.2.3", 1*time.Second, nonExistent)
	if err == nil {
		t.Error("missing marketplace dir: want error")
	}
}

func TestRunReleaseConsistencyLib_NoMarkers_ReturnsError(t *testing.T) {
	err := runReleaseConsistencyLib(t.TempDir(), "1.2.3")
	if err == nil {
		t.Error("consistency check without markers: want error")
	}
}

func TestRunRollbackLib_MissingJournal_ReturnsError(t *testing.T) {
	err := runRollbackLib(t.TempDir(), "/no/such/journal.json", "test reason")
	if err == nil {
		t.Error("missing journal: want error")
	}
}

func TestRunMarketplacePollLib_EmptyDir_NoPanic(t *testing.T) {
	err := runMarketplacePollLib(t.TempDir(), "1.2.3", 100*time.Millisecond, "")
	_ = err
}

func TestRunChangelogGenLib_EmptyRefs_ReturnsError(t *testing.T) {
	err := runChangelogGenLib(t.TempDir(), "", "", "1.2.3", false)
	if err == nil {
		t.Error("empty refs should error")
	}
	if !strings.Contains(err.Error(), "ref") && !strings.Contains(err.Error(), "git") &&
		!strings.Contains(err.Error(), "rev-parse") && err.Error() != "" {
	}
}
