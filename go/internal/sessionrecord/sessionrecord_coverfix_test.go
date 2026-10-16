package sessionrecord

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadAll_OpenFailureOtherThanMissingIsReported(t *testing.T) {
	t.Parallel()
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAll(filepath.Join(blocker, "reg")); err == nil {
		t.Fatal("want open error")
	}
}

func TestReadAll_ScanFailureIsReported(t *testing.T) {
	t.Parallel()
	if _, err := ReadAll(t.TempDir()); err == nil {
		t.Fatal("want scan error reading a directory")
	}
}

func TestReadAllResolving_FailureOfEitherFileIsReported(t *testing.T) {
	t.Parallel()
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAllResolving(filepath.Join(blocker, "reg")); err == nil {
		t.Fatal("want live read error")
	}
	dir := t.TempDir()
	reg := filepath.Join(dir, "reg")
	if err := os.Mkdir(reg+ReapedSuffix, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAllResolving(reg); err == nil {
		t.Fatal("want tombstone read error")
	}
}
