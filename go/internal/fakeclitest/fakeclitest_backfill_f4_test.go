package fakeclitest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type recordingTBF4 struct {
	testing.TB
	logs   []string
	fatals []string
}

type fatalSentinelF4 struct{}

func (r *recordingTBF4) Helper() {}

func (r *recordingTBF4) Logf(format string, args ...any) {
	r.logs = append(r.logs, strings.TrimSpace(format))
}

func (r *recordingTBF4) Fatalf(format string, args ...any) {
	r.fatals = append(r.fatals, format)
	panic(fatalSentinelF4{})
}

func installF4(t *testing.T, path, body string) *recordingTBF4 {
	t.Helper()
	rec := &recordingTBF4{TB: t}
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(fatalSentinelF4); !ok {
					panic(r)
				}
			}
		}()
		Install(rec, path, body)
	}()
	return rec
}

func TestInstall_FailuresAreFatalAndTheCopyFallbackIsLogged(t *testing.T) {
	t.Run("the test binary cannot be located", func(t *testing.T) {
		orig := testBinaryPath
		t.Cleanup(func() { testBinaryPath = orig })
		testBinaryPath = func() (string, error) { return "", errors.New("no executable") }
		rec := installF4(t, filepath.Join(t.TempDir(), "fake"), "#!/bin/sh\n")
		if len(rec.fatals) != 1 || !strings.Contains(rec.fatals[0], "locate the test binary") {
			t.Fatalf("fatals = %v", rec.fatals)
		}
	})
	t.Run("the install itself fails", func(t *testing.T) {
		rec := installF4(t, filepath.Join(t.TempDir(), "no", "such", "dir", "fake"), "#!/bin/sh\n")
		if len(rec.fatals) != 1 || strings.Contains(rec.fatals[0], "locate") {
			t.Fatalf("fatals = %v", rec.fatals)
		}
	})
	t.Run("a failed hard link falls back to a logged copy", func(t *testing.T) {
		orig := linkTestBinary
		t.Cleanup(func() { linkTestBinary = orig })
		linkTestBinary = func(string, string) error { return errors.New("cross-device link") }
		path := filepath.Join(t.TempDir(), "fake")
		rec := installF4(t, path, "#!/bin/sh\n")
		if len(rec.fatals) != 0 {
			t.Fatalf("fatals = %v", rec.fatals)
		}
		if len(rec.logs) != 1 || !strings.Contains(rec.logs[0], "is a copy of the test binary") {
			t.Fatalf("logs = %v", rec.logs)
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm()&0o200 != 0 || info.Mode().Perm()&0o100 == 0 {
			t.Fatalf("the copy must be executable and read-only: %v %v", info, err)
		}
	})
}

func TestInstall_AnInstalledFakeThatCannotBeGuardedIsReported(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fake")
	linkErr, err := install("ignored-self", path, "#!/bin/sh\n", func(string, string) error { return nil })

	if linkErr != nil || err == nil || !strings.Contains(err.Error(), "guard "+path+" against writes") {
		t.Fatalf("linkErr=%v err=%v", linkErr, err)
	}
}

func TestCopyExecutable_ReadFailureClosesTheDestinationAndReportsTheError(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "copy")

	err := copyExecutable(t.TempDir(), dst)

	if err == nil {
		t.Fatal("copying a directory must fail")
	}
	if _, statErr := os.Stat(dst); statErr != nil {
		t.Errorf("the destination was created before the failed read: %v", statErr)
	}
	if err := copyExecutable(filepath.Join(dir, "absent"), filepath.Join(dir, "other")); err == nil {
		t.Error("a missing source must fail")
	}
	if err := copyExecutable(os.Args[0], dst); err == nil {
		t.Error("an existing destination must not be overwritten")
	}
}
