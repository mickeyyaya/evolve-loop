package commentaudit

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

var _ Git = ExecGit{}

func chdir(t *testing.T, dir string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	})
}

func writeRepoFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExecGit_VerifiesFromASubdirectory(t *testing.T) {
	repo := gittest.Fixture(t)
	file := filepath.Join(repo.Dir, "go", "p", "a.go")
	writeRepoFile(t, file, "package p\n\n// history\nfunc a() {}\n")
	repo.Git("add", ".")
	repo.Git("commit", "-q", "-m", "base")
	writeRepoFile(t, file, "package p\n\nfunc a() {}\n")
	chdir(t, filepath.Join(repo.Dir, "go"))

	var out, errOut bytes.Buffer
	if code := Main([]string{"verify", "-base", "HEAD"}, &out, &errOut, ExecGit{}); code != 0 {
		t.Fatalf("a comment-only edit verified from go/ must pass (exit %d):\n%s%s", code, out.String(), errOut.String())
	}
}

func TestExecGit_ARenameNamesItsSourceToo(t *testing.T) {
	repo := gittest.Fixture(t)
	writeRepoFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), "package p\n\n// A runs.\nfunc A() {}\n")
	repo.Git("add", ".")
	repo.Git("commit", "-q", "-m", "base")
	repo.Git("mv", "go/p/a.go", "go/p/b.go")
	chdir(t, repo.Dir)

	var out, errOut bytes.Buffer
	code := Main([]string{"verify", "-base", "HEAD"}, &out, &errOut, ExecGit{})
	if code != 1 || !bytes.Contains(out.Bytes(), []byte("go/p/a.go: file deleted")) || !bytes.Contains(out.Bytes(), []byte("go/p/b.go: file added")) {
		t.Fatalf("a rename is a delete plus an add, and verify names both (exit %d):\n%s%s", code, out.String(), errOut.String())
	}
}

func TestExecGit_RootAndShowReportTheRepository(t *testing.T) {
	repo := gittest.Fixture(t)
	writeRepoFile(t, filepath.Join(repo.Dir, "a.go"), "package p\n")
	repo.Git("add", ".")
	repo.Git("commit", "-q", "-m", "base")
	chdir(t, repo.Dir)

	root, err := ExecGit{}.Root()
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := filepath.EvalSymlinks(root); got != want {
		t.Errorf("Root() = %q, want %q", root, want)
	}
	body, err := ExecGit{}.Show("HEAD", "a.go")
	if err != nil || string(body) != "package p\n" {
		t.Errorf("Show(HEAD, a.go) = %q, %v", body, err)
	}
	if _, err := (ExecGit{}).ChangedFiles("no-such-ref"); err == nil {
		t.Error("ChangedFiles on an unknown base must fail with git's error")
	}
}
