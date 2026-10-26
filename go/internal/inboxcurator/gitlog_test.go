package inboxcurator

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestLanded_ReadsEachCommitsPathsAndTextInTheRange(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Write("README.md", "x\n")
	repo.Git("add", "README.md")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	repo.Write("go/a.go", "package a\n")
	repo.Write("go/a_test.go", "package a\n\nfunc TestA_Works(t *testing.T) {}\nfunc helper() {}\n")
	repo.Git("add", "go")
	repo.Git("commit", "-q", "-m", "feat: the pool (closes rolling-pool)")
	head := repo.Git("rev-parse", "HEAD")

	got, err := Landed(context.Background(), gitexec.Default(repo.Dir), base, "HEAD")

	if err != nil || len(got) != 1 {
		t.Fatalf("Landed = %+v, %v; want one change", got, err)
	}
	if got[0].Commit != head || !slices.Equal(got[0].Paths, []string{"go/a.go", "go/a_test.go"}) || got[0].Text != "feat: the pool (closes rolling-pool)" {
		t.Errorf("Landed = %+v, want the head commit, its two paths and its text", got[0])
	}
	if !slices.Equal(got[0].Tests, []string{"TestA_Works"}) || got[0].When.IsZero() {
		t.Errorf("Landed = %+v, want the test that the commit's test file defines and the commit time", got[0])
	}
	one, err := Commit(context.Background(), gitexec.Default(repo.Dir), head)
	if err != nil || len(one) != 1 || one[0].Commit != head {
		t.Errorf("Commit = %+v, %v; want the head commit", one, err)
	}
}

func TestLanded_AnUnknownRevisionIsAnError(t *testing.T) {
	repo := gittest.Fixture(t)

	if _, err := Landed(context.Background(), gitexec.Default(repo.Dir), "0123456789abcdef", "HEAD"); err == nil || errors.Is(err, ErrBadRevision) {
		t.Fatal("Landed of an unknown revision = nil error, want an error")
	}
}

func TestLanded_ARevisionThatIsNotAHexCommitIDIsRefused(t *testing.T) {
	repo := gittest.Fixture(t)
	git := gitexec.Default(repo.Dir)

	for _, rev := range []string{"--output=x", "HEAD~1", ""} {
		if _, err := Landed(context.Background(), git, rev, "HEAD"); !errors.Is(err, ErrBadRevision) {
			t.Errorf("Landed(%q) = %v, want ErrBadRevision", rev, err)
		}
		if _, err := Commit(context.Background(), git, rev); !errors.Is(err, ErrBadRevision) {
			t.Errorf("Commit(%q) = %v, want ErrBadRevision", rev, err)
		}
	}
}

func TestCommit_KeepsAPathWithASpaceOrANonASCIIName(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Write("docs/a b.md", "x\n")
	repo.Write("docs/caf\u00e9.md", "x\n")
	repo.Git("add", "docs")
	repo.Git("commit", "-q", "-m", "docs")

	got, err := Commit(context.Background(), gitexec.Default(repo.Dir), repo.Git("rev-parse", "HEAD"))

	if err != nil || len(got) != 1 || !slices.Equal(got[0].Paths, []string{"docs/a b.md", "docs/caf\u00e9.md"}) {
		t.Errorf("Commit = %+v, %v; want both paths verbatim", got, err)
	}
}

func TestCommit_AFaultReadingTheTestsOfTheCommitIsAnError(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Write("go/a_test.go", "package a\n\nfunc TestA(t *testing.T) {}\n")
	repo.Git("add", "go")
	repo.Git("commit", "-q", "-m", "a test")
	head := repo.Git("rev-parse", "HEAD")
	gittest.FaultOn(t, "grep")

	if _, err := Commit(context.Background(), gitexec.Default(repo.Dir), head); err == nil || !strings.Contains(err.Error(), "read the tests of commit") {
		t.Errorf("Commit = %v, want the grep fault named", err)
	}
}

func TestIsCommitID_AcceptsOnlyAHexCommitID(t *testing.T) {
	for rev, want := range map[string]bool{"0123abc": true, "HEAD": false, "HEAD~1": false, "--output=x": false, "": false} {
		if got := IsCommitID(rev); got != want {
			t.Errorf("IsCommitID(%q) = %v, want %v", rev, got, want)
		}
	}
}
