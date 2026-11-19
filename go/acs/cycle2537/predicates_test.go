//go:build acs

package cycle2537

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

const archiveCap = 4096

func digestOf(t *testing.T, src []byte) string {
	t.Helper()
	res, err := evidencex.Extract(src, evidencex.Options{Mode: evidencex.ModePointer})
	if err != nil {
		t.Fatalf("pointer digest: %v", err)
	}
	return res.Digest
}

func openArchive(t *testing.T, dir string, maxBytes int64) evidencex.EvidenceArchive {
	t.Helper()
	var a evidencex.EvidenceArchive
	a, err := evidencex.NewLocalArchive(dir, maxBytes)
	if err != nil {
		t.Fatalf("NewLocalArchive(%q, %d): %v", dir, maxBytes, err)
	}
	return a
}

func archiveDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "evidence", "archive")
}

func regularFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.Type().IsRegular() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return files
}

func TestC2537_001_ArchivedSourceIsFoundBySourceSHA256(t *testing.T) {
	src := []byte(strings.Repeat("cycle 7 phase build line\n", 40) + "ERROR boom\n")
	sha := digestOf(t, src)
	dir := archiveDir(t)
	if err := openArchive(t, dir, archiveCap*4).Put(sha, src); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := openArchive(t, dir, archiveCap*4).Get(sha)
	if err != nil {
		t.Fatalf("Get by source_sha256 from a fresh adapter on the same dir: %v", err)
	}
	if !bytes.Equal(got, src) {
		t.Errorf("round trip changed the bytes: got %d bytes, want %d", len(got), len(src))
	}
}

func TestC2537_002_EachSourceIsFoundByItsOwnDigest(t *testing.T) {
	a := openArchive(t, archiveDir(t), archiveCap)
	sources := [][]byte{
		[]byte("first source\n"),
		[]byte("second source\n"),
		{},
		{0x00, 0xff, 0x7f, 0x80, '\n', 0x00},
	}
	for _, src := range sources {
		if err := a.Put(digestOf(t, src), src); err != nil {
			t.Fatalf("Put(%q): %v", src, err)
		}
	}
	for _, src := range sources {
		got, err := a.Get(digestOf(t, src))
		if err != nil {
			t.Fatalf("Get(%q): %v", src, err)
		}
		if !bytes.Equal(got, src) {
			t.Errorf("Get returned %q, want %q", got, src)
		}
	}
}

func TestC2537_003_WriteAboveTheSizeCapIsRefusedAndLeavesNothing(t *testing.T) {
	dir := archiveDir(t)
	a := openArchive(t, dir, archiveCap)
	oversized := []byte(strings.Repeat("a", archiveCap+1))
	sha := digestOf(t, oversized)
	err := a.Put(sha, oversized)
	if !errors.Is(err, evidencex.ErrArchiveTooLarge) {
		t.Fatalf("Put of cap+1 bytes: err = %v, want ErrArchiveTooLarge", err)
	}
	if _, getErr := a.Get(sha); !errors.Is(getErr, evidencex.ErrArchiveNotFound) {
		t.Errorf("Get after refused Put: err = %v, want ErrArchiveNotFound", getErr)
	}
	if files := regularFiles(t, dir); len(files) != 0 {
		t.Errorf("refused write left files behind: %v", files)
	}
}

func TestC2537_004_SizeCapCountsSourceBytesAndAllowsExactlyTheCap(t *testing.T) {
	a := openArchive(t, archiveDir(t), archiveCap)
	atCap := []byte(strings.Repeat("z", archiveCap))
	shaAtCap := digestOf(t, atCap)
	if err := a.Put(shaAtCap, atCap); err != nil {
		t.Fatalf("Put of exactly cap bytes must succeed: %v", err)
	}
	got, err := a.Get(shaAtCap)
	if err != nil || !bytes.Equal(got, atCap) {
		t.Errorf("Get at cap: len=%d err=%v", len(got), err)
	}
	compressibleOverCap := []byte(strings.Repeat("z", archiveCap+1))
	if err := a.Put(digestOf(t, compressibleOverCap), compressibleOverCap); !errors.Is(err, evidencex.ErrArchiveTooLarge) {
		t.Errorf("highly compressible cap+1 source: err = %v, want ErrArchiveTooLarge", err)
	}
}

func TestC2537_005_UnknownDigestIsNotFound(t *testing.T) {
	a := openArchive(t, archiveDir(t), archiveCap)
	absent := hex.EncodeToString(make([]byte, sha256.Size))
	got, err := a.Get(absent)
	if !errors.Is(err, evidencex.ErrArchiveNotFound) {
		t.Fatalf("Get of an unarchived digest: err = %v, want ErrArchiveNotFound", err)
	}
	if got != nil {
		t.Errorf("Get of an unarchived digest returned %d bytes", len(got))
	}
}

func TestC2537_006_ArchivingTheSameSourceTwiceKeepsOneIntactCopy(t *testing.T) {
	dir := archiveDir(t)
	a := openArchive(t, dir, archiveCap)
	src := []byte("repeat source line\nERROR again\n")
	sha := digestOf(t, src)
	if err := a.Put(sha, src); err != nil {
		t.Fatalf("first Put: %v", err)
	}
	before := regularFiles(t, dir)
	if err := a.Put(sha, src); err != nil {
		t.Fatalf("second Put of the same source: %v", err)
	}
	if after := regularFiles(t, dir); len(after) != len(before) {
		t.Errorf("files before = %d, after = %d: duplicate Put must not add a copy", len(before), len(after))
	}
	got, err := a.Get(sha)
	if err != nil || !bytes.Equal(got, src) {
		t.Errorf("Get after duplicate Put: %q err=%v", got, err)
	}
}

func TestC2537_007_MalformedKeyIsRefusedAndNothingEscapesTheDirectory(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "archive")
	a := openArchive(t, dir, archiveCap)
	for _, key := range []string{"", "../escape", "..", "a/b", strings.Repeat("g", 64), strings.Repeat("a", 63), strings.Repeat("A", 64)} {
		if err := a.Put(key, []byte("payload")); err == nil {
			t.Errorf("Put(%q) must be refused", key)
		}
		if _, err := a.Get(key); err == nil {
			t.Errorf("Get(%q) must fail", key)
		}
	}
	for _, f := range regularFiles(t, parent) {
		if !strings.HasPrefix(f, dir+string(filepath.Separator)) {
			t.Errorf("file outside the archive directory: %s", f)
		}
	}
	if files := regularFiles(t, dir); len(files) != 0 {
		t.Errorf("refused keys left files behind: %v", files)
	}
}

func TestC2537_008_NonPositiveCapIsRefusedAtConstruction(t *testing.T) {
	for _, maxBytes := range []int64{0, -1} {
		if a, err := evidencex.NewLocalArchive(archiveDir(t), maxBytes); err == nil {
			t.Errorf("NewLocalArchive(cap=%d) = %v, want an error", maxBytes, a)
		}
	}
}
