package modelcatalog

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func requireErrContains(t *testing.T, err error, sub string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", sub)
	}
	if !strings.Contains(err.Error(), sub) {
		t.Fatalf("error %q does not contain %q", err.Error(), sub)
	}
}

func TestReadOnDirectoryPathErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, FileName), 0o755); err != nil {
		t.Fatalf("arrange: mkdir catalog-as-dir: %v", err)
	}

	c, err := Read(dir)

	requireErrContains(t, err, "modelcatalog: read")
	if !c.Empty() {
		t.Fatalf("on read error the returned catalog must be empty, got %+v", c)
	}
}

func TestWriteMkdirAllFailsWhenParentIsFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("i am a file, not a dir"), 0o644); err != nil {
		t.Fatalf("arrange: write blocker: %v", err)
	}
	evolveDir := filepath.Join(blocker, ".evolve")

	err := Write(evolveDir, sampleCatalog(time.Unix(0, 0)))

	requireErrContains(t, err, "modelcatalog: mkdir")
}

func TestWriteCreateTempFailsInReadOnlyDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permission bits are not enforced")
	}
	if runtime.GOOS == "windows" {
		t.Skip("unix permission semantics not applicable on windows")
	}
	t.Parallel()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("arrange: chmod read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	err := Write(dir, sampleCatalog(time.Unix(0, 0)))

	requireErrContains(t, err, "modelcatalog: tempfile")
}

func TestWriteRenameFailsWhenTargetIsDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, FileName)
	if err := os.MkdirAll(filepath.Join(target, "child"), 0o755); err != nil {
		t.Fatalf("arrange: mkdir target-as-nonempty-dir: %v", err)
	}

	err := Write(dir, sampleCatalog(time.Unix(0, 0)))

	requireErrContains(t, err, "modelcatalog: rename")

	entries, rerr := os.ReadDir(dir)
	if rerr != nil {
		t.Fatalf("ReadDir after failed rename: %v", rerr)
	}
	for _, e := range entries {
		if e.Name() != FileName && filepath.Ext(e.Name()) == ".tmp" {
			t.Fatalf("temp file leaked after failed rename: %s", e.Name())
		}
	}
}
