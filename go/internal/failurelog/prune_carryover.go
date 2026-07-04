package failurelog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// PruneExpiredCarryoverTodos walks state.json:carryoverTodos and removes entries
// whose expiresAt is in the past — the structurally-parallel sibling of
// PruneExpired (failedApproaches), applied to the array that today has no removal
// path at all (65 entries / 26KB, cycles 366→506). Semantics mirror PruneExpired
// exactly (single-sourced intent via the shared isExpired oracle):
//
//   - entry.expiresAt in the past      → removed
//   - entry with NO expiresAt (legacy) → KEPT (age unknown; never delete)
//   - missing / carryoverTodos-less    → {0,0,0}, nil (safe no-op)
//
// statePath is typically <projectRoot>/.evolve/state.json. now is usually
// time.Now().UTC(); the zero value means "use real now".
func PruneExpiredCarryoverTodos(statePath string, now time.Time) (PruneResult, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}

	raw, err := os.ReadFile(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return PruneResult{}, nil
		}
		return PruneResult{}, fmt.Errorf("failurelog: read state: %w", err)
	}
	var state map[string]any
	if err := json.Unmarshal(raw, &state); err != nil {
		return PruneResult{}, fmt.Errorf("failurelog: parse state: %w", err)
	}

	entries, _ := state["carryoverTodos"].([]any)
	if len(entries) == 0 {
		return PruneResult{}, nil
	}

	before := len(entries)
	kept := make([]any, 0, before)
	for _, e := range entries {
		m, ok := e.(map[string]any)
		if !ok {
			// Non-object entry — keep as-is (don't lose un-modeled data).
			kept = append(kept, e)
			continue
		}
		if !isExpired(m, now) {
			kept = append(kept, m)
		}
	}
	state["carryoverTodos"] = kept

	result := PruneResult{
		Before:  before,
		After:   len(kept),
		Removed: before - len(kept),
	}
	if result.Removed == 0 {
		// No change — skip the disk write (mirrors PruneExpired: don't churn
		// mtime + risk race-on-rename for nothing).
		return result, nil
	}
	if err := atomicWriteJSON(statePath, state); err != nil {
		return PruneResult{}, fmt.Errorf("failurelog: prune carryover write: %w", err)
	}
	return result, nil
}
