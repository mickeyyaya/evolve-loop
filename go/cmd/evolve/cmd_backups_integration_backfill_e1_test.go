//go:build integration

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestRunBackups_ARealRepoClassifiesBundlesAndPatchesAgainstOriginMain(t *testing.T) {
	origin := gittest.Bare(t)
	repo := gittest.Clone(t, origin.Dir)
	if err := os.WriteFile(filepath.Join(repo.Dir, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("add", "a.txt")
	repo.Git("commit", "-q", "-m", "landed")
	repo.Git("push", "-q", "origin", "HEAD:main")
	repo.Git("fetch", "-q", "origin")
	landed := repo.Git("rev-parse", "HEAD")
	repo.Git("checkout", "-q", "-b", "lost")
	if err := os.WriteFile(filepath.Join(repo.Dir, "a.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("commit", "-q", "-am", "never pushed")
	lost := repo.Git("rev-parse", "HEAD")

	backups := t.TempDir()
	repo.Git("bundle", "create", filepath.Join(backups, "main.bundle"), "main")
	repo.Git("format-patch", "-q", "-1", landed, "-o", backups)
	safeOut, safeErr, safeCode := runBackupsIn(t, repo.Dir, backups)
	if safeCode != 0 {
		t.Fatalf("code = %d, want 0 for a landed head and an applied patch (stdout=%q stderr=%q)", safeCode, safeOut, safeErr)
	}
	if !strings.Contains(safeOut, landed) || !strings.Contains(safeOut, "IN_MAIN") || !strings.Contains(safeOut, "APPLIED") {
		t.Errorf("stdout = %q, want the landed head IN_MAIN and the patch APPLIED", safeOut)
	}

	repo.Git("bundle", "create", filepath.Join(backups, "lost.bundle"), "lost")
	repo.Git("format-patch", "-q", "-1", lost, "-o", backups, "--start-number", "2")
	out, errOut, code := runBackupsIn(t, repo.Dir, backups)
	if code != 1 {
		t.Fatalf("code = %d, want 1 for an unpushed head (stdout=%q stderr=%q)", code, out, errOut)
	}
	if !strings.Contains(errOut, "refusing: head "+lost+" (lost.bundle refs/heads/lost) exists only in the backup") {
		t.Errorf("stderr = %q, want the lost head refused", errOut)
	}
	if !strings.Contains(errOut, "is not applied to origin/main") {
		t.Errorf("stderr = %q, want the unapplied patch refused", errOut)
	}
}

func runBackupsIn(t *testing.T, root, dir string) (string, string, int) {
	t.Helper()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var out, errb bytes.Buffer
	code := runBackups([]string{"verify", "--dir", dir}, nil, &out, &errb)
	return out.String(), errb.String(), code
}
