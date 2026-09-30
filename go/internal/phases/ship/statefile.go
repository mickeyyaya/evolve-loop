package ship

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/statemap"
)

// readStateMap projects the single-source statemap.ReadStateMap under the
// short name ship's call sites already use.
func readStateMap(path string) (map[string]any, error) {
	return statemap.ReadStateMap(path)
}

// writeStateMap projects the single-source statemap.WriteStateMap under the
// short name ship's call sites already use; it stays unlocked because callers
// hold withStateLock around their own read+write.
func writeStateMap(path string, m map[string]any) error {
	return statemap.WriteStateMap(path, m)
}

func stateString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

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

// PluginVersion reads .claude-plugin/plugin.json:version; empty when the file
// or key is missing.
func PluginVersion(pluginRoot string) string { return pluginVersion(pluginRoot) }

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

// lockStateFile acquires the same `<path>.lock` advisory lock storage.UpdateState
// holds, via flock.PathLock.
// See ADR-0049.
func lockStateFile(statePath string) (release func(), err error) {
	return flock.PathLock(statePath)
}

func withStateLock(statePath string, fn func() error) error {
	release, err := lockStateFile(statePath)
	if err != nil {
		return fmt.Errorf("lock %s: %w", statePath, err)
	}
	defer release()
	return fn()
}
