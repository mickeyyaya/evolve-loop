package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// stale_marker_autoseal.go — auto-seal a stranded cycle-state marker whose owner
// process is dead, at loop boot (cycle 507, task wire-boot-recovery-functions).
// A crashed cycle strands a role-gated marker (e.g. phase=retro) whose role-gate
// then BLOCKS operator/inbox writes, gating even the recovery actions behind a
// manual `evolve cycle reset --force`. Boot must auto-seal such a marker when its
// owner PID is dead — REUSING the same SealCycle(Force) path (no duplicated seal
// logic, per never_duplicate_centralize_via_design_patterns).

// markerShouldAutoseal decides whether a stranded marker must be auto-sealed.
// A marker owned by a DEAD pid is sealed; a LIVE owner is left untouched (boot
// must not tear down an in-progress cycle just because it is old). A marker that
// cannot assert liveness (no usable pid) fails SAFE toward auto-seal — the alive
// probe is not even consulted. Liveness is an injected probe (kill -0 semantics)
// so callers/tests don't depend on the OS process table.
func markerShouldAutoseal(ownerPID int, hasPID bool, alive func(int) bool) bool {
	if !hasPID {
		return true // cannot assert liveness ⇒ fail safe toward seal
	}
	return !alive(ownerPID)
}

// AutosealStaleMarker seals the in-progress cycle described by
// <EvolveDir>/cycle-state.json when its owner PID is dead, clearing the role-gate
// block so the next dispatch/inbox write proceeds. It REUSES SealCycle(Force)
// (exactly one ledger append — no bespoke seal path): Force overrides the
// liveness fence because the lease heartbeat may still look fresh even though the
// owner process is gone (pid-based liveness, not heartbeat-age). Returns
// sealed=false with a nil error when the live owner must be left alone, and
// ErrNothingToReset when there is no marker to seal (so a second boot is a no-op).
func AutosealStaleMarker(ctx context.Context, ledger ledgerAppender, opts SealOptions, alive func(int) bool) (SealResult, bool, error) {
	csPath := ResolveCycleStatePath(opts.EvolveDir)
	raw, err := os.ReadFile(csPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SealResult{}, false, ErrNothingToReset
		}
		return SealResult{}, false, fmt.Errorf("autoseal: read cycle-state: %w", err)
	}
	var cs map[string]any
	if err := json.Unmarshal(raw, &cs); err != nil {
		return SealResult{}, false, fmt.Errorf("autoseal: parse cycle-state: %w", err)
	}
	if intFromAny(cs["cycle_id"]) == 0 {
		return SealResult{}, false, ErrNothingToReset
	}

	// A stranded marker heals automatically only when a run lease names its
	// owner: without a lease we cannot prove the owner is dead, so we DEFER to
	// the operator's unfinished-cycle guard (resume|reset) rather than tear the
	// cycle down. A lease present but with an unusable pid fails safe toward seal
	// (markerShouldAutoseal), matching the malformed-marker contract.
	workspace := strFromAny(cs["workspace_path"])
	if workspace == "" {
		return SealResult{}, false, nil
	}
	lease, leaseOK, _ := runlease.Read(workspace)
	if !leaseOK {
		return SealResult{}, false, nil
	}
	if !markerShouldAutoseal(lease.OwnerPID, lease.OwnerPID != 0, alive) {
		// A live owner — do not seal an in-progress cycle out from under it.
		return SealResult{}, false, nil
	}

	sealOpts := opts
	sealOpts.Force = true // override the (fresh-but-dead-owner) lease fence
	res, err := SealCycle(ctx, ledger, sealOpts)
	if err != nil {
		return SealResult{}, false, err
	}
	return res, true, nil
}
