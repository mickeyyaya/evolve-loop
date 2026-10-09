package failurelog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

const LegacyEffectiveTTL = 24 * time.Hour

type PruneResult struct {
	Before  int `json:"before"`
	After   int `json:"after"`
	Removed int `json:"removed"`
}

func PruneExpired(statePath string, now time.Time) (PruneResult, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}

	state, entries, err := readStateArray(statePath, "failedApproaches")
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
	return writePruneResult(statePath, state, "failedApproaches", kept, before, "prune")
}

func readStateArray(statePath, key string) (map[string]any, []any, error) {
	raw, err := os.ReadFile(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("failurelog: read state: %w", err)
	}
	var state map[string]any
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, nil, fmt.Errorf("failurelog: parse state: %w", err)
	}
	entries, _ := state[key].([]any)
	return state, entries, nil
}

func writePruneResult(statePath string, state map[string]any, key string, kept []any, before int, errLabel string) (PruneResult, error) {
	state[key] = kept
	result := PruneResult{
		Before:  before,
		After:   len(kept),
		Removed: before - len(kept),
	}
	if result.Removed == 0 {
		return result, nil
	}
	if err := atomicWriteJSON(statePath, state); err != nil {
		return PruneResult{}, fmt.Errorf("failurelog: %s write: %w", errLabel, err)
	}
	return result, nil
}

func PruneByClassification(statePath string, classes []Classification) (PruneResult, error) {
	if len(classes) == 0 {
		return PruneResult{}, nil
	}
	target := make(map[Classification]struct{}, len(classes))
	for _, c := range classes {
		target[c] = struct{}{}
	}

	state, entries, err := readStateArray(statePath, "failedApproaches")
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
		cls, _ := m["classification"].(string)
		if cls == "" {
			kept = append(kept, m)
			continue
		}
		if _, hit := target[Classification(cls)]; hit {
			continue
		}
		kept = append(kept, m)
	}
	return writePruneResult(statePath, state, "failedApproaches", kept, before, "prune-by-class")
}

func isExpired(entry map[string]any, now time.Time) bool {
	if expiresAt, ok := entry["expiresAt"].(string); ok && expiresAt != "" {
		exp, err := time.Parse(time.RFC3339, expiresAt)
		if err != nil {
			return false
		}
		return now.After(exp)
	}
	if recordedAt, ok := entry["recordedAt"].(string); ok && recordedAt != "" {
		rec, err := time.Parse(time.RFC3339, recordedAt)
		if err != nil {
			return false
		}
		return now.After(rec.Add(LegacyEffectiveTTL))
	}
	return false
}

type PrunePreview struct {
	PruneResult
	Expired []map[string]any
}

func PreviewExpired(statePath string, now time.Time) (failed PrunePreview, carryover PrunePreview, err error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	state, entries, err := readStateArray(statePath, "failedApproaches")
	if err != nil {
		return PrunePreview{}, PrunePreview{}, err
	}
	todos, _ := state["carryoverTodos"].([]any)
	return previewEntries(entries, now), previewEntries(todos, now), nil
}

func previewEntries(entries []any, now time.Time) PrunePreview {
	expired := []map[string]any{}
	for _, e := range entries {
		if m, ok := e.(map[string]any); ok && isExpired(m, now) {
			expired = append(expired, m)
		}
	}
	return PrunePreview{
		PruneResult: PruneResult{Before: len(entries), After: len(entries) - len(expired), Removed: len(expired)},
		Expired:     expired,
	}
}
