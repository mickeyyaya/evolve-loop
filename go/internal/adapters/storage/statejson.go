// Package storage is the filesystem adapter for the core.Storage port.
// It reads/writes the .evolve/ state surface used by the orchestrator.
package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// FilesystemStorage reads/writes JSON state files under a fixed .evolve
// directory. Concurrent-safe via flock on .evolve/.lock (see lock.go).
type FilesystemStorage struct {
	evolveDir string
	pl        *processLock
}

// New returns a storage adapter rooted at evolveDir.
func New(evolveDir string) *FilesystemStorage {
	return &FilesystemStorage{
		evolveDir: evolveDir,
		pl:        newProcessLock(filepath.Join(evolveDir, ".lock")),
	}
}

// ReadState parses .evolve/state.json. Missing file → zero State (no error).
func (s *FilesystemStorage) ReadState(_ context.Context) (core.State, error) {
	var st core.State
	path := filepath.Join(s.evolveDir, "state.json")
	if err := readJSON(path, &st); err != nil {
		return core.State{}, err
	}
	return st, nil
}

// WriteState atomically replaces .evolve/state.json.
func (s *FilesystemStorage) WriteState(_ context.Context, st core.State) error {
	path := filepath.Join(s.evolveDir, "state.json")
	return writeJSONAtomic(path, st)
}

// ReadCycleState parses .evolve/cycle-state.json. Missing file → zero
// CycleState (no error) — matches bash cycle-state.sh bootstrap behaviour.
func (s *FilesystemStorage) ReadCycleState(_ context.Context) (core.CycleState, error) {
	var cs core.CycleState
	path := filepath.Join(s.evolveDir, "cycle-state.json")
	if err := readJSON(path, &cs); err != nil {
		return core.CycleState{}, err
	}
	return cs, nil
}

// WriteCycleState atomically replaces .evolve/cycle-state.json.
func (s *FilesystemStorage) WriteCycleState(_ context.Context, cs core.CycleState) error {
	path := filepath.Join(s.evolveDir, "cycle-state.json")
	return writeJSONAtomic(path, cs)
}

// readJSON reads a JSON file into v. Missing file is not an error;
// returns nil and leaves v untouched.
func readJSON(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read %s: %w", path, err)
	}
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("unmarshal %s: %w", path, err)
	}
	return nil
}

// ioHooks holds injectable seams so tests can drive error branches
// (fsync, close, rename) that are otherwise impossible to trigger with
// a real filesystem on macOS/Linux without root.
type ioHooks struct {
	marshal func(v any) ([]byte, error)
	write   func(f *os.File, b []byte) (int, error)
	sync    func(f *os.File) error
	closeF  func(f *os.File) error
	rename  func(oldpath, newpath string) error
}

// hooks is the default I/O surface. Tests swap fields temporarily via
// withHooks to inject failures.
var hooks = ioHooks{
	marshal: func(v any) ([]byte, error) { return json.MarshalIndent(v, "", "  ") },
	write:   func(f *os.File, b []byte) (int, error) { return f.Write(b) },
	sync:    func(f *os.File) error { return f.Sync() },
	closeF:  func(f *os.File) error { return f.Close() },
	rename:  os.Rename,
}

// withHooks temporarily replaces hooks for the duration of fn. Used by
// tests only; the calling test must hold the package-level test mutex
// (see hooks_test_helpers.go).
func withHooks(replacement ioHooks, fn func()) {
	prev := hooks
	if replacement.marshal != nil {
		hooks.marshal = replacement.marshal
	}
	if replacement.write != nil {
		hooks.write = replacement.write
	}
	if replacement.sync != nil {
		hooks.sync = replacement.sync
	}
	if replacement.closeF != nil {
		hooks.closeF = replacement.closeF
	}
	if replacement.rename != nil {
		hooks.rename = replacement.rename
	}
	defer func() { hooks = prev }()
	fn()
}

// writeJSONAtomic writes v as indented JSON to path via a sibling tmp
// file + rename. Crash-safe: either the old file or the new file is
// visible, never a half-written truncation.
func writeJSONAtomic(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	buf, err := hooks.marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	buf = append(buf, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("tmp: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := hooks.write(tmp, buf); err != nil {
		_ = hooks.closeF(tmp)
		cleanup()
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := hooks.sync(tmp); err != nil {
		_ = hooks.closeF(tmp)
		cleanup()
		return fmt.Errorf("sync tmp: %w", err)
	}
	if err := hooks.closeF(tmp); err != nil {
		cleanup()
		return fmt.Errorf("close tmp: %w", err)
	}
	if err := hooks.rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
