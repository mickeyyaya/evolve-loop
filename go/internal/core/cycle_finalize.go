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

type FinalizeOptions struct {
	Now      func() time.Time
	LeaseTTL time.Duration
	PidAlive func(pid int) bool
}

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
		return false, nil
	}

	workspace := strFromAny(cs["workspace_path"])
	t := now()
	if workspace != "" {
		if lease, ok, _ := runlease.Read(workspace); ok && runlease.OwnerLive(lease, t, opts.LeaseTTL, opts.PidAlive) {
			return false, nil
		}
	}

	if err := os.Remove(csPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("cycle_finalize: remove cycle-state: %w", err)
	}
	return true, nil
}

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
