package modelcatalog

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeTempFile struct {
	name       string
	failWrite  bool
	failSync   bool
	failClose  bool
	closeCalls int
}

func (f *fakeTempFile) Write(p []byte) (int, error) {
	if f.failWrite {
		return 0, errors.New("injected: temp write failure")
	}
	return len(p), nil
}

func (f *fakeTempFile) Sync() error {
	if f.failSync {
		return errors.New("injected: temp sync failure")
	}
	return nil
}

func (f *fakeTempFile) Close() error {
	f.closeCalls++
	if f.failClose {
		return errors.New("injected: temp close failure")
	}
	return nil
}

func (f *fakeTempFile) Name() string { return f.name }

func installFailingTemp(t *testing.T, fake *fakeTempFile) {
	t.Helper()
	orig := createTemp
	t.Cleanup(func() { createTemp = orig })
	createTemp = func(dir, pattern string) (tempFile, error) {
		real, err := os.CreateTemp(dir, pattern)
		if err != nil {
			return nil, err
		}
		_ = real.Close()
		fake.name = real.Name()
		return fake, nil
	}
}

func assertWriteFailureCleanedUp(t *testing.T, dir, tmpPath string) {
	t.Helper()
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf("temp file %s not cleaned up after failure (stat err=%v)", tmpPath, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir after failed write: %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("temp file leaked after failed write: %s", e.Name())
		}
	}
}

func TestWriteSurfacesTempWriteFailureAndCleansUp(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeTempFile{failWrite: true}
	installFailingTemp(t, fake)

	err := Write(dir, sampleCatalog(time.Unix(0, 0)))

	requireErrContains(t, err, "modelcatalog: write temp")
	if fake.closeCalls != 1 {
		t.Errorf("write-failure path must Close the temp exactly once, got %d", fake.closeCalls)
	}
	assertWriteFailureCleanedUp(t, dir, fake.name)
}

func TestWriteSurfacesTempSyncFailureAndCleansUp(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeTempFile{failSync: true}
	installFailingTemp(t, fake)

	err := Write(dir, sampleCatalog(time.Unix(0, 0)))

	requireErrContains(t, err, "modelcatalog: sync temp")
	if fake.closeCalls != 1 {
		t.Errorf("sync-failure path must Close the temp exactly once, got %d", fake.closeCalls)
	}
	assertWriteFailureCleanedUp(t, dir, fake.name)
}

func TestWriteSurfacesTempCloseFailureAndCleansUp(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeTempFile{failClose: true}
	installFailingTemp(t, fake)

	err := Write(dir, sampleCatalog(time.Unix(0, 0)))

	requireErrContains(t, err, "modelcatalog: close temp")
	if fake.closeCalls != 1 {
		t.Errorf("close-failure path must call Close exactly once (no double-close), got %d", fake.closeCalls)
	}
	assertWriteFailureCleanedUp(t, dir, fake.name)
}
