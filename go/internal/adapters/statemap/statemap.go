// Package statemap is the single source of truth for the full-fidelity,
// lock-owning read-modify-write of state.json (and any sibling JSON-object
// state file).
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

func ResolveWriteTarget(path string) string {
	const maxDepth = 8
	cur := path
	for i := 0; i < maxDepth; i++ {
		fi, err := os.Lstat(cur)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			return cur
		}
		dst, err := readlink(cur)
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

func todosLen(m map[string]any) int {
	if arr, ok := m["carryoverTodos"].([]any); ok {
		return len(arr)
	}
	return -1
}

func WriteStateMap(path string, m map[string]any) error {
	path = ResolveWriteTarget(path)
	if onDisk, err := ReadStateMap(path); err == nil {
		for _, key := range []string{"stateRevision", statemapRevisionKey} {
			if diskRev, ok := counterOf(onDisk, key); ok {
				if inRev, ok := counterOf(m, key); ok && inRev < diskRev {
					return fmt.Errorf("%w (%s: incoming %v < on-disk %v at %s)", ErrStaleRevision, key, inRev, diskRev, path)
				}
			}
		}
		if oldN, newN := todosLen(onDisk), todosLen(m); oldN > carryoverShrinkWarnFloor && (newN == -1 || newN < oldN/2) {
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

var readlink = os.Readlink

var writeHooks = struct {
	write func(*os.File, []byte) (int, error)
	sync  func(*os.File) error
	close func(*os.File) error
}{
	write: func(f *os.File, b []byte) (int, error) { return f.Write(b) },
	sync:  func(f *os.File) error { return f.Sync() },
	close: func(f *os.File) error { return f.Close() },
}

func UpdateStateMap(path string, mutate func(map[string]any)) error {
	path = ResolveWriteTarget(path)
	return flock.WithPathLock(path, func() error {
		m, err := ReadStateMap(path)
		if err != nil {
			return err
		}
		mutate(m)
		if rev, ok := counterOf(m, statemapRevisionKey); ok {
			m[statemapRevisionKey] = rev + 1
		} else {
			m[statemapRevisionKey] = float64(1)
		}
		m["lastUpdated"] = nowFn().UTC().Format(time.RFC3339)
		return WriteStateMap(path, m)
	})
}

var nowFn = time.Now

const statemapRevisionKey = "statemapRevision"

const carryoverShrinkWarnFloor = 20

var ErrStaleRevision = errors.New("statemap: stale stateRevision — on-disk state is newer; re-read before writing")
