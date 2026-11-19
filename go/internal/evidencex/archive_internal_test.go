package evidencex

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failingWriter struct {
	failAt int
	calls  int
}

func (w *failingWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.calls >= w.failAt {
		return 0, errors.New("write failed")
	}
	return len(p), nil
}

type failingCodec struct{}

func (failingCodec) ext() string {
	return ".fail"
}

func (failingCodec) encode([]byte) ([]byte, error) {
	return nil, errors.New("encode failed")
}

func (failingCodec) decode([]byte) ([]byte, error) {
	return nil, errors.New("decode failed")
}

type stubTempFile struct {
	name     string
	writes   int
	writeErr error
	closeErr error
}

func (f *stubTempFile) Write(p []byte) (int, error) {
	f.writes++
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(p), nil
}

func (f *stubTempFile) Close() error {
	return f.closeErr
}

func (f *stubTempFile) Name() string {
	return f.name
}

func TestWriteGzip_ErrorPaths(t *testing.T) {
	t.Parallel()
	if err := writeGzip(&failingWriter{failAt: 1}, []byte("hello")); err == nil {
		t.Fatalf("expected header write error")
	}
	if err := writeGzip(&failingWriter{failAt: 2}, []byte("hello")); err == nil {
		t.Fatalf("expected close/trailer write error")
	}
}

func TestLocalArchive_InternalErrorPaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	key := strings.Repeat("a", 64)

	var codecErrArchive EvidenceArchive = &localArchive{
		dir:        dir,
		maxBytes:   64,
		codec:      failingCodec{},
		createTemp: osCreateTemp,
	}
	if err := codecErrArchive.Put(key, []byte("ok")); err == nil {
		t.Fatalf("expected codec encode error")
	}

	fileAsDir := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(fileAsDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	statErrArchive := &localArchive{
		dir:        fileAsDir,
		maxBytes:   64,
		codec:      gzipCodec{},
		createTemp: osCreateTemp,
	}
	if err := statErrArchive.Put(key, []byte("ok")); err == nil {
		t.Fatalf("expected stat error when dir is a file")
	}
	if _, err := osCreateTemp(fileAsDir, "tmp"); err == nil {
		t.Fatalf("expected osCreateTemp error when dir is a file")
	}

	tmpPath := filepath.Join(dir, "stub.tmp")
	if err := writeAtomic(dir, key, filepath.Join(dir, "dst"), []byte("x"), func(string, string) (tempFile, error) {
		return nil, errors.New("createTemp failed")
	}); err == nil {
		t.Fatalf("expected createTemp error")
	}
	writeStub := &stubTempFile{name: tmpPath, writeErr: errors.New("write failed")}
	if err := writeAtomic(dir, key, filepath.Join(dir, "dst"), []byte("x"), func(string, string) (tempFile, error) {
		return writeStub, nil
	}); err == nil || writeStub.writes != 1 {
		t.Fatalf("expected write error with 1 write, got err=%v writes=%d", err, writeStub.writes)
	}
	closeStub := &stubTempFile{name: tmpPath, closeErr: errors.New("close failed")}
	if err := writeAtomic(dir, key, filepath.Join(dir, "dst"), []byte("x"), func(string, string) (tempFile, error) {
		return closeStub, nil
	}); err == nil || closeStub.writes != 1 {
		t.Fatalf("expected close error with 1 write, got err=%v writes=%d", err, closeStub.writes)
	}

	nonEmptyDstDir := filepath.Join(dir, "occupied")
	if err := os.MkdirAll(filepath.Join(nonEmptyDstDir, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomic(dir, key, nonEmptyDstDir, []byte("x"), osCreateTemp); err == nil {
		t.Fatalf("expected rename error over non-empty directory")
	}

	truncatedGzip, err := gzipCodec{}.encode([]byte(strings.Repeat("abc", 30)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (gzipCodec{}).decode(bytes.Clone(truncatedGzip[:len(truncatedGzip)-4])); err == nil {
		t.Fatalf("expected truncated gzip decode error")
	}
}
