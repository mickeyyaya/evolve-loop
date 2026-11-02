package qualitypanel

import (
	"path/filepath"
	"testing"
)

func TestGitRevertRunner_AScratchTheRepoCannotAddIsAnError(t *testing.T) {
	repo, base, ship := shipFixture(t)
	hunks := ParseHunks(repo.Git("diff", base, ship))
	runner := GitRevertRunner{Repo: repo.Dir, Commit: "no-such-commit"}
	if outcome, err := runner.Revert(filepath.Join(t.TempDir(), "scratch"), hunks[0]); err == nil {
		t.Errorf("outcome = %q, err = nil, want an error for a commit the repo lacks", outcome)
	}
}

func TestGitRevertRunner_APackageDirectoryThatCannotRunGoIsAnErrorNotAFailedTest(t *testing.T) {
	repo, base, ship := shipFixture(t)
	h := ParseHunks(repo.Git("diff", base, ship))[0]
	h.Package = "no/such/dir"
	outcome, err := GitRevertRunner{Repo: repo.Dir, Commit: ship}.Revert(filepath.Join(t.TempDir(), "scratch"), h)
	if err == nil {
		t.Errorf("outcome = %q, err = nil, want an error, not a silent sensitive count", outcome)
	}
}
