package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
)

func TestExecGit_VerifiesFromASubdirectory(t *testing.T) {
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	file := filepath.Join(repo, "go", "p", "a.go")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("package p\n\n// history\nfunc a() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("init", "-q")
	run("-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false", "add", ".")
	run("-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "base")
	if err := os.WriteFile(file, []byte("package p\n\nfunc a() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	if err := os.Chdir(filepath.Join(repo, "go")); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	var out, errOut bytes.Buffer
	if code := commentaudit.Main([]string{"verify", "-base", "HEAD"}, &out, &errOut, execGit{}); code != 0 {
		t.Fatalf("a comment-only edit verified from go/ must pass (exit %d):\n%s%s", code, out.String(), errOut.String())
	}
}

func TestExecGit_ARenameNamesItsSourceToo(t *testing.T) {
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	file := filepath.Join(repo, "go", "p", "a.go")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("package p\n\n// A runs.\nfunc A() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("init", "-q")
	run("add", ".")
	run("commit", "-q", "-m", "base")
	run("mv", "go/p/a.go", "go/p/b.go")
	wd, _ := os.Getwd()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	var out, errOut bytes.Buffer
	code := commentaudit.Main([]string{"verify", "-base", "HEAD"}, &out, &errOut, execGit{})
	if code != 1 || !bytes.Contains(out.Bytes(), []byte("go/p/a.go: file deleted")) || !bytes.Contains(out.Bytes(), []byte("go/p/b.go: file added")) {
		t.Fatalf("a rename is a delete plus an add, and verify names both (exit %d):\n%s%s", code, out.String(), errOut.String())
	}
}
