// Package statemap is the single source of truth for the full-fidelity,
// lock-owning read-modify-write of state.json (and any sibling JSON-object
// state file).
//
// Three phases previously each carried their own copy of this exact
// read→mutate→atomic-write dance — ship/statefile.go (flocked),
// core/reset.go (readJSONMapFile/writeJSONMapFileAtomic) and
// phaseintegrity/repin.go (inline). They are consolidated here so exactly one
// lock-owning implementation exists (cycle-659 single-source pin).
//
// The state file is modelled as a raw map[string]any rather than a typed
// struct: state.json holds operator-owned keys no orchestrator struct models
// (expected_ship_sha, currentBatch, …) and those MUST survive a round-trip that
// touches an unrelated key. The typed adapters/storage.UpdateState is the
// complementary path for the orchestrator-modelled subset; this package is the
// full-fidelity path.
//
// statemap is a LEAF: it imports only internal/adapters/flock (the sidecar-lock
// convention) and the standard library, so core → statemap → flock is acyclic
// (the cycle-644 core→storage import-cycle trap is why storage was rejected as
// the home).
package statemap

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

// ReadStateMap parses path as a JSON object into a map. A missing or empty file
// reads as an empty (non-nil) map with no error — the first writer of a fresh
// project must not crash. A present-but-malformed file returns an error and
// nil map so callers refuse to clobber an operator's hand-broken file.
func ReadStateMap(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

// WriteStateMap atomically replaces path with the indented JSON of m via
// tmp-file + rename. The 2-space indent matches jq's default output so diffs
// against bash-written state.json files stay minimal. This is the UNLOCKED
// primitive: callers that compose several read/writes under one lock (the
// ship withStateLock pattern, the reset seal) call it inside their own held
// flock.PathLock; standalone callers use UpdateStateMap.
func WriteStateMap(path string, m map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	buf, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	buf = append(buf, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("tmp: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := writeHooks.write(tmp, buf); err != nil {
		_ = writeHooks.close(tmp)
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := writeHooks.sync(tmp); err != nil {
		_ = writeHooks.close(tmp)
		_ = os.Remove(tmpPath)
		return fmt.Errorf("sync tmp: %w", err)
	}
	if err := writeHooks.close(tmp); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close tmp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// writeHooks seams the syscalls WriteStateMap makes on the temp file so the
// white-box tests can exercise the rare write/sync/close failure branches
// deterministically (mirrors adapters/storage.writeJSONAtomic). Production
// always uses the real *os.File methods; only tests reassign these.
var writeHooks = struct {
	write func(*os.File, []byte) (int, error)
	sync  func(*os.File) error
	close func(*os.File) error
}{
	write: func(f *os.File, b []byte) (int, error) { return f.Write(b) },
	sync:  func(f *os.File) error { return f.Sync() },
	close: func(f *os.File) error { return f.Close() },
}

// UpdateStateMap performs a serialized, full-fidelity read-modify-write of
// path. It holds flock.PathLock(path) — the "<path>.lock" sidecar advisory
// lock shared with storage.UpdateState — across the WHOLE read→mutate→write, so
// concurrent writers (goroutines or processes) serialize and no update is lost.
// mutate receives the parsed map and edits it in place; it must be fast and
// side-effect-free (it runs under the cross-process lock) and must NOT call
// UpdateStateMap on the same path (the blocking flock would deadlock).
// A malformed file aborts before the write, leaving it untouched.
func UpdateStateMap(path string, mutate func(map[string]any)) error {
	return flock.WithPathLock(path, func() error {
		m, err := ReadStateMap(path)
		if err != nil {
			return err
		}
		mutate(m)
		return WriteStateMap(path, m)
	})
}
