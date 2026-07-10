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
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/statemap"
)

// readStateMap parses path as JSON into a map. Missing/empty → empty map.
// Thin projection of the single-source statemap.ReadStateMap (cycle-659); the
// ship package keeps the short name its many call sites already use.
func readStateMap(path string) (map[string]any, error) {
	return statemap.ReadStateMap(path)
}

// writeStateMap atomically replaces path with the JSON of m via the
// single-source statemap.WriteStateMap (cycle-659). Callers hold withStateLock
// around read+write themselves, so this stays the UNLOCKED primitive.
func writeStateMap(path string, m map[string]any) error {
	return statemap.WriteStateMap(path, m)
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

// PluginVersion reads .claude-plugin/plugin.json:version — the SAME resolver
// verifySelfSHA uses to classify a ship-SHA mismatch as within-version
// (tampering) vs across-version (a legit bump). Exported so the boot-time
// self-SHA gate (cmd/evolve) classifies mismatches byte-identically to the
// terminal ship gate instead of duplicating the read (single source of truth).
// Empty when the file/key is missing — normal in test repos.
func PluginVersion(pluginRoot string) string { return pluginVersion(pluginRoot) }

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

// lockStateFile acquires the advisory lock that serializes state.json
// read-modify-writes — the SAME <path>.lock storage.UpdateState holds
// (ADR-0049 S2 / gap G2). flock is BLOCKING and per-open-file-description, so
// ship's map-based RMW and the typed UpdateState/allocator writers never
// interleave (the lost-update / stale-pin class). Projects through
// flock.PathLock — the single home for the "<file>.lock" sidecar suffix.
// Callers that need a phase-specific ShipError on lock failure
// use this directly + `defer release()`; the rest use withStateLock. A no-op
// for the live loop (the whole-cycle project lock already serializes ship vs
// the allocator); this joins the CA.3 lock domain so it stays correct once the
// coarse lock is scoped per-run.
func lockStateFile(statePath string) (release func(), err error) {
	return flock.PathLock(statePath)
}

// withStateLock runs fn while holding lockStateFile(statePath). fn does the
// read→modify→write between the lock and its release. A lock-acquire failure
// surfaces as a plain error the caller wraps in its phase-appropriate error.
func withStateLock(statePath string, fn func() error) error {
	release, err := lockStateFile(statePath)
	if err != nil {
		return fmt.Errorf("lock %s: %w", statePath, err)
	}
	defer release()
	return fn()
}
