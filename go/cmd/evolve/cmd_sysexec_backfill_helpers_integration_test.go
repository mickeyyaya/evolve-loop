//go:build integration

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func bfStub(t *testing.T, name, body string) (record string) {
	t.Helper()
	bin := t.TempDir()
	record = filepath.Join(bin, "calls")
	fakeclitest.Install(t, filepath.Join(bin, name), "#!/bin/sh\necho \"$PWD|$*\" >> '"+record+"'\n"+body+"\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return record
}

func bfGitOnlyPath(t *testing.T) string {
	t.Helper()
	real, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}
	bin := t.TempDir()
	if err := os.Symlink(real, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	return bin
}

func bfCalls(t *testing.T, record string) string {
	t.Helper()
	data, err := os.ReadFile(record)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func bfRealDir(t *testing.T, dir string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return real
}

func bfCommittedRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	if err := os.WriteFile(filepath.Join(repo.Dir, "README"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	return repo
}

func bfNonRepo(t *testing.T) string {
	t.Helper()
	dir := bfRealDir(t, t.TempDir())
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	return dir
}

func bfChdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(prev); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	})
}

func bfSwap[T any](t *testing.T, seam *T, fake T) {
	t.Helper()
	saved := *seam
	*seam = fake
	t.Cleanup(func() { *seam = saved })
}

func bfLines(s string) []string {
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}
