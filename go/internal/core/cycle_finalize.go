package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// cycle_finalize.go — S2 (workspace-hygiene-2026-07 plan): the clean-exit
// counterpart to SealCycle. Where SealCycle ABANDONS a stuck cycle (archive +
// faillearn lesson + ledger entry), ClearCompletedCycleMarker clears a
// terminal-but-not-yet-cleared on-disk cycle-state.json marker left behind by
// a normal `max_cycles` batch exit — SILENTLY, so a healthy exit never
// poisons failure-learning the way SealCycle's abandon semantics would.

// FinalizeOptions configures ClearCompletedCycleMarker. Now/LeaseTTL/PidAlive
// mirror the same-named SealOptions fields — the liveness fence must agree
// with SealCycle's.
type FinalizeOptions struct {
	// Now defaults to time.Now.
	Now func() time.Time
	// LeaseTTL overrides the liveness-fence freshness window; 0 = runlease.DefaultTTL.
	LeaseTTL time.Duration
	// PidAlive probes whether the lease's owner process is still running. See
	// runlease.OwnerLive; nil falls back to freshness-only.
	PidAlive func(pid int) bool
}

// ClearCompletedCycleMarker removes <evolveDir>/cycle-state.json (via
// ResolveCycleStatePath, the fleet-safe resolver) when the marker describes a
// COMPLETED cycle (cycle_id <= state.json's lastCycleNumber) with no live
// owner. It never mutates state.json, never archives, and never writes a
// faillearn lesson — those are SealCycle's abandon-only responsibilities.
//
// cleared=false, err=nil covers every case where the marker must be left
// alone: absent, in-progress (cycle_id > lastCycleNumber), or owned by a live
// run lease.
func ClearCompletedCycleMarker(evolveDir string, opts FinalizeOptions) (bool, error) {
	now := opts.Now
	if now == nil {
		now = time.Now
	}

	csPath := ResolveCycleStatePath(evolveDir)
	raw, err := os.ReadFile(csPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("cycle_finalize: read cycle-state: %w", err)
	}
	var cs map[string]any
	if err := json.Unmarshal(raw, &cs); err != nil {
		return false, fmt.Errorf("cycle_finalize: parse cycle-state: %w", err)
	}
	cycleID := intFromAny(cs["cycle_id"])
	if cycleID == 0 {
		return false, nil
	}

	lastCycleNumber, err := readLastCycleNumber(evolveDir)
	if err != nil {
		return false, fmt.Errorf("cycle_finalize: read state.json: %w", err)
	}
	if cycleID > lastCycleNumber {
		return false, nil // still in progress (or crashed) — resumable
	}

	workspace := strFromAny(cs["workspace_path"])
	t := now()
	if workspace != "" {
		if lease, ok, _ := runlease.Read(workspace); ok && runlease.OwnerLive(lease, t, opts.LeaseTTL, opts.PidAlive) {
			return false, nil // a live owner still holds this cycle
		}
	}

	if err := os.Remove(csPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("cycle_finalize: remove cycle-state: %w", err)
	}
	return true, nil
}

// readLastCycleNumber reads state.json's lastCycleNumber field directly (not
// via the typed storage adapter) so ClearCompletedCycleMarker stays a plain
// file-based primitive, independent of any in-memory storage seam. Missing
// state.json reads as 0 (never treats an in-progress marker as completed).
func readLastCycleNumber(evolveDir string) (int, error) {
	raw, err := os.ReadFile(filepath.Join(evolveDir, "state.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	var sm map[string]any
	if err := json.Unmarshal(raw, &sm); err != nil {
		return 0, err
	}
	return intFromAny(sm["lastCycleNumber"]), nil
}
