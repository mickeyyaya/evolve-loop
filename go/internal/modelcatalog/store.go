package modelcatalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// FileName is the catalog's basename inside the .evolve directory.
const FileName = "model-catalog.json"

// PrevFileName is the basename of the copy Write retains of the catalog it is
// about to overwrite. The live catalog is gitignored and partly hand-authored
// (it can hold entries no picker parser is able to regenerate), so an atomic
// rename with nothing retained makes a single bad refresh unrecoverable.
// Rollback is `cp model-catalog.prev.json model-catalog.json`.
const PrevFileName = "model-catalog.prev.json"

// pathFor returns the catalog file path under evolveDir.
func pathFor(evolveDir string) string {
	return filepath.Join(evolveDir, FileName)
}

// prevPathFor returns the retained-previous-catalog path under evolveDir.
func prevPathFor(evolveDir string) string {
	return filepath.Join(evolveDir, PrevFileName)
}

// retainPrevious best-effort copies the catalog currently on disk to
// PrevFileName. Every failure is swallowed: retention is a rollback
// convenience and must NEVER block the write that matters (a missing prior on
// the first write is the common case, not an error).
//
// Temp+rename, matching Write, for three reasons a plain WriteFile gets wrong:
//   - ATOMICITY: fleet lanes are separate processes sharing one .evolve, so
//     concurrent refreshes would interleave into a torn .prev — and a torn
//     .prev turns the documented rollback into a second outage.
//   - SYMLINKS: WriteFile writes THROUGH a pre-existing symlink at the
//     retention path; Rename replaces it. This is the only write in the
//     package that a planted link could otherwise redirect.
//   - MODE: os.CreateTemp yields 0600 and Rename preserves it, so the copy
//     inherits the live catalog's owner-only access instead of widening it.
//
// os.CreateTemp is called DIRECTLY rather than through the createTemp package
// var: that seam exists so store's failure-injection tests can drive Write's
// error paths, and retention must not consume it.
func retainPrevious(evolveDir string) {
	raw, err := os.ReadFile(pathFor(evolveDir))
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(evolveDir, PrevFileName+".*.tmp")
	if err != nil {
		return
	}
	name := tmp.Name()
	if _, werr := tmp.Write(raw); werr != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return
	}
	if cerr := tmp.Close(); cerr != nil {
		_ = os.Remove(name)
		return
	}
	if rerr := os.Rename(name, prevPathFor(evolveDir)); rerr != nil {
		_ = os.Remove(name)
	}
}

type tempFile interface {
	io.Writer
	Sync() error
	Close() error
	Name() string
}

var createTemp = func(dir, pattern string) (tempFile, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Read loads the catalog from evolveDir. A missing file is NOT an error: it
// yields a zero Catalog (which Empty()/IsStale() report as needs-refresh), so
// the first run transparently triggers a refresh. Malformed JSON is an error
// — a corrupt cache should be surfaced, not silently treated as empty.
func Read(evolveDir string) (Catalog, error) {
	return readCatalogFile(pathFor(evolveDir))
}

// readCatalogFile implements the shared read contract for the live and shadow
// catalog files (missing → zero catalog, corrupt → error).
func readCatalogFile(p string) (Catalog, error) {
	raw, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return Catalog{}, nil
	}
	if err != nil {
		return Catalog{}, fmt.Errorf("modelcatalog: read %s: %w", p, err)
	}
	var c Catalog
	if err := json.Unmarshal(raw, &c); err != nil {
		return Catalog{}, fmt.Errorf("modelcatalog: parse %s: %w", p, err)
	}
	return c, nil
}

// Write persists the catalog to evolveDir atomically (temp file + rename), so
// a crash mid-write never leaves a torn cache. evolveDir is created if absent.
func Write(evolveDir string, c Catalog) error {
	// Retain the outgoing catalog only once the replacement is fully written
	// and about to land — a failed marshal/write must not rotate .prev.
	// writeCatalogFile invokes the hook between temp-close and rename.
	return writeCatalogFileWithPreRename(evolveDir, FileName, c, func() { retainPrevious(evolveDir) })
}

// writeCatalogFile atomically writes c to basename under evolveDir with no
// pre-rename hook (the shadow path: no .prev rotation).
func writeCatalogFile(evolveDir, basename string, c Catalog) error {
	return writeCatalogFileWithPreRename(evolveDir, basename, c, nil)
}

// writeCatalogFileWithPreRename is the one atomic write path for catalog
// files: temp file + fsync + rename, with an optional hook that runs after the
// replacement is durably written but before it lands (Write's .prev
// retention). The createTemp seam is preserved so store's failure-injection
// tests keep driving every error path.
func writeCatalogFileWithPreRename(evolveDir, basename string, c Catalog, preRename func()) error {
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		return fmt.Errorf("modelcatalog: mkdir %s: %w", evolveDir, err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("modelcatalog: marshal: %w", err)
	}
	data = append(data, '\n')

	tmp, err := createTemp(evolveDir, basename+".*.tmp")
	if err != nil {
		return fmt.Errorf("modelcatalog: tempfile: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) } // best-effort; no-op after a successful rename

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("modelcatalog: write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("modelcatalog: sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("modelcatalog: close temp: %w", err)
	}
	if preRename != nil {
		preRename()
	}
	if err := os.Rename(tmpName, filepath.Join(evolveDir, basename)); err != nil {
		cleanup()
		return fmt.Errorf("modelcatalog: rename: %w", err)
	}
	return nil
}
