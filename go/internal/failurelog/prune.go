package failurelog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// LegacyEffectiveTTL is the default TTL applied to legacy entries that
// have a recordedAt but no expiresAt — mirrors the bash `effective TTL
// 1 day from recordedAt` fallback in cycle-state.sh:319-327.
const LegacyEffectiveTTL = 24 * time.Hour

// PruneResult summarises the prune pass.
type PruneResult struct {
	Before  int `json:"before"`
	After   int `json:"after"`
	Removed int `json:"removed"`
}

// PruneExpired walks state.json:failedApproaches and removes entries
// whose expiresAt is in the past. Entries without expiresAt but with a
// recordedAt are treated as expired if recordedAt + LegacyEffectiveTTL
// has passed. Entries with neither timestamp are kept (true legacy
// records — no way to know their age, do not auto-delete).
//
// Returns a summary of what was pruned. Safe to call when state.json
// is missing or contains no failedApproaches — in both cases returns
// {0,0,0} and nil.
//
// statePath is typically <projectRoot>/.evolve/state.json. now is
// usually time.Now().UTC(); zero value means "use real now".
func PruneExpired(statePath string, now time.Time) (PruneResult, error) {
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

	entries, _ := state["failedApproaches"].([]any)
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
	state["failedApproaches"] = kept

	result := PruneResult{
		Before:  before,
		After:   len(kept),
		Removed: before - len(kept),
	}
	if result.Removed == 0 {
		// No change — skip the disk write so we don't churn mtime + risk
		// race-on-rename for nothing.
		return result, nil
	}
	if err := atomicWriteJSON(statePath, state); err != nil {
		return PruneResult{}, fmt.Errorf("failurelog: prune write: %w", err)
	}
	return result, nil
}

// PruneByClassification removes failedApproaches entries whose
// classification matches any of `classes`. Used by the --reset operator
// path to unblock recurring infrastructure-systemic / -transient /
// ship-gate-config accumulations. Mirrors bash dispatcher's
// archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh:749-790.
//
// Returns a PruneResult summary. No-op (rc=0, removed=0) when statePath
// missing OR no failedApproaches present OR no matches. Entries with no
// classification field are kept (true legacy records — operator must
// edit state.json directly to drop those).
func PruneByClassification(statePath string, classes []Classification) (PruneResult, error) {
	if len(classes) == 0 {
		return PruneResult{}, nil
	}
	target := make(map[Classification]struct{}, len(classes))
	for _, c := range classes {
		target[c] = struct{}{}
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

	entries, _ := state["failedApproaches"].([]any)
	if len(entries) == 0 {
		return PruneResult{}, nil
	}

	before := len(entries)
	kept := make([]any, 0, before)
	for _, e := range entries {
		m, ok := e.(map[string]any)
		if !ok {
			kept = append(kept, e) // non-object: keep
			continue
		}
		cls, _ := m["classification"].(string)
		if cls == "" {
			kept = append(kept, m) // no classification: keep
			continue
		}
		if _, hit := target[Classification(cls)]; hit {
			continue // drop
		}
		kept = append(kept, m)
	}
	state["failedApproaches"] = kept

	result := PruneResult{Before: before, After: len(kept), Removed: before - len(kept)}
	if result.Removed == 0 {
		return result, nil
	}
	if err := atomicWriteJSON(statePath, state); err != nil {
		return PruneResult{}, fmt.Errorf("failurelog: prune-by-class write: %w", err)
	}
	return result, nil
}

// isExpired returns true when entry's expiresAt < now OR the legacy
// fallback (recordedAt + LegacyEffectiveTTL < now) applies. Entries
// with neither timestamp return false — they're true legacy records
// and we don't auto-prune them.
func isExpired(entry map[string]any, now time.Time) bool {
	if expiresAt, ok := entry["expiresAt"].(string); ok && expiresAt != "" {
		exp, err := time.Parse(time.RFC3339, expiresAt)
		if err != nil {
			// Malformed expiresAt — treat as non-expired to be safe
			// (don't delete data we can't parse).
			return false
		}
		return now.After(exp)
	}
	// Legacy entry: no expiresAt. Try the recordedAt + default-TTL
	// fallback.
	if recordedAt, ok := entry["recordedAt"].(string); ok && recordedAt != "" {
		rec, err := time.Parse(time.RFC3339, recordedAt)
		if err != nil {
			return false
		}
		return now.After(rec.Add(LegacyEffectiveTTL))
	}
	// Neither timestamp present — keep.
	return false
}
