//go:build unix

package commentaudit

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func refuseFileGrowthF2(t *testing.T) {
	t.Helper()
	var old syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &old); err != nil {
		t.Skipf("no RLIMIT_FSIZE: %v", err)
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: 0, Max: old.Max}); err != nil {
		t.Skipf("cannot lower RLIMIT_FSIZE: %v", err)
	}
	t.Cleanup(func() {
		if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &old); err != nil {
			t.Errorf("restore RLIMIT_FSIZE: %v", err)
		}
	})
}

func TestAppendFile_ReportsAFailedWriteAndLeavesNoDataF2(t *testing.T) {
	name := filepath.Join(t.TempDir(), "page.md")
	refuseFileGrowthF2(t)

	err := appendFile(name, strings.Repeat("history\n", 64))

	info, statErr := os.Stat(name)
	if err == nil || statErr != nil || info.Size() != 0 {
		t.Errorf("appendFile = %v, stat (%v, %v); want a write error and an empty page", err, info, statErr)
	}
}

func TestAppendFile_ReportsAnUnopenablePageF2(t *testing.T) {
	t.Parallel()

	err := appendFile(filepath.Join(t.TempDir(), "no", "such", "page.md"), "x")

	if !os.IsNotExist(err) {
		t.Errorf("appendFile = %v, want a not-exist error", err)
	}
}

func TestReplaceFile_ReportsAFailedWriteRemovesTheTempAndKeepsTheTargetF2(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "a.go")
	if err := os.WriteFile(target, []byte("package a\n// c\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	refuseFileGrowthF2(t)

	err := replaceFile(target, []byte(strings.Repeat("package a\n", 64)))

	entries, readErr := os.ReadDir(dir)
	if err == nil || !strings.Contains(err.Error(), target) || readErr != nil || len(entries) != 1 || entries[0].Name() != "a.go" {
		t.Errorf("replaceFile = %v, dir %v (%v); want a write error naming the target and no temp file left", err, entries, readErr)
	}
	if got, _ := os.ReadFile(target); string(got) != "package a\n// c\n" {
		t.Errorf("the target changed to %q", got)
	}
}
