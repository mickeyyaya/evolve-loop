package modelcatalog

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// TestReadOnDirectoryPathErrors pins that a read error OTHER than "file does
// not exist" is surfaced, not swallowed into a zero catalog. We make the
// catalog path itself a directory: os.ReadFile then returns an EISDIR-class
// error (not fs.ErrNotExist), which must propagate. This guards the contract
// distinction in Read's doc comment — missing file ⇒ zero catalog, any other
// read failure ⇒ error.
func TestReadOnDirectoryPathErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Create a directory exactly where the catalog file would live.
	if err := os.MkdirAll(filepath.Join(dir, FileName), 0o755); err != nil {
		t.Fatalf("arrange: mkdir catalog-as-dir: %v", err)
	}

	c, err := Read(dir)

	fixtures.RequireErrContains(t, err, "modelcatalog: read")
	if !c.Empty() {
		t.Fatalf("on read error the returned catalog must be empty, got %+v", c)
	}
}

// TestWriteMkdirAllFailsWhenParentIsFile pins that Write surfaces a directory
// creation failure rather than proceeding. We seed a regular FILE at the path
// Write would try to MkdirAll, so os.MkdirAll fails with ENOTDIR.
func TestWriteMkdirAllFailsWhenParentIsFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// A regular file occupying the slot where evolveDir's parent segment lives,
	// forcing MkdirAll(evolveDir) to fail (a path component is not a directory).
	blocker := filepath.Join(root, "blocker")
	fixtures.MustWrite(t, blocker, "i am a file, not a dir")
	evolveDir := filepath.Join(blocker, ".evolve") // child of a regular file

	err := Write(evolveDir, sampleCatalog(time.Unix(0, 0)))

	fixtures.RequireErrContains(t, err, "modelcatalog: mkdir")
}

// TestWriteCreateTempFailsInReadOnlyDir pins that Write surfaces a tempfile
// creation failure. evolveDir exists (so MkdirAll is a no-op success) but is
// read-only, so os.CreateTemp inside it fails with EACCES. Skipped under root,
// where mode bits are not enforced.
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
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) }) // let t.TempDir cleanup remove it

	err := Write(dir, sampleCatalog(time.Unix(0, 0)))

	fixtures.RequireErrContains(t, err, "modelcatalog: tempfile")
}

// TestWriteRenameFailsWhenTargetIsDirectory pins that a failed atomic rename is
// surfaced. The temp file is created successfully, but os.Rename onto a path
// that is an existing NON-EMPTY directory fails (ENOTEMPTY/EISDIR), so Write
// must return the rename error and leave no torn cache.
func TestWriteRenameFailsWhenTargetIsDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Occupy the final catalog path with a non-empty directory: renaming the
	// temp FILE over a non-empty DIR fails on every supported platform.
	target := filepath.Join(dir, FileName)
	if err := os.MkdirAll(filepath.Join(target, "child"), 0o755); err != nil {
		t.Fatalf("arrange: mkdir target-as-nonempty-dir: %v", err)
	}

	err := Write(dir, sampleCatalog(time.Unix(0, 0)))

	fixtures.RequireErrContains(t, err, "modelcatalog: rename")

	// And the failed write left no leftover temp file in the directory.
	entries, rerr := os.ReadDir(dir)
	fixtures.RequireNoErr(t, rerr, "ReadDir after failed rename")
	for _, e := range entries {
		if e.Name() != FileName && filepath.Ext(e.Name()) == ".tmp" {
			t.Fatalf("temp file leaked after failed rename: %s", e.Name())
		}
	}
}
