package reviewcheck_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/reviewcheck"
)

func writeFile(t *testing.T, repo *gittest.Repo, rel, body string) {
	t.Helper()
	path := filepath.Join(repo.Dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commit(repo *gittest.Repo, message string) string {
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "--allow-empty", "-m", message)
	return repo.Git("rev-parse", "HEAD")
}

func fixture(t *testing.T, files map[string]string) (*gittest.Repo, string) {
	t.Helper()
	repo := gittest.Fixture(t)
	for rel, body := range files {
		writeFile(t, repo, rel, body)
	}
	return repo, commit(repo, "base")
}

func TestInboxIntegrityAt_RefusesTheDeletionAndAcceptsAMoveOutOfTheInbox(t *testing.T) {
	t.Parallel()
	repo, base := fixture(t, map[string]string{deletedItem: `{"id":"x-item"}`, ".evolve/inbox/keep.json": `{"id":"keep"}`})
	repo.Git("rm", "-q", deletedItem)
	commit(repo, "inbox: tidy")
	got, err := reviewcheck.InboxIntegrityAt(context.Background(), gitexec.Default(repo.Dir), reviewcheck.Range{Base: base, Head: "HEAD"})
	if err != nil || len(got) != 1 || got[0].Subject != "x-item" {
		t.Fatalf("deletion: refusals = %+v, err = %v, want one refusal of x-item", got, err)
	}
	repo.Git("reset", "-q", "--hard", base)
	repo.Git("config", "diff.renames", "false")
	writeFile(t, repo, "docs/archive/.keep", "")
	repo.Git("add", "-A")
	repo.Git("mv", deletedItem, "docs/archive/x-item.json")
	commit(repo, "archive x")
	got, err = reviewcheck.InboxIntegrityAt(context.Background(), gitexec.Default(repo.Dir), reviewcheck.Range{Base: base, Head: "HEAD"})
	if err != nil || len(got) != 0 {
		t.Errorf("move: refusals = %+v, err = %v, want none", got, err)
	}
}

func TestInboxIntegrityAt_RefusesTheDeletionOfAnItemWithANonASCIIName(t *testing.T) {
	t.Parallel()
	item := ".evolve/inbox/2026-11-03T01-00-00Z-caf\u00e9.json"
	repo, base := fixture(t, map[string]string{item: `{"id":"cafe"}`})
	repo.Git("rm", "-q", item)
	commit(repo, "inbox: tidy")
	got, err := reviewcheck.InboxIntegrityAt(context.Background(), gitexec.Default(repo.Dir), reviewcheck.Range{Base: base, Head: "HEAD"})
	if err != nil || len(got) != 1 || got[0].Subject != "cafe" {
		t.Errorf("refusals = %+v, err = %v, want one refusal of cafe", got, err)
	}
}

func TestReviewChecksAt_ABaseThatIsNotACommitIsAnErrorAndNeverAGitOption(t *testing.T) {
	t.Parallel()
	repo, _ := fixture(t, map[string]string{"a.txt": "a\n"})
	written := filepath.Join(repo.Dir, "written")
	for _, base := range []string{"no-such-ref", "--output=" + written} {
		_, err := reviewcheck.InboxIntegrityAt(context.Background(), gitexec.Default(repo.Dir), reviewcheck.Range{Base: base, Head: "HEAD"})
		if err == nil || !strings.Contains(err.Error(), "is not a commit") {
			t.Errorf("inbox base %q: err = %v, want a not-a-commit error", base, err)
		}
		_, err = reviewcheck.PermissionWideningAt(context.Background(), gitexec.Default(repo.Dir), reviewcheck.Range{Base: base, Head: "HEAD"})
		if err == nil || !strings.Contains(err.Error(), "is not a commit") {
			t.Errorf("permissions base %q: err = %v, want a not-a-commit error", base, err)
		}
	}
	if _, err := os.Stat(written); err == nil {
		t.Error("a base that starts with a dash reached git as an option and wrote a file")
	}
}

func TestPermissionWideningAt_ReadsProfilesAndManifestsThroughTheirTypes(t *testing.T) {
	t.Parallel()
	repo, base := fixture(t, map[string]string{
		".evolve/profiles/p.json":     `{"allowed_tools":["Read"],"sandbox":{"write_subpaths":["a/"]}}`,
		".evolve/profiles/gone.json":  `{"allowed_tools":["Read"]}`,
		".evolve/profiles/move.json":  `{"allowed_tools":["Read","Grep","Glob","Bash"]}`,
		".evolve/profiles/empty.json": `{}`,
	})
	repo.Git("config", "diff.renames", "false")
	writeFile(t, repo, ".evolve/profiles/p.json", `{"allowed_tools":["Read","Edit"],"sandbox":{"write_subpaths":["a/","b/"]}}`)
	writeFile(t, repo, "go/internal/bridge/manifests/new.json", `{"env_passthrough":["HOME"]}`)
	writeFile(t, repo, ".evolve/profiles/README.md", "notes\n")
	writeFile(t, repo, ".evolve/profiles/empty.json", "")
	repo.Git("rm", "-q", ".evolve/profiles/gone.json")
	repo.Git("mv", ".evolve/profiles/move.json", ".evolve/profiles/moved.json")
	writeFile(t, repo, "agents/uses-edit.md", "Edit\n")
	commit(repo, "feat: widen\n\nWidening-consumer: .evolve/profiles/p.json allowed_tools agents/uses-edit.md Edit")
	got, err := reviewcheck.PermissionWideningAt(context.Background(), gitexec.Default(repo.Dir), reviewcheck.Range{Base: base, Head: "HEAD"})
	want := []string{
		".evolve/profiles/p.json sandbox.write_subpaths b/",
		"go/internal/bridge/manifests/new.json env_passthrough HOME",
	}
	if err != nil || !slices.Equal(subjects(got), want) {
		t.Errorf("subjects = %q, err = %v, want %q", subjects(got), err, want)
	}
}

func TestPermissionWideningAt_AnUnparseableFileIsAnErrorThatNamesIt(t *testing.T) {
	t.Parallel()
	repo, base := fixture(t, map[string]string{".evolve/profiles/p.json": `{}`})
	writeFile(t, repo, ".evolve/profiles/p.json", `{`)
	commit(repo, "break it")
	if _, err := reviewcheck.PermissionWideningAt(context.Background(), gitexec.Default(repo.Dir), reviewcheck.Range{Base: base, Head: "HEAD"}); err == nil || !strings.Contains(err.Error(), ".evolve/profiles/p.json") {
		t.Errorf("err = %v, want an error that names the profile", err)
	}
}
