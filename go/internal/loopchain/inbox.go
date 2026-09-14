package loopchain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// LoadChainConfig loads the chain block from paths.PolicyPath(evolveDir). Absent
// or malformed policy falls back to the built-in defaults (chaining off, the
// positive compiled cap).
func LoadChainConfig(evolveDir string) policy.ChainConfig {
	pol, err := policy.Load(paths.PolicyPath(evolveDir))
	if err != nil {
		return policy.Policy{}.ChainConfig()
	}
	return pol.ChainConfig()
}

// InboxPendingCount counts unclaimed inbox items — the `*.json` files
// directly under .evolve/inbox that actually PARSE as an inbox item.
// Lifecycle subdirectories and non-json files are not pending work and stay
// invisible. A root-level `*.json` that is not an item (truncated, 0-byte, a
// top-level array, no `id`) is returned by NAME in skipped rather than
// counted: counting it would pin pending>0 permanently and burn the chain to
// max_batches consuming nothing, and swallowing it would hide a real item
// lost to a typo. A MISSING inbox is legitimately zero pending; any other
// read error is returned so the caller stops loudly rather than chaining on
// a guess. Pure: the diagnostic lives with the Driver.
func InboxPendingCount(evolveDir string) (int, []string, error) {
	dir := filepath.Join(evolveDir, "inbox")
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil, nil
		}
		return 0, nil, fmt.Errorf("read inbox: %w", err)
	}
	n := 0
	var skipped []string
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if isInboxItemFile(filepath.Join(dir, e.Name())) {
			n++
			continue
		}
		skipped = append(skipped, e.Name())
	}
	return n, skipped, nil
}

// isInboxItemFile reports whether a root-level `*.json` is a real inbox item:
// a JSON OBJECT carrying a non-empty `id` (the field every inbox consumer
// keys on). Deliberately shallow — a stricter check would silently drop
// items the real consumers accept.
func isInboxItemFile(path string) bool {
	buf, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var doc map[string]any
	if err := json.Unmarshal(buf, &doc); err != nil {
		return false
	}
	id, _ := doc["id"].(string)
	return strings.TrimSpace(id) != ""
}

// BrakeEngaged reports whether the operator dropped the `.evolve/loop-stop`
// brake file: the chain stops at the next boundary (the in-flight batch is
// never interrupted — SIGINT does that, and still checkpoints).
func BrakeEngaged(evolveDir string) bool {
	_, err := os.Stat(paths.LoopStopPath(evolveDir))
	return err == nil
}
