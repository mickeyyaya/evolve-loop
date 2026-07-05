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

// DefaultCarryoverBackfillTTL is the conservative TTL stamped on legacy
// carryoverTodos that carry no expiresAt. 30 days matches the CodeBuildFail/
// CodeAuditFail taxonomy the newer todos already inherit — no new classification
// heuristic, and long enough that a still-relevant legacy todo re-surfaces (and
// re-stamps fresh) before it ages out.
const DefaultCarryoverBackfillTTL = 30 * 24 * time.Hour

// BackfillLegacyCarryoverExpiry stamps expiresAt = now+defaultTTL on every
// carryoverTodos entry that lacks one, so the pre-TTL-stamping legacy population
// (entries created before the stamping fix landed, which PruneExpiredCarryoverTodos
// keeps forever because their age is unknown) becomes convergeable by the existing
// prune path. It is the one-time counterpart to that prune's "no expiresAt → keep"
// safety default — a deliberate age-assignment, not a change to the default.
//
// Idempotent: an entry that already has a non-empty expiresAt is skipped (never
// re-stamped, so a converging TTL is never pushed forward). When nothing is
// stamped the disk write is skipped (mirrors PruneExpiredCarryoverTodos — no mtime
// churn), which also makes a second pass byte-identical to the first.
//
// statePath is typically <projectRoot>/.evolve/state.json. now is usually
// time.Now().UTC(); the zero value means "use real now". Missing /
// carryoverTodos-less state is a safe no-op (0, nil) — never aborts loop boot.
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
			continue // already stamped — leave untouched (idempotent).
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

// IncrementCarryoverUnpicked bumps cycles_unpicked by exactly one on every
// carryoverTodos entry present at boot — i.e. every todo that SURVIVED a full
// cycle without being resolved. This makes the field mean what its name promises
// (a real staleness signal the advisor prompt renders) instead of a hardcoded 0.
//
// Called once per boot, ordered AFTER the prune/backfill steps so a todo removed
// this boot is not counted, and BEFORE recordFailureLearning writes any fresh
// todo (which starts at 0) — so a same-cycle-created todo is not incremented.
//
// Missing / carryoverTodos-less state is a safe no-op (0, nil) — never aborts
// loop boot.
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
		cur, _ := m["cycles_unpicked"].(float64) // JSON numbers decode to float64; absent ⇒ 0.
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
