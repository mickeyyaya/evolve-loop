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

const FileName = "model-catalog.json"

const PrevFileName = "model-catalog.prev.json"

func pathFor(evolveDir string) string {
	return filepath.Join(evolveDir, FileName)
}

func prevPathFor(evolveDir string) string {
	return filepath.Join(evolveDir, PrevFileName)
}

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

func Read(evolveDir string) (Catalog, error) {
	return readCatalogFile(pathFor(evolveDir))
}

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

func Write(evolveDir string, c Catalog) error {
	return writeCatalogFileWithPreRename(evolveDir, FileName, c, func() { retainPrevious(evolveDir) })
}

func writeCatalogFile(evolveDir, basename string, c Catalog) error {
	return writeCatalogFileWithPreRename(evolveDir, basename, c, nil)
}

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
	cleanup := func() { _ = os.Remove(tmpName) }

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
