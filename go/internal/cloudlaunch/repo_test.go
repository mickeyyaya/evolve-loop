package cloudlaunch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func pushedRepo(t *testing.T, url string) *gittest.Repo {
	t.Helper()
	r := gittest.Fixture(t)
	r.Git("checkout", "-q", "-b", "feature/x")
	if err := os.WriteFile(filepath.Join(r.Dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.Git("add", "a.txt")
	r.Git("commit", "-q", "-m", "a")
	r.Git("remote", "add", "origin", url)
	r.Git("update-ref", "refs/remotes/origin/feature/x", "HEAD")
	r.Git("config", "branch.feature/x.remote", "origin")
	r.Git("config", "branch.feature/x.merge", "refs/heads/feature/x")
	return r
}

func refusalReason(t *testing.T, err error) string {
	t.Helper()
	var r *Refusal
	if !errors.As(err, &r) {
		t.Fatalf("error = %v, want a *Refusal", err)
	}
	if err.Error() != r.Reason {
		t.Errorf("Refusal.Error() = %q, want the reason %q", err.Error(), r.Reason)
	}
	return r.Reason
}

func TestReadRepo_APushedBranchGivesItsRemoteAndNoCommitAhead(t *testing.T) {
	r := pushedRepo(t, "https://github.com/acme/widgets.git")
	got, err := ReadRepo(context.Background(), sysexec.DefaultRunner, r.Dir)
	if err != nil {
		t.Fatalf("ReadRepo = %v", err)
	}
	want := Repo{Branch: "feature/x", Remote: "origin", Merge: "refs/heads/feature/x", URL: "https://github.com/acme/widgets.git", Ahead: 0}
	if got != want {
		t.Errorf("ReadRepo = %+v, want %+v", got, want)
	}
}

func TestReadRepo_CountsTheCommitsThatAreNotPushed(t *testing.T) {
	r := pushedRepo(t, "https://github.com/acme/widgets.git")
	r.Git("commit", "-q", "--allow-empty", "-m", "b")
	r.Git("commit", "-q", "--allow-empty", "-m", "c")
	got, err := ReadRepo(context.Background(), sysexec.DefaultRunner, r.Dir)
	if err != nil {
		t.Fatalf("ReadRepo = %v", err)
	}
	if got.Ahead != 2 {
		t.Errorf("ReadRepo Ahead = %d, want 2", got.Ahead)
	}
}

func TestReadRepo_RefusesATreeItCannotLaunchFrom(t *testing.T) {
	cases := []struct {
		name  string
		setup func(r *gittest.Repo)
		want  string
	}{
		{"detached HEAD", func(r *gittest.Repo) { r.Git("checkout", "-q", "--detach") }, "on no branch"},
		{"no upstream remote", func(r *gittest.Repo) { r.Git("config", "--unset", "branch.feature/x.remote") }, "has no upstream"},
		{"no upstream branch", func(r *gittest.Repo) { r.Git("config", "--unset", "branch.feature/x.merge") }, "has no upstream branch"},
		{"a remote with no URL", func(r *gittest.Repo) { r.Git("config", "branch.feature/x.remote", "nosuch") }, "has no URL"},
		{"no upstream ref", func(r *gittest.Repo) { r.Git("update-ref", "-d", "refs/remotes/origin/feature/x") }, "cannot be compared"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := pushedRepo(t, "https://github.com/acme/widgets.git")
			tc.setup(r)
			_, err := ReadRepo(context.Background(), sysexec.DefaultRunner, r.Dir)
			if reason := refusalReason(t, err); !strings.Contains(reason, tc.want) {
				t.Errorf("refusal = %q, want it to contain %q", reason, tc.want)
			}
		})
	}
}

func TestCheck_RefusesUnpushedCommitsAndAnotherHost(t *testing.T) {
	hosts := []string{"github.com"}
	pushed := Repo{Branch: "main", Remote: "origin", URL: "git@github.com:acme/widgets.git"}
	if err := pushed.Check(hosts); err != nil {
		t.Errorf("Check of a pushed GitHub branch = %v, want nil", err)
	}
	for _, n := range []int{1, 3} {
		ahead := pushed
		ahead.Ahead = n
		if reason := refusalReason(t, ahead.Check(hosts)); !strings.Contains(reason, fmt.Sprintf("%d commit(s) that are not pushed", n)) {
			t.Errorf("unpushed refusal for %d = %q", n, reason)
		}
	}
	other := pushed
	other.URL = "https://user:secret@gitlab.com/acme/widgets.git"
	reason := refusalReason(t, other.Check(hosts))
	if !strings.Contains(reason, "gitlab.com") || !strings.Contains(reason, "not on a host of the manifest (github.com)") {
		t.Errorf("host refusal = %q, want it to name the host", reason)
	}
	if strings.Contains(reason, "secret") {
		t.Errorf("host refusal = %q leaks the credential of the URL", reason)
	}
}

func TestRemoteHostAndRepoSlug_ReadEachURLForm(t *testing.T) {
	cases := []struct{ url, host, slug string }{
		{"https://github.com/acme/widgets.git", "github.com", "acme/widgets"},
		{"https://x-access-token:t@github.com:443/acme/widgets", "github.com", "acme/widgets"},
		{"ssh://git@github.com/acme/widgets.git", "github.com", "acme/widgets"},
		{"git@github.com:acme/widgets.git", "github.com", "acme/widgets"},
		{"github.com:acme/widgets", "github.com", "acme/widgets"},
		{"git@GitHub.com:acme/widgets.git", "github.com", "acme/widgets"},
		{"https://GitHub.com/acme/widgets", "github.com", "acme/widgets"},
		{"/srv/git/widgets.git", "", ""},
		{"https://%zz", "", ""},
	}
	for _, tc := range cases {
		if got := RemoteHost(tc.url); got != tc.host {
			t.Errorf("RemoteHost(%q) = %q, want %q", tc.url, got, tc.host)
		}
		if got := RepoSlug(tc.url); got != tc.slug {
			t.Errorf("RepoSlug(%q) = %q, want %q", tc.url, got, tc.slug)
		}
	}
}

func TestReadRepo_RefusesAnUnpushedCountThatIsNotANumber(t *testing.T) {
	r := pushedRepo(t, "https://github.com/acme/widgets.git")
	wrapped := func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && len(args) > 0 && args[0] == "rev-list" {
			_, _ = io.WriteString(stdout, "hook: 2 commits\n")
			return 0, nil
		}
		return sysexec.DefaultRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}
	_, err := ReadRepo(context.Background(), wrapped, r.Dir)
	if reason := refusalReason(t, err); !strings.Contains(reason, "is not a number") {
		t.Errorf("refusal = %q, want the count named as not a number", reason)
	}
}

func remoteBackedRepo(t *testing.T) (*gittest.Repo, *gittest.Repo, Repo) {
	t.Helper()
	bare := gittest.Bare(t)
	r := pushedRepo(t, bare.Dir)
	r.Git("push", "-q", "origin", "feature/x")
	repo, err := ReadRepo(context.Background(), sysexec.DefaultRunner, r.Dir)
	if err != nil {
		t.Fatalf("ReadRepo = %v", err)
	}
	return r, bare, repo
}

func TestRepoMatchRemote_AcceptsARemoteHeadThatIsTheLocalHead(t *testing.T) {
	r, _, repo := remoteBackedRepo(t)
	if err := repo.MatchRemote(context.Background(), sysexec.DefaultRunner, r.Dir); err != nil {
		t.Errorf("MatchRemote of a pushed branch = %v, want nil", err)
	}
}

func TestRepoMatchRemote_RefusesARemoteHeadThatDiffersFromTheLocalHead(t *testing.T) {
	cases := map[string]func(t *testing.T, r, bare *gittest.Repo){
		"a push from elsewhere": func(t *testing.T, _, bare *gittest.Repo) {
			other := gittest.Clone(t, bare.Dir)
			other.Git("checkout", "-q", "feature/x")
			other.Git("commit", "-q", "--allow-empty", "-m", "elsewhere")
			other.Git("push", "-q", "origin", "feature/x")
		},
		"a local commit after the check": func(_ *testing.T, r, _ *gittest.Repo) {
			r.Git("commit", "-q", "--allow-empty", "-m", "late")
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			r, bare, repo := remoteBackedRepo(t)
			change(t, r, bare)
			err := repo.MatchRemote(context.Background(), sysexec.DefaultRunner, r.Dir)
			if reason := refusalReason(t, err); !strings.Contains(reason, "differs from the local head") || !strings.Contains(reason, "feature/x") {
				t.Errorf("refusal = %q, want the differing heads named", reason)
			}
			if strings.Contains(err.Error(), bare.Dir) {
				t.Errorf("refusal = %q names the URL of the remote", err)
			}
		})
	}
}

func TestRepoMatchRemote_RefusesWhenAHeadCannotBeRead(t *testing.T) {
	r, _, repo := remoteBackedRepo(t)
	noBranch := repo
	noBranch.Merge = "refs/heads/absent"
	if reason := refusalReason(t, noBranch.MatchRemote(context.Background(), sysexec.DefaultRunner, r.Dir)); !strings.Contains(reason, "cannot read the head of refs/heads/absent on origin") {
		t.Errorf("refusal for an absent remote branch = %q", reason)
	}
	if reason := refusalReason(t, repo.MatchRemote(context.Background(), sysexec.DefaultRunner, t.TempDir())); !strings.Contains(reason, "has no local head") {
		t.Errorf("refusal outside a repository = %q", reason)
	}
	suffix := func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && len(args) > 0 && args[0] == "ls-remote" {
			_, _ = io.WriteString(stdout, "0123\trefs/heads/other/refs/heads/feature/x\n")
			return 0, nil
		}
		return sysexec.DefaultRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}
	if reason := refusalReason(t, repo.MatchRemote(context.Background(), suffix, r.Dir)); !strings.Contains(reason, "cannot read the head of refs/heads/feature/x on origin") {
		t.Errorf("refusal for a ref that only ends with the branch = %q", reason)
	}
}
