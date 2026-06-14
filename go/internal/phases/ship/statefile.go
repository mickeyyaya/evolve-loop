// statefile.go — map-based read/write for .evolve/state.json and
// .evolve/cycle-state.json.
//
// The bash ship.sh uses `jq '. + {key: val}'` to mutate state.json,
// which preserves every existing field. The strongly-typed
// adapters/storage package would drop unknown fields on round-trip, so
// the ship package uses its own map-based helpers.
//
// All writes are atomic via tmp-file + rename. Mirrors the bash pattern
// `mv "$tmp" "$state"`.
package ship

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

// readStateMap parses path as JSON into a map. Missing/empty → empty map.
func readStateMap(path string) (map[string]any, error) {
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

// writeStateMap atomically replaces path with the JSON of m. 2-space
// indent matches `jq` default output so diffs against bash-written
// state.json files are minimal.
func writeStateMap(path string, m map[string]any) error {
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
	if _, err := tmp.Write(buf); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("sync tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close tmp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// stateString reads a string field from a map. Missing/non-string → "".
func stateString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// stateInt reads an int field from a map. JSON numbers decode as
// float64, so we coerce. Missing/non-numeric → 0, found=false.
func stateInt(m map[string]any, key string) (int, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	}
	return 0, false
}

// pluginVersion reads .claude-plugin/plugin.json:version. Empty when
// the file/key is missing — this is normal in test repos.
func pluginVersion(pluginRoot string) string {
	path := filepath.Join(pluginRoot, ".claude-plugin", "plugin.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var p struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return ""
	}
	return p.Version
}

// withStateLock runs fn while holding the advisory lock that serializes
// state.json read-modify-writes — the SAME <path>.lock that
// storage.UpdateState holds (ADR-0049 S2 / gap G2). flock is BLOCKING and
// per-open-file-description, so ship's map-based RMW and the typed
// UpdateState/allocator writers never interleave (the lost-update / stale-pin
// class). fn does the read→modify→write between the lock and its release. A
// no-op for the live loop (the whole-cycle project lock already serializes
// ship vs the allocator); this joins the CA.3 lock domain so it stays correct
// once that coarse lock is scoped per-run. A lock-acquire failure surfaces as
// a plain error the caller wraps in its phase-appropriate ShipError.
func withStateLock(statePath string, fn func() error) error {
	release, err := flock.Lock(statePath + ".lock")
	if err != nil {
		return fmt.Errorf("lock %s: %w", statePath, err)
	}
	defer release()
	return fn()
}
