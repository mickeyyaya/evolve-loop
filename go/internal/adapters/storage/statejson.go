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
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/statemap"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func (s *FilesystemStorage) cycleStatePath() string {
	return core.ResolveCycleStatePath(s.evolveDir)
}

type FilesystemStorage struct {
	evolveDir string
	pl        *processLock
}

func New(evolveDir string) *FilesystemStorage {
	return &FilesystemStorage{
		evolveDir: evolveDir,
		pl:        newProcessLock(filepath.Join(evolveDir, ".lock")),
	}
}

func (s *FilesystemStorage) ReadState(_ context.Context) (core.State, error) {
	var st core.State
	path := filepath.Join(s.evolveDir, "state.json")
	if err := readJSON(path, &st); err != nil {
		return core.State{}, err
	}
	return st, nil
}

func (s *FilesystemStorage) WriteState(_ context.Context, st core.State) error {
	path := statemap.ResolveWriteTarget(filepath.Join(s.evolveDir, "state.json"))
	return writeJSONAtomic(path, st)
}

func (s *FilesystemStorage) ReadCycleState(_ context.Context) (core.CycleState, error) {
	var cs core.CycleState
	path := s.cycleStatePath()
	if err := readJSON(path, &cs); err != nil {
		return core.CycleState{}, err
	}
	return cs, nil
}

func (s *FilesystemStorage) WriteCycleState(_ context.Context, cs core.CycleState) error {
	path := statemap.ResolveWriteTarget(s.cycleStatePath())
	return flock.WithPathLock(path, func() error {
		checkpoint, ok, err := readExistingCheckpoint(path)
		if err != nil {
			return err
		}

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

type ioHooks struct {
	marshal func(v any) ([]byte, error)
}

var hooks = ioHooks{
	marshal: func(v any) ([]byte, error) { return json.MarshalIndent(v, "", "  ") },
}

func withHooks(replacement ioHooks, fn func()) {
	prev := hooks
	if replacement.marshal != nil {
		hooks.marshal = replacement.marshal
	}
	defer func() { hooks = prev }()
	fn()
}

func writeJSONAtomic(path string, v any) error {
	buf, err := hooks.marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return atomicwrite.Durable(path, append(buf, '\n'), 0o600)
}
