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

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
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
	path := filepath.Join(s.evolveDir, core.CycleStateFile)
	if err := readJSON(path, &cs); err != nil {
		return core.CycleState{}, err
	}
	return cs, nil
}

// WriteCycleState atomically replaces .evolve/cycle-state.json while preserving
// the checkpoint block written by the checkpoint package. core.CycleState does
// not model that key, so a plain struct rewrite would erase resume state.
// It then dual-writes the state to <WorkspacePath>/run.json (CB.4): the
// per-run mirror the worktree provisioner symlinks guard hooks at, so guards
// inside a cycle worktree read this run's phase — not whichever concurrent
// run last wrote the global file.
func (s *FilesystemStorage) WriteCycleState(_ context.Context, cs core.CycleState) error {
	path := filepath.Join(s.evolveDir, core.CycleStateFile)
	// ADR-0049 G7: serialize the whole read-modify-write (and the run.json
	// mirror) on the cycle-state.json sidecar lock. checkpoint.ApplyToStateFile
	// (the other read-modify-writer of this file) holds the SAME lock, so a
	// concurrent fleet cycle that owns a different key (the checkpoint block)
	// never renames over a stale read and reverts this write — or vice versa.
	// No-op cost for the live sequential loop (uncontended); shared with the
	// checkpoint writer across packages via flock.WithPathLock (the single
	// sidecar-lock home).
	return flock.WithPathLock(path, func() error {
		checkpoint, ok, err := readExistingCheckpoint(path)
		if err != nil {
			return err
		}

		// Determine the value to persist at the global path. When a checkpoint
		// block already exists it must be spliced in (the CycleState struct does
		// not model it, so a plain rewrite would erase resume state).
		var global any = cs
		if ok {
			raw, err := json.Marshal(cs)
			if err != nil {
				return fmt.Errorf("marshal cycle state: %w", err)
			}
			var merged map[string]json.RawMessage
			if err := json.Unmarshal(raw, &merged); err != nil {
				return fmt.Errorf("unmarshal cycle state: %w", err)
			}
			merged["checkpoint"] = checkpoint
			global = merged
		}
		if err := writeJSONAtomic(path, global); err != nil {
			return err
		}
		return mirrorRunState(cs)
	})
}

// mirrorRunState writes the run.json guard mirror into the run workspace.
// The checkpoint block is deliberately NOT mirrored: it is resume state owned
// by the global cycle-state.json (guards never read it, and resume reads the
// global file directly). A failed mirror surfaces as an error — guards inside
// the worktree deciding on a stale phase would be a fail-open, not a
// degradation.
func mirrorRunState(cs core.CycleState) error {
	if cs.WorkspacePath == "" {
		return nil
	}
	p := filepath.Join(cs.WorkspacePath, core.RunStateFile)
	if err := writeJSONAtomic(p, cs); err != nil {
		return fmt.Errorf("run-state mirror %s: %w", p, err)
	}
	return nil
}

func readExistingCheckpoint(path string) (json.RawMessage, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read %s: %w", path, err)
	}
	if len(raw) == 0 {
		return nil, false, nil
	}
	var existing map[string]json.RawMessage
	if err := json.Unmarshal(raw, &existing); err != nil {
		return nil, false, fmt.Errorf("unmarshal %s: %w", path, err)
	}
	checkpoint, ok := existing["checkpoint"]
	if !ok || len(checkpoint) == 0 || string(checkpoint) == "null" {
		return nil, false, nil
	}
	return checkpoint, true, nil
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
