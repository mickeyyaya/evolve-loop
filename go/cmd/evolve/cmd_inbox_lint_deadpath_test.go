package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const lintItemTemplate = `{"id":"ID","kind":"bug","weight":0.9,"priority_class":"correctness","title":"t","summary":"s","files":[FILES]}`

func lintItemJSON(id string, files ...string) string {
	quoted := make([]string, len(files))
	for i, f := range files {
		quoted[i] = `"` + f + `"`
	}
	return strings.NewReplacer("ID", id, "FILES", strings.Join(quoted, ",")).Replace(lintItemTemplate)
}

func lintRepoWithTrackedFile(t *testing.T, tracked string) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	contractUnitWrite(t, repo.Dir, tracked, "package live\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "seed")
	return repo
}

func TestLint_FlagsAPathThatIsGoneOnMain(t *testing.T) {
	t.Parallel()
	repo := lintRepoWithTrackedFile(t, "go/internal/live/live.go")
	contractUnitWrite(t, repo.Dir, ".evolve/inbox/cites-gone.json", lintItemJSON("cites-gone", "go/internal/live/live.go", "go/internal/gone/gone.go"))
	var stdout, stderr bytes.Buffer

	rc := runInboxLint(repo.Dir, nil, &stdout, &stderr)

	if rc != 0 {
		t.Errorf("rc = %d, want 0: a dead path is a warning and never fails the lint (stdout %q, stderr %q)", rc, stdout.String(), stderr.String())
	}
	if want := "cites-gone: go/internal/gone/gone.go is gone on main"; !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr = %q, want it to contain %q", stderr.String(), want)
	}
	if strings.Contains(stderr.String(), "live.go is gone") {
		t.Errorf("stderr = %q, a path tracked on main must not be flagged", stderr.String())
	}
}

func TestLint_JudgesAPathByTheTrackedTreeNotTheDisk(t *testing.T) {
	t.Parallel()
	repo := lintRepoWithTrackedFile(t, "go/internal/live/live.go")
	contractUnitWrite(t, repo.Dir, "go/internal/loose.go", "package internal\n")
	contractUnitWrite(t, repo.Dir, ".evolve/inbox/cites-mixed.json", lintItemJSON("cites-mixed", "go/internal/live", "go/internal/loose.go"))
	var stdout, stderr bytes.Buffer

	rc := runInboxLint(repo.Dir, nil, &stdout, &stderr)

	if rc != 0 {
		t.Errorf("rc = %d, want 0 (stderr %q)", rc, stderr.String())
	}
	if want := "cites-mixed: go/internal/loose.go is gone on main"; !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr = %q, want %q: an untracked file is not on main", stderr.String(), want)
	}
	if strings.Contains(stderr.String(), "go/internal/live is gone") {
		t.Errorf("stderr = %q, a directory that holds tracked files is on main", stderr.String())
	}
}

func TestLint_SkipsTheDeadPathCheckWhenTheTrackedTreeIsUnreadable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	contractUnitWrite(t, root, ".evolve/inbox/cites-gone.json", lintItemJSON("cites-gone", "go/internal/gone/gone.go"))
	var stdout, stderr bytes.Buffer

	rc := runInboxLint(root, nil, &stdout, &stderr)

	if rc != 0 || strings.Contains(stderr.String(), "is gone on main") {
		t.Errorf("rc = %d, stderr %q, want exit 0 and no dead-path verdict from a tree that could not be read", rc, stderr.String())
	}
}

func TestLint_ADeadPathDoesNotMaskAnUndeclaredSurface(t *testing.T) {
	t.Parallel()
	repo := lintRepoWithTrackedFile(t, "go/internal/live/live.go")
	contractUnitWrite(t, repo.Dir, ".evolve/inbox/cites-gone.json", lintItemJSON("cites-gone", "go/internal/gone/gone.go"))
	contractUnitWrite(t, repo.Dir, ".evolve/inbox/plain.json", `{"id":"plain","kind":"bug","weight":0.8,"priority_class":"correctness","title":"the queue renders slowly","summary":"s"}`)
	var stdout, stderr bytes.Buffer

	rc := runInboxLint(repo.Dir, nil, &stdout, &stderr)

	if rc != cliExitFailed || !strings.Contains(stdout.String(), "plain: undeclared surface") {
		t.Errorf("rc = %d, stdout %q, want exit 1 for the undeclared item", rc, stdout.String())
	}
	if !strings.Contains(stderr.String(), "cites-gone: go/internal/gone/gone.go is gone on main") {
		t.Errorf("stderr = %q, want the dead-path warning beside the undeclared failure", stderr.String())
	}
}
