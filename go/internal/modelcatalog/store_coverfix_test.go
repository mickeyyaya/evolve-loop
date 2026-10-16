package modelcatalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteCatalogFileCoverfix_MkdirFailureIsReported(t *testing.T) {
	t.Parallel()
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := writeCatalogFileWithPreRename(filepath.Join(blocker, "sub"), FileName, Catalog{}, nil)
	if err == nil || !strings.Contains(err.Error(), "mkdir") {
		t.Fatalf("expected mkdir error, got %v", err)
	}
}

func TestWriteCatalogFileCoverfix_MarshalFailureLeavesNoFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bad := Catalog{FetchedAt: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)}
	err := writeCatalogFileWithPreRename(dir, FileName, bad, nil)
	if err == nil || !strings.Contains(err.Error(), "marshal") {
		t.Fatalf("expected marshal error, got %v", err)
	}
	entries, rerr := os.ReadDir(dir)
	if rerr != nil || len(entries) != 0 {
		t.Fatalf("marshal failure must leave dir empty, got %v err %v", entries, rerr)
	}
}
