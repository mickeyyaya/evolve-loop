package evidencex_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evidencex"
)

func sha256Hex(src []byte) string {
	sum := sha256.Sum256(src)
	return hex.EncodeToString(sum[:])
}

func listRegularFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.Type().IsRegular() {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return out
}

func TestLocalArchive_RoundTripAndPersistence(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "nested", "archive")
	writer, err := evidencex.NewLocalArchive(dir, 4096)
	if err != nil {
		t.Fatalf("NewLocalArchive: %v", err)
	}
	cases := [][]byte{
		[]byte("line 1\nERROR failure\nline 3\n"),
		{},
		{0x00, 0xff, 0x1b, '\n', 0x00},
	}
	for _, src := range cases {
		key := sha256Hex(src)
		if err := writer.Put(key, src); err != nil {
			t.Fatalf("Put(%s): %v", key, err)
		}
	}
	reader, err := evidencex.NewLocalArchive(dir, 4096)
	if err != nil {
		t.Fatalf("reopen NewLocalArchive: %v", err)
	}
	for _, src := range cases {
		got, err := reader.Get(sha256Hex(src))
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if !bytes.Equal(got, src) {
			t.Errorf("Get = %q, want %q", got, src)
		}
	}
}

func TestLocalArchive_SizeCapCountsRawBytes(t *testing.T) {
	t.Parallel()
	const capBytes = 1024
	dir := filepath.Join(t.TempDir(), "archive")
	archive, err := evidencex.NewLocalArchive(dir, capBytes)
	if err != nil {
		t.Fatalf("NewLocalArchive: %v", err)
	}
	atCap := bytes.Repeat([]byte("a"), capBytes)
	if err := archive.Put(sha256Hex(atCap), atCap); err != nil {
		t.Fatalf("Put at exact cap: %v", err)
	}
	overCap := bytes.Repeat([]byte("a"), capBytes+1)
	overKey := sha256Hex(overCap)
	err = archive.Put(overKey, overCap)
	if !errors.Is(err, evidencex.ErrArchiveTooLarge) {
		t.Fatalf("Put over cap err = %v, want ErrArchiveTooLarge", err)
	}
	if got, getErr := archive.Get(overKey); !errors.Is(getErr, evidencex.ErrArchiveNotFound) || got != nil {
		t.Errorf("Get after refused Put: got=%v err=%v", got, getErr)
	}
	if files := listRegularFiles(t, dir); len(files) != 1 {
		t.Errorf("expected only 1 file after refused Put, got %v", files)
	}
}

func TestLocalArchive_DuplicatePutIdempotent(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "archive")
	archive, err := evidencex.NewLocalArchive(dir, 4096)
	if err != nil {
		t.Fatalf("NewLocalArchive: %v", err)
	}
	src := []byte("duplicate source payload\n")
	key := sha256Hex(src)
	if err := archive.Put(key, src); err != nil {
		t.Fatalf("first Put: %v", err)
	}
	before := listRegularFiles(t, dir)
	if err := archive.Put(key, src); err != nil {
		t.Fatalf("second Put: %v", err)
	}
	after := listRegularFiles(t, dir)
	if len(before) != 1 || len(after) != 1 {
		t.Errorf("files before=%v after=%v", before, after)
	}
	got, err := archive.Get(key)
	if err != nil || !bytes.Equal(got, src) {
		t.Errorf("Get = %q err=%v", got, err)
	}
}

func TestLocalArchive_ValidationAndErrors(t *testing.T) {
	t.Parallel()
	for _, badDir := range []string{"", "   "} {
		if _, err := evidencex.NewLocalArchive(badDir, 100); err == nil {
			t.Errorf("NewLocalArchive(%q, 100) succeeded, want error", badDir)
		}
	}
	for _, badCap := range []int64{0, -1, -100} {
		if _, err := evidencex.NewLocalArchive(t.TempDir(), badCap); err == nil {
			t.Errorf("NewLocalArchive(cap=%d) succeeded, want error", badCap)
		}
	}
	parent := t.TempDir()
	dir := filepath.Join(parent, "archive")
	archive, err := evidencex.NewLocalArchive(dir, 4096)
	if err != nil {
		t.Fatalf("NewLocalArchive: %v", err)
	}
	badKeys := []string{
		"",
		"..",
		"../escape",
		"a/b",
		strings.Repeat("a", 63),
		strings.Repeat("a", 65),
		strings.Repeat("A", 64),
		strings.Repeat("g", 64),
	}
	for _, key := range badKeys {
		if err := archive.Put(key, []byte("x")); err == nil {
			t.Errorf("Put(%q) succeeded, want error", key)
		}
		if got, err := archive.Get(key); err == nil || got != nil {
			t.Errorf("Get(%q) = (%v, %v), want error and nil bytes", key, got, err)
		}
	}
	if files := listRegularFiles(t, parent); len(files) != 0 {
		t.Errorf("unexpected files created: %v", files)
	}
	missingKey := strings.Repeat("0", 64)
	if got, err := archive.Get(missingKey); !errors.Is(err, evidencex.ErrArchiveNotFound) || got != nil {
		t.Errorf("Get missing = (%v, %v), want ErrArchiveNotFound", got, err)
	}
	blockingFile := filepath.Join(parent, "blocking-file")
	if err := os.WriteFile(blockingFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := evidencex.NewLocalArchive(filepath.Join(blockingFile, "sub"), 100); err == nil {
		t.Errorf("NewLocalArchive under regular file succeeded, want error")
	}
	dirAsObject := filepath.Join(dir, strings.Repeat("e", 64)+".gz")
	if err := os.MkdirAll(dirAsObject, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := archive.Get(strings.Repeat("e", 64)); err == nil || got != nil {
		t.Errorf("Get directory object = (%v, %v), want error and nil bytes", got, err)
	}
}

func TestLocalArchive_CorruptStoredPayloadFailsDecode(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "archive")
	archive, err := evidencex.NewLocalArchive(dir, 4096)
	if err != nil {
		t.Fatalf("NewLocalArchive: %v", err)
	}
	key := strings.Repeat("f", 64)
	if err := os.WriteFile(filepath.Join(dir, key+".gz"), []byte("not-gzip"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if got, err := archive.Get(key); err == nil || got != nil {
		t.Errorf("Get corrupt = (%v, %v), want decode error and nil bytes", got, err)
	}
}
