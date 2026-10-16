package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func markerShouldAutoseal(ownerPID int, hasPID bool, alive func(int) bool) bool {
	if !hasPID {
		return true
	}
	return !alive(ownerPID)
}

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

	workspace := strFromAny(cs["workspace_path"])
	if workspace == "" {
		return SealResult{}, false, nil
	}
	lease, leaseOK, _ := runlease.Read(workspace)
	if !leaseOK {
		return SealResult{}, false, nil
	}
	if !markerShouldAutoseal(lease.OwnerPID, lease.OwnerPID != 0, alive) {
		return SealResult{}, false, nil
	}

	sealOpts := opts
	sealOpts.Force = true
	sealOpts.AutomatedRecovery = true
	res, err := SealCycle(ctx, ledger, sealOpts)
	if err != nil {
		return SealResult{}, false, err
	}
	return res, true, nil
}
