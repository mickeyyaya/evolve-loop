package modelcatalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// FileName is the catalog's basename inside the .evolve directory.
const FileName = "model-catalog.json"

// pathFor returns the catalog file path under evolveDir.
func pathFor(evolveDir string) string {
	return filepath.Join(evolveDir, FileName)
}

// Read loads the catalog from evolveDir. A missing file is NOT an error: it
// yields a zero Catalog (which Empty()/IsStale() report as needs-refresh), so
// the first run transparently triggers a refresh. Malformed JSON is an error
// — a corrupt cache should be surfaced, not silently treated as empty.
func Read(evolveDir string) (Catalog, error) {
	p := pathFor(evolveDir)
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
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		return fmt.Errorf("modelcatalog: mkdir %s: %w", evolveDir, err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("modelcatalog: marshal: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(evolveDir, FileName+".*.tmp")
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
	if err := os.Rename(tmpName, pathFor(evolveDir)); err != nil {
		cleanup()
		return fmt.Errorf("modelcatalog: rename: %w", err)
	}
	return nil
}
