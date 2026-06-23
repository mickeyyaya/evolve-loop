package modelcatalog

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/test/fixtures"
)

// ---------------------------------------------------------------------------
// RED tests for the temp-file Write/Sync/Close failure branches of store.Write.
//
// store.Write at baseline is 66.7% covered. The mkdir / CreateTemp / Rename
// error exits are already exercised by store_errors_test.go via filesystem
// tricks. The three REMAINING dark branches operate on the *temp file* after it
// is opened — tmp.Write, tmp.Sync, tmp.Close — and there is NO filesystem trick
// that makes a freshly-created regular file fail those operations portably. They
// require an injectable seam over os.CreateTemp.
//
// These tests reference a package-internal seam that Builder must add to
// store.go (see test-report.md "Handoff to Builder"):
//
//	type tempFile interface {
//	    io.Writer
//	    Sync() error
//	    Close() error
//	    Name() string
//	}
//	var createTemp = func(dir, pattern string) (tempFile, error) {
//	    f, err := os.CreateTemp(dir, pattern)
//	    if err != nil {
//	        return nil, err
//	    }
//	    return f, nil
//	}
//
// and Write must call createTemp(...) instead of os.CreateTemp(...). Until that
// seam exists this file does not compile, which is the RED state (the TDD skill
// permits compile-failure as RED). Each test verifies the full contract of an
// error branch: the wrapped error IS surfaced AND the temp file is cleaned up
// (no torn cache, no leaked .tmp) — that cleanup is the atomic-write invariant,
// not an incidental detail.
// ---------------------------------------------------------------------------

// fakeTempFile satisfies the tempFile seam and fails the one operation flagged,
// so the corresponding Write error-and-cleanup branch is driven deterministically.
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

// installFailingTemp swaps createTemp for the cycle of one test. It still creates
// a REAL file on disk (so the cleanup() os.Remove(tmpName) has an observable
// target) and points the fake's Name() at it, then returns the fake by value of
// the seam. The original createTemp is restored on cleanup.
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

// assertWriteFailureCleanedUp pins the post-condition shared by every temp-file
// error branch: the named temp file is gone and no *.tmp leaked into dir.
func assertWriteFailureCleanedUp(t *testing.T, dir, tmpPath string) {
	t.Helper()
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf("temp file %s not cleaned up after failure (stat err=%v)", tmpPath, err)
	}
	entries, err := os.ReadDir(dir)
	fixtures.RequireNoErr(t, err, "ReadDir after failed write")
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("temp file leaked after failed write: %s", e.Name())
		}
	}
}

// TestWriteSurfacesTempWriteFailureAndCleansUp drives the tmp.Write error exit:
// Write must Close the temp, remove it, and return the wrapped "write temp"
// error. RED until the createTemp seam exists.
func TestWriteSurfacesTempWriteFailureAndCleansUp(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeTempFile{failWrite: true}
	installFailingTemp(t, fake)

	err := Write(dir, sampleCatalog(time.Unix(0, 0)))

	fixtures.RequireErrContains(t, err, "modelcatalog: write temp")
	if fake.closeCalls != 1 {
		t.Errorf("write-failure path must Close the temp exactly once, got %d", fake.closeCalls)
	}
	assertWriteFailureCleanedUp(t, dir, fake.name)
}

// TestWriteSurfacesTempSyncFailureAndCleansUp drives the tmp.Sync error exit.
// The data write succeeds, but the fsync fails; Write must Close, clean up, and
// return the wrapped "sync temp" error.
func TestWriteSurfacesTempSyncFailureAndCleansUp(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeTempFile{failSync: true}
	installFailingTemp(t, fake)

	err := Write(dir, sampleCatalog(time.Unix(0, 0)))

	fixtures.RequireErrContains(t, err, "modelcatalog: sync temp")
	if fake.closeCalls != 1 {
		t.Errorf("sync-failure path must Close the temp exactly once, got %d", fake.closeCalls)
	}
	assertWriteFailureCleanedUp(t, dir, fake.name)
}

// TestWriteSurfacesTempCloseFailureAndCleansUp drives the tmp.Close error exit.
// Write+Sync succeed; the Close itself fails. Write must NOT re-close, must clean
// up the temp, and must return the wrapped "close temp" error.
func TestWriteSurfacesTempCloseFailureAndCleansUp(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeTempFile{failClose: true}
	installFailingTemp(t, fake)

	err := Write(dir, sampleCatalog(time.Unix(0, 0)))

	fixtures.RequireErrContains(t, err, "modelcatalog: close temp")
	if fake.closeCalls != 1 {
		t.Errorf("close-failure path must call Close exactly once (no double-close), got %d", fake.closeCalls)
	}
	assertWriteFailureCleanedUp(t, dir, fake.name)
}
