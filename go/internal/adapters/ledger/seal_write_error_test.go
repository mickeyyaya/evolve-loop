package ledger

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteSegment_MkdirError(t *testing.T) {
	dir := t.TempDir()
	parentFile := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(parentFile, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := writeSegment(filepath.Join(parentFile, "seg-0001.jsonl.gz"), []byte("x\n"))
	if err == nil {
		t.Fatal("writeSegment with file parent succeeded, want mkdir error")
	}
	if !strings.Contains(err.Error(), "mkdir") {
		t.Fatalf("error = %v, want mkdir context", err)
	}
}

func TestWriteSegment_CreateTempError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permission bits are not enforced")
	}
	if runtime.GOOS == "windows" {
		t.Skip("unix permission semantics not applicable on windows")
	}

	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	err := writeSegment(filepath.Join(dir, "seg-0001.jsonl.gz"), []byte("x\n"))
	if err == nil {
		t.Fatal("writeSegment in read-only dir succeeded, want temp error")
	}
	if !strings.Contains(err.Error(), "tmp") {
		t.Fatalf("error = %v, want tmp context", err)
	}
}

func TestRewriteLive_CreateTempError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permission bits are not enforced")
	}
	if runtime.GOOS == "windows" {
		t.Skip("unix permission semantics not applicable on windows")
	}

	dir := t.TempDir()
	l := New(dir)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	err := l.rewriteLive([][]byte{[]byte("x")})
	if err == nil {
		t.Fatal("rewriteLive in read-only dir succeeded, want temp error")
	}
	if !strings.Contains(err.Error(), "live rewrite tmp") {
		t.Fatalf("error = %v, want live rewrite tmp context", err)
	}
}

func TestSealLocked_ResidueRewriteLiveCreateTempError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permission bits are not enforced")
	}
	if runtime.GOOS == "windows" {
		t.Skip("unix permission semantics not applicable on windows")
	}

	l, dir := seedLedger(t, 5)
	raw, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := splitLines(raw)
	if len(lines) < 3 {
		t.Fatalf("seed ledger produced %d lines, want at least 3", len(lines))
	}
	prefix := raw[:prefixLen(raw, 3)]
	segPath := filepath.Join(dir, segmentsDirName, "seg-0001.jsonl.gz")
	if err := writeSegment(segPath, prefix); err != nil {
		t.Fatalf("write residue segment: %v", err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	_, err = l.sealLocked(2)
	if err == nil {
		t.Fatal("sealLocked residue recovery in read-only dir succeeded, want live rewrite temp error")
	}
	if !strings.Contains(err.Error(), "live rewrite tmp") {
		t.Fatalf("error = %v, want live rewrite tmp context", err)
	}
}

func TestSegmentFiles_ReadDirError(t *testing.T) {
	dir := t.TempDir()
	segDir := filepath.Join(dir, segmentsDirName)
	if err := os.WriteFile(segDir, []byte("file-not-dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := segmentFiles(segDir); err == nil {
		t.Fatal("segmentFiles on regular file succeeded, want read error")
	}
}

func TestSeal_AnchorAppendError(t *testing.T) {
	l, dir := seedLedger(t, 5)
	if err := os.WriteFile(filepath.Join(dir, "ledger.tip"), []byte("malformed-tip"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := l.Seal(context.Background(), 2)
	if err == nil {
		t.Fatal("Seal with malformed tip succeeded, want anchor append error")
	}
	if !strings.Contains(err.Error(), "anchor append") {
		t.Fatalf("error = %v, want anchor append context", err)
	}
}

func TestPrefixLen_NoTrailingNewline(t *testing.T) {
	raw := []byte("one\ntwo")
	if got := prefixLen(raw, 2); got != len(raw) {
		t.Fatalf("prefixLen without trailing newline = %d, want %d", got, len(raw))
	}
}
