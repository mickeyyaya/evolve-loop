package treefence

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

func cleanRepo(t *testing.T) string {
	t.Helper()
	dir := initRepo(t)
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "clean")
	return dir
}

func fencedRepo(t *testing.T) (string, *Fence) {
	t.Helper()
	dir := cleanRepo(t)
	fence := Begin(context.Background(), dir, true)
	if err := fence.TakeErr(); err != nil {
		t.Fatal(err)
	}
	return dir, fence
}

func TestEnd_StagedPathsOutsideTheListLeaveIndexAndWorkingCopyInStep(t *testing.T) {
	dir, fence := fencedRepo(t)
	write(t, dir, "src/new.go", "package src\n")
	write(t, dir, "src/mat.go", "package src // staged edit\n")
	runGit(t, dir, "add", "--", "src/new.go", "src/mat.go")
	write(t, dir, "src/mat.go", "package src // edited again\n")

	out := fence.End(context.Background())

	if !out.Verified || out.RestoreErr != nil {
		t.Fatalf("out=%+v", out)
	}
	if status := runGit(t, dir, "status", "--porcelain"); strings.TrimSpace(status) != "" {
		t.Errorf("status after revert:\n%s", status)
	}
}

func TestEnd_AnEditUnstagedAtDispatchIsNotStagedByTheRevert(t *testing.T) {
	dir := cleanRepo(t)
	write(t, dir, "src/mat.go", "package src // unstaged at dispatch\n")
	fence := Begin(context.Background(), dir, true)
	write(t, dir, "src/mat.go", "package src // seat\n")
	runGit(t, dir, "add", "--", "src/mat.go")

	out := fence.End(context.Background())

	if !out.Verified {
		t.Fatalf("out=%+v", out)
	}
	if status := strings.TrimRight(runGit(t, dir, "status", "--porcelain"), "\n"); status != " M src/mat.go" {
		t.Errorf("status = %q", status)
	}
}

func TestEnd_ALockedIndexFailsTheRestoreLoudly(t *testing.T) {
	dir, fence := fencedRepo(t)
	write(t, dir, "src/new.go", "package src\n")
	runGit(t, dir, "add", "--", "src/new.go")
	if err := os.WriteFile(filepath.Join(dir, ".git", "index.lock"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	out := fence.End(context.Background())

	if out.Verified || out.RestoreErr == nil || !strings.Contains(out.RestoreErr.Error(), "git update-index") {
		t.Fatalf("out=%+v", out)
	}
}

func fakeGitFor(t *testing.T, lsFilesBody string) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}
	binDir := t.TempDir()
	script := "#!/bin/sh\nif [ \"$1\" = ls-files ]; then\n" + lsFilesBody + "\nfi\nexec " + realGit + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestBegin_AnIndexListingThatCannotBeReadIsATakeError(t *testing.T) {
	dir := cleanRepo(t)
	fakeGitFor(t, "echo forced >&2; exit 3")

	fence := Begin(context.Background(), dir, true)

	if err := fence.TakeErr(); err == nil || !strings.Contains(err.Error(), "ls-files") {
		t.Fatalf("TakeErr = %v", err)
	}
}

func TestBegin_AMalformedIndexListingIsATakeError(t *testing.T) {
	dir := cleanRepo(t)
	fakeGitFor(t, "printf 'garbage\\0'; exit 0")

	fence := Begin(context.Background(), dir, true)

	if err := fence.TakeErr(); err == nil || !strings.Contains(err.Error(), "malformed index entry") {
		t.Fatalf("TakeErr = %v", err)
	}
}

func TestEnd_AnIndexListingThatFailsAfterDispatchIsLoud(t *testing.T) {
	dir, fence := fencedRepo(t)
	fakeGitFor(t, "echo forced >&2; exit 3")

	out := fence.End(context.Background())

	if out.Verified || out.RestoreErr == nil || !strings.Contains(out.RestoreErr.Error(), "ls-files") {
		t.Fatalf("out=%+v dir=%s", out, dir)
	}
}

func TestEnd_ADriftLeftAfterTheResetFailsVerification(t *testing.T) {
	dir := cleanRepo(t)
	snap, err := Take(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	write(t, dir, "src/new.go", "package src\n")
	runGit(t, dir, "add", "--", "src/new.go")
	if err := os.Remove(filepath.Join(dir, "src/new.go")); err != nil {
		t.Fatal(err)
	}

	err = snap.verify(context.Background())

	if err == nil || !strings.Contains(err.Error(), "restored index differs") || !strings.Contains(err.Error(), "src/new.go") {
		t.Fatalf("verify = %v", err)
	}
}

func TestVerify_AnIndexListingThatFailsIsReturned(t *testing.T) {
	dir := cleanRepo(t)
	snap, err := Take(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	fakeGitFor(t, "echo forced >&2; exit 3")

	if err := snap.verify(context.Background()); err == nil || !strings.Contains(err.Error(), "ls-files") {
		t.Fatalf("verify = %v", err)
	}
}

func TestMergeSorted_JoinsAndDeduplicates(t *testing.T) {
	got := mergeSorted([]string{"b", "a"}, []string{"b", "c"})
	if strings.Join(got, ",") != "a,b,c" {
		t.Fatalf("got %v", got)
	}
}

func TestRetract_ARetractionReadingAFailingIndexIsLoud(t *testing.T) {
	dir := cleanRepo(t)
	snap, err := Take(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	fakeGitFor(t, "echo forced >&2; exit 3")

	if _, err := snap.Retract(context.Background(), t.TempDir()); err == nil {
		t.Fatal("want an error")
	}
}
