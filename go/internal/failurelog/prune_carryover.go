package failurelog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

func PruneExpiredCarryoverTodos(statePath string, now time.Time) (PruneResult, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}

	state, entries, err := readStateArray(statePath, "carryoverTodos")
	if err != nil {
		return PruneResult{}, err
	}
	if len(entries) == 0 {
		return PruneResult{}, nil
	}

	before := len(entries)
	kept := make([]any, 0, before)
	for _, e := range entries {
		m, ok := e.(map[string]any)
		if !ok {
			kept = append(kept, e)
			continue
		}
		if !isExpired(m, now) {
			kept = append(kept, m)
		}
	}
	return writePruneResult(statePath, state, "carryoverTodos", kept, before, "prune carryover")
}

const DefaultCarryoverBackfillTTL = 30 * 24 * time.Hour

func BackfillLegacyCarryoverExpiry(statePath string, defaultTTL time.Duration, now time.Time) (stamped int, err error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}

	raw, err := os.ReadFile(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("failurelog: read state: %w", err)
	}
	var state map[string]any
	if err := json.Unmarshal(raw, &state); err != nil {
		return 0, fmt.Errorf("failurelog: parse state: %w", err)
	}

	entries, _ := state["carryoverTodos"].([]any)
	if len(entries) == 0 {
		return 0, nil
	}

	expiry := now.Add(defaultTTL).Format(time.RFC3339)
	for _, e := range entries {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if s, _ := m["expiresAt"].(string); s != "" {
			continue
		}
		m["expiresAt"] = expiry
		stamped++
	}
	if stamped == 0 {
		return 0, nil
	}
	if err := atomicWriteJSON(statePath, state); err != nil {
		return 0, fmt.Errorf("failurelog: backfill carryover write: %w", err)
	}
	return stamped, nil
}

func IncrementCarryoverUnpicked(statePath string) (incremented int, err error) {
	raw, err := os.ReadFile(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("failurelog: read state: %w", err)
	}
	var state map[string]any
	if err := json.Unmarshal(raw, &state); err != nil {
		return 0, fmt.Errorf("failurelog: parse state: %w", err)
	}

	entries, _ := state["carryoverTodos"].([]any)
	if len(entries) == 0 {
		return 0, nil
	}

	for _, e := range entries {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		cur, _ := m["cycles_unpicked"].(float64)
		m["cycles_unpicked"] = cur + 1
		incremented++
	}
	if incremented == 0 {
		return 0, nil
	}
	if err := atomicWriteJSON(statePath, state); err != nil {
		return 0, fmt.Errorf("failurelog: increment carryover write: %w", err)
	}
	return incremented, nil
}
