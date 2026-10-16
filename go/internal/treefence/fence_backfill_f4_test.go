package treefence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func shimGitF4(t *testing.T, failVerb string) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}
	binDir := t.TempDir()
	script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = %q ]; then echo \"shim: forced failure of %s\" >&2; exit 3; fi\nexec %q \"$@\"\n", failVerb, failVerb, realGit)
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func lockedDirF4(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	probe := filepath.Join(dir, ".probe")
	if f, err := os.Create(probe); err == nil {
		f.Close()
		_ = os.Remove(probe)
		t.Skip("directory permissions are not enforced for this user")
	}
}

func TestSnapshotDiffFailure_IsReturnedByRestoreDifferingAndVerify(t *testing.T) {
	dir := initRepo(t)
	ctx := context.Background()
	snap, err := Take(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	other := snap
	other.Tree = "0000000000000000000000000000000000000001"
	write(t, dir, "src/mat.go", "package src // changed after the snapshot\n")
	shimGitF4(t, "diff-tree")

	t.Run("Restore", func(t *testing.T) {
		res, err := snap.Restore(ctx)
		if err == nil || !strings.Contains(err.Error(), "git diff-tree") || len(res.Restored) != 0 {
			t.Fatalf("res=%+v err=%v", res, err)
		}
		if got := read(t, dir, "src/mat.go"); !strings.Contains(got, "changed after the snapshot") {
			t.Errorf("a failed diff must not touch the worktree: %q", got)
		}
	})
	t.Run("Differing", func(t *testing.T) {
		paths, err := snap.Differing(ctx, other)
		if err == nil || !strings.Contains(err.Error(), "git diff-tree") || paths != nil {
			t.Fatalf("paths=%v err=%v", paths, err)
		}
	})
	t.Run("verify", func(t *testing.T) {
		if err := snap.verify(ctx); err == nil || !strings.Contains(err.Error(), "git diff-tree") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestWriteTree_WriteTreeCommandFailureIsReturned(t *testing.T) {
	dir := initRepo(t)
	shimGitF4(t, "write-tree")

	snap, err := Take(context.Background(), dir)

	if err == nil || !strings.Contains(err.Error(), "[write-tree]") || snap.Tree != "" {
		t.Fatalf("snap=%+v err=%v", snap, err)
	}
}

func TestIndexCopyTree_UnusableTempDirIsReported(t *testing.T) {
	dir := initRepo(t)
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "absent"))

	tree, err := indexCopyTree(context.Background(), dir, allFiles, nil)

	if err == nil || !strings.Contains(err.Error(), "treefence: snapshot tree: gitexec: scratch index") || tree != "" {
		t.Fatalf("tree=%q err=%v", tree, err)
	}
}

func TestRestore_EntriesThatCannotBeUndoneAreReportedByNameWhileTheRestIsStillRestored(t *testing.T) {
	dir := initRepo(t)
	ctx := context.Background()
	write(t, dir, "locked/orig.txt", "original\n")
	write(t, dir, "free.txt", "free original\n")
	snap, err := Take(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	write(t, dir, "locked/orig.txt", "mutated\n")
	write(t, dir, "locked/added.txt", "added by the phase\n")
	write(t, dir, "free.txt", "free mutated\n")
	lockedDirF4(t, filepath.Join(dir, "locked"))

	res, err := snap.Restore(ctx)

	if err == nil {
		t.Fatal("want the joined failures")
	}
	for _, want := range []string{"remove locked/added.txt", "clear locked/orig.txt"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q: %v", want, err)
		}
	}
	if got := read(t, dir, "free.txt"); got != "free original\n" {
		t.Errorf("an unrelated path must still be restored: %q", got)
	}
	if len(res.Restored) == 0 {
		t.Errorf("the attempted paths are still reported: %+v", res)
	}
}

func TestDiagnostics_LongRestoredListIsTruncatedWithTheRemainderCounted(t *testing.T) {
	paths := make([]string, listMax+3)
	for i := range paths {
		paths[i] = fmt.Sprintf("p%02d.go", i)
	}

	diags := Outcome{Restored: paths}.Diagnostics("audit")

	if len(diags) != 1 {
		t.Fatalf("diagnostics = %+v", diags)
	}
	msg := diags[0].Message
	if !strings.Contains(msg, fmt.Sprintf("wrote %d path(s)", listMax+3)) || !strings.Contains(msg, "p11.go … +3 more") || strings.Contains(msg, "p12.go") {
		t.Errorf("message = %q", msg)
	}

	exact := Outcome{Restored: paths[:listMax]}.Diagnostics("audit")
	if len(exact) != 1 || strings.Contains(exact[0].Message, "more") || !strings.Contains(exact[0].Message, "p11.go") {
		t.Errorf("a list exactly at the cap is shown whole: %+v", exact)
	}

	failed := Outcome{RestoreErr: errors.New("disk")}.Diagnostics("audit")
	if len(failed) != 1 || !strings.Contains(failed[0].Message, "restore failed for read-only phase audit (disk)") {
		t.Errorf("restore failure diagnostic = %+v", failed)
	}
}
