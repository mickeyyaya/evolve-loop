// Package atomicwrite is the single implementation of crash-safe file writes:
// write to a uniquely-named temp file in the destination's directory, then
// rename it over the target (rename is atomic within one filesystem). The
// parent directory is created if missing, the final file is mode 0644, and the
// temp file is removed on any failure.
//
// It replaces the ~half-dozen near-identical writeJSONAtomic/atomicWrite copies
// that were scattered across packages. The OS-fault branches (temp create,
// write, close, rename) live here and are exercised once via the package-level
// seams below, so every caller — which collapses to a one-line delegation — is
// fully covered by its own happy-path test.
//
// NOTE: storage.writeJSONAtomic is deliberately NOT a client — it has distinct
// semantics (trailing newline + fsync for state.json durability) and its own
// injectable hooks seam.
package atomicwrite

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Seams. Overridable in tests to exercise the OS-fault branches deterministically.
var (
	mkdirAll   = os.MkdirAll
	renameFile = os.Rename
	removeFile = os.Remove
	createTemp = func(dir, pattern string) (tempFile, error) { return os.CreateTemp(dir, pattern) }
)

// tempFile is the subset of *os.File the algorithm needs; an interface so tests
// can inject a handle whose Write/Chmod/Close fail.
type tempFile interface {
	io.Writer
	Name() string
	Chmod(fs.FileMode) error
	Close() error
}

// Bytes atomically writes data to path (mode 0644), creating path's parent
// directory if needed.
func Bytes(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := mkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("atomicwrite: mkdir %s: %w", dir, err)
	}
	tmp, err := createTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("atomicwrite: create temp in %s: %w", dir, err)
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = removeFile(name)
		return fmt.Errorf("atomicwrite: write %s: %w", name, err)
	}
	// CreateTemp makes the file 0600; match the historical 0644 of the helpers
	// this replaces so on-disk permissions are unchanged for their callers.
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		_ = removeFile(name)
		return fmt.Errorf("atomicwrite: chmod %s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		_ = removeFile(name)
		return fmt.Errorf("atomicwrite: close %s: %w", name, err)
	}
	if err := renameFile(name, path); err != nil {
		_ = removeFile(name)
		return fmt.Errorf("atomicwrite: rename %s -> %s: %w", name, path, err)
	}
	return nil
}

// JSON marshals v as 2-space-indented JSON (no trailing newline) and writes it
// atomically via Bytes — the exact format the writeJSONAtomic copies produced.
func JSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("atomicwrite: marshal: %w", err)
	}
	return Bytes(path, data)
}
