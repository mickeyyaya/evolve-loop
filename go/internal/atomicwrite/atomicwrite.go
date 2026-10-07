// Package atomicwrite is the single implementation of crash-safe write-then-rename file writes.
// See docs/architecture/packages/internal-atomicwrite.md.
package atomicwrite

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Seams. Overridable in tests to exercise the OS-fault branches deterministically.
var (
	mkdirAll   = os.MkdirAll
	renameFile = os.Rename
	removeFile = os.Remove
	createTemp = func(dir, pattern string) (tempFile, error) { return os.CreateTemp(dir, pattern) }
	openDir    = func(dir string) (syncCloser, error) { return os.Open(dir) }
)

// tempFile is the subset of *os.File the algorithm needs; an interface so tests
// can inject a handle whose Write/Chmod/Close fail.
type tempFile interface {
	io.Writer
	syncCloser
	Name() string
	Chmod(fs.FileMode) error
}

type syncCloser interface {
	Sync() error
	Close() error
}

const tempSuffix = ".tmp"

// Bytes atomically writes data to path (mode 0644), creating path's parent
// directory if needed.
func Bytes(path string, data []byte) error {
	return write(path, data, 0o644, plainTempPattern(path), false)
}

func Durable(path string, data []byte, mode fs.FileMode) error {
	return write(path, data, mode, tempPattern(path, os.Getpid()), true)
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

func TempWriter(name string) (pid int, ok bool) {
	body, ok := strings.CutSuffix(name, tempSuffix)
	if !ok || !strings.HasPrefix(body, ".") {
		return 0, false
	}
	random := strings.LastIndexByte(body, '.')
	if random <= 0 {
		return 0, false
	}
	body = body[:random]
	sep := strings.LastIndexByte(body, '.')
	if sep <= 0 {
		return 0, false
	}
	pid, err := strconv.Atoi(body[sep+1:])
	return pid, err == nil && pid > 0
}

func tempPattern(path string, pid int) string {
	return "." + filepath.Base(path) + "." + strconv.Itoa(pid) + ".*" + tempSuffix
}

func plainTempPattern(path string) string {
	return "." + filepath.Base(path) + ".*" + tempSuffix
}

func write(path string, data []byte, mode fs.FileMode, pattern string, durable bool) error {
	dir := filepath.Dir(path)
	if err := mkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("atomicwrite: mkdir %s: %w", dir, err)
	}
	tmp, err := createTemp(dir, pattern)
	if err != nil {
		return fmt.Errorf("atomicwrite: create temp in %s: %w", dir, err)
	}
	name := tmp.Name()
	if err := fill(tmp, data, mode, durable); err != nil {
		_ = removeFile(name)
		return err
	}
	if err := renameFile(name, path); err != nil {
		_ = removeFile(name)
		return fmt.Errorf("atomicwrite: rename %s -> %s: %w", name, path, err)
	}
	if !durable {
		return nil
	}
	return syncDir(dir)
}

func fill(tmp tempFile, data []byte, mode fs.FileMode, durable bool) error {
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("atomicwrite: write %s: %w", name, err)
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("atomicwrite: chmod %s: %w", name, err)
	}
	if durable {
		if err := tmp.Sync(); err != nil {
			_ = tmp.Close()
			return fmt.Errorf("atomicwrite: sync %s: %w", name, err)
		}
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("atomicwrite: close %s: %w", name, err)
	}
	return nil
}

func syncDir(dir string) error {
	d, err := openDir(dir)
	if err != nil {
		return fmt.Errorf("atomicwrite: open dir %s: %w", dir, err)
	}
	if err := d.Sync(); err != nil {
		_ = d.Close()
		return fmt.Errorf("atomicwrite: sync dir %s: %w", dir, err)
	}
	if err := d.Close(); err != nil {
		return fmt.Errorf("atomicwrite: close dir %s: %w", dir, err)
	}
	return nil
}
