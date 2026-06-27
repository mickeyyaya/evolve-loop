package ledger

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAnchor_CreateTempError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permission bits are not enforced")
	}
	if runtime.GOOS == "windows" {
		t.Skip("unix permission semantics not applicable on windows")
	}

	l, dir := seedLedger(t, 3)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	err := l.Anchor(context.Background(), 1, "read-only anchor dir")
	if err == nil {
		t.Fatal("Anchor in read-only dir succeeded, want create temp error")
	}
	if !strings.Contains(err.Error(), "create temp") {
		t.Fatalf("error = %v, want create temp context", err)
	}
}

func TestAnchor_CreateTempParentFileError(t *testing.T) {
	l, dir := seedLedger(t, 3)
	blocker := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	l.anchorPath = filepath.Join(blocker, "ledger-anchor.json")

	err := l.Anchor(context.Background(), 1, "anchor parent is a file")
	if err == nil {
		t.Fatal("Anchor with file parent for anchor path succeeded, want create temp error")
	}
	if !strings.Contains(err.Error(), "create temp") {
		t.Fatalf("error = %v, want create temp context", err)
	}
}

func TestAnchor_WriteError(t *testing.T) {
	old := anchorWrite
	t.Cleanup(func() { anchorWrite = old })
	anchorWrite = func(*os.File, []byte) (int, error) { return 0, errors.New("write failed") }

	l, _ := seedLedger(t, 3)
	err := l.Anchor(context.Background(), 1, "write fails")
	if err == nil {
		t.Fatal("Anchor with write failure succeeded, want error")
	}
	if !strings.Contains(err.Error(), "write") {
		t.Fatalf("error = %v, want write context", err)
	}
}

func TestAnchor_CloseError(t *testing.T) {
	old := anchorClose
	t.Cleanup(func() { anchorClose = old })
	anchorClose = func(f *os.File) error {
		_ = old(f)
		return errors.New("close failed")
	}

	l, _ := seedLedger(t, 3)
	err := l.Anchor(context.Background(), 1, "close fails")
	if err == nil {
		t.Fatal("Anchor with close failure succeeded, want error")
	}
	if !strings.Contains(err.Error(), "close") {
		t.Fatalf("error = %v, want close context", err)
	}
}

func TestAnchor_RenameError(t *testing.T) {
	l, dir := seedLedger(t, 3)
	anchorPath := filepath.Join(dir, "ledger-anchor.json")
	if err := os.MkdirAll(filepath.Join(anchorPath, "child"), 0o755); err != nil {
		t.Fatalf("occupy anchor path: %v", err)
	}

	err := l.Anchor(context.Background(), 1, "anchor path occupied by directory")
	if err == nil {
		t.Fatal("Anchor over occupied directory succeeded, want rename error")
	}
	if !strings.Contains(err.Error(), "rename") {
		t.Fatalf("error = %v, want rename context", err)
	}
	matches, globErr := filepath.Glob(filepath.Join(dir, "ledger-anchor.*.tmp"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files leaked after failed anchor rename: %v", matches)
	}
}

func TestAnchor_GatherError(t *testing.T) {
	l, dir := seedLedger(t, 3)
	segDir := filepath.Join(dir, segmentsDirName)
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(segDir, "seg-0001.jsonl.gz"), []byte("not gzip\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := l.Anchor(context.Background(), 1, "corrupt segment")
	if err == nil {
		t.Fatal("Anchor with corrupt segment succeeded, want gather error")
	}
	if !strings.Contains(err.Error(), "gunzip") {
		t.Fatalf("error = %v, want corrupt segment/gunzip context", err)
	}
}

func TestLoadAnchorSHA_CorruptJSON(t *testing.T) {
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	lines, sha := chainLines()
	var body string
	for _, ln := range lines {
		body += ln + "\n"
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "ledger.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "ledger.tip"), []byte(fmt.Sprintf("3:%s", sha[3])), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "ledger-anchor.json"), []byte("{not-json"), 0o644); err != nil {
		t.Fatal(err)
	}

	l := New(evolveDir)
	if got := l.loadAnchorSHA(); got != "" {
		t.Fatalf("loadAnchorSHA corrupt JSON = %q, want empty string", got)
	}
	if err := l.Verify(context.Background()); err != nil {
		t.Fatalf("Verify should degrade to strict no-anchor behavior on corrupt anchor JSON: %v", err)
	}
}

func TestDecodeLedgerLine_InvalidJSON(t *testing.T) {
	if _, _, err := decodeLedgerLine([]byte("{not-json")); err == nil {
		t.Fatal("decodeLedgerLine accepted invalid JSON, want error")
	}
}
