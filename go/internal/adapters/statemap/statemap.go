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
	"time"

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

// resolveWriteTarget follows a symlink chain (bounded, dangling-tolerant) to
// the FINAL write target. Rename-over-a-symlink replaces the LINK with a
// regular file — exactly how worktree .evolve/state.json links to canonical
// were severed and mutations stranded in detached copies (cycle-999/1000).
// Writing through to the resolved target keeps the link intact and every
// mutation visible on the canonical file.
func resolveWriteTarget(path string) string {
	const maxDepth = 8
	cur := path
	for i := 0; i < maxDepth; i++ {
		fi, err := os.Lstat(cur)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			return cur // regular file, missing (dangling tail), or unreadable
		}
		dst, err := os.Readlink(cur)
		if err != nil {
			return cur
		}
		if !filepath.IsAbs(dst) {
			dst = filepath.Join(filepath.Dir(cur), dst)
		}
		cur = dst
	}
	return cur
}

// counterOf extracts a numeric counter key from m (JSON round-trips give
// float64; in-process maps may carry int). ok=false when absent/non-numeric.
func counterOf(m map[string]any, key string) (float64, bool) {
	switch v := m[key].(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	}
	return 0, false
}

// todosLen returns len(carryoverTodos) when present as an array, else -1.
func todosLen(m map[string]any) int {
	if arr, ok := m["carryoverTodos"].([]any); ok {
		return len(arr)
	}
	return -1
}

// WriteStateMap atomically replaces path with the indented JSON of m via
// tmp-file + rename. The 2-space indent matches jq's default output so diffs
// against bash-written state.json files stay minimal. This is the UNLOCKED
// primitive: callers that compose several read/writes under one lock (the
// ship withStateLock pattern, the reset seal) call it inside their own held
// flock.PathLock; standalone callers use UpdateStateMap.
//
// Two integrity floors (cycle-1001 lost-write / cycle-999 stranded-write):
//   - symlink write-through: the rename lands on resolveWriteTarget(path), so
//     a worktree's state.json link to canonical survives and the bytes reach
//     the live file;
//   - stateRevision CAS: when both the incoming map and the on-disk target
//     carry a numeric stateRevision, an incoming revision BELOW the on-disk
//     one is a stale writer and is refused with ErrStaleRevision — the lock
//     serializes writers, this floor validates their freshness.
//
// A forensic tripwire WARNs (never blocks) when a write would shrink
// carryoverTodos by more than half from >20 entries — the cycle-1001
// signature — so a legal-but-suspicious mass drop is loud in the log.
func WriteStateMap(path string, m map[string]any) error {
	path = resolveWriteTarget(path)
	if onDisk, err := ReadStateMap(path); err == nil {
		// CAS floor on BOTH lineage counters: stateRevision is
		// storage.UpdateState's EXCLUSIVE OCC audit trail (cyclestate CA.3 —
		// statemap never bumps it, only compares), statemapRevision is this
		// package's own counter bumped by UpdateStateMap. A stale snapshot is
		// stale in whichever lineage it aged out of.
		for _, key := range []string{"stateRevision", statemapRevisionKey} {
			if diskRev, ok := counterOf(onDisk, key); ok {
				if inRev, ok := counterOf(m, key); ok && inRev < diskRev {
					return fmt.Errorf("%w (%s: incoming %v < on-disk %v at %s)", ErrStaleRevision, key, inRev, diskRev, path)
				}
			}
		}
		// oldN>20 with the key deleted (newN==-1) counts as a shrink-to-zero.
		if oldN, newN := todosLen(onDisk), todosLen(m); oldN > 20 && (newN == -1 || newN < oldN/2) {
			fmt.Fprintf(os.Stderr, "[statemap] WARN: write shrinks carryoverTodos %d -> %d at %s (cycle-1001 signature; allowed, but verify the writer)\n", oldN, newN, path)
		}
	}
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
// path. The path is symlink-RESOLVED first and the lock is taken on the
// RESOLVED path's sidecar — a worktree writer (through the .evolve/state.json
// link) and a canonical writer therefore share ONE lock instead of racing on
// two different lock files over the same data (the cross-tree half of the
// cycle-1001 lost-write). It holds flock.PathLock across the WHOLE
// read→mutate→write, so concurrent writers (goroutines or processes)
// serialize and no update is lost.
//
// Every successful update advances stateRevision (seeded to 1 when absent)
// and refreshes lastUpdated — the frozen-timestamp forensics of cycle-1001
// showed some writers skipped both, making stale snapshots indistinguishable
// from live state. mutate receives the parsed map and edits it in place; it
// must be fast and side-effect-free (it runs under the cross-process lock)
// and must NOT call UpdateStateMap on the same path (the blocking flock would
// deadlock). A malformed file aborts before the write, leaving it untouched.
func UpdateStateMap(path string, mutate func(map[string]any)) error {
	path = resolveWriteTarget(path)
	return flock.WithPathLock(path, func() error {
		m, err := ReadStateMap(path)
		if err != nil {
			return err
		}
		mutate(m)
		// Bump statemap's OWN lineage counter. stateRevision is deliberately
		// untouched: it is storage.UpdateState's exclusive CA.3 audit trail
		// ("a gap/repeat betrays a bypassing writer" — core/alloc.go even
		// restores it before overwrites so only UpdateState's ++ moves it).
		if rev, ok := counterOf(m, statemapRevisionKey); ok {
			m[statemapRevisionKey] = rev + 1
		} else {
			m[statemapRevisionKey] = float64(1)
		}
		m["lastUpdated"] = nowFn().UTC().Format(time.RFC3339)
		return WriteStateMap(path, m)
	})
}

// nowFn seams time for tests.
var nowFn = time.Now

// statemapRevisionKey is statemap's own write-lineage counter — namespaced so
// it can never be confused with storage.UpdateState's exclusive stateRevision
// OCC audit trail (two independent counters, one CAS floor over both).
const statemapRevisionKey = "statemapRevision"

// ErrStaleRevision is returned when a write carries a lineage counter below
// the on-disk value — a stale writer that must re-read, never clobber.
var ErrStaleRevision = errors.New("statemap: stale stateRevision — on-disk state is newer; re-read before writing")
