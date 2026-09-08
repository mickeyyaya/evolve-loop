package core

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

// restoreResumeGoal never treats a new caller goal as permission to retarget
// an existing run. Legacy fleet checkpoints use their lane pin, never a sibling's
// mutable batch goal. A descriptive fallback keeps old closeout records valid
// while explicitly admitting that their original human goal was not persisted.
func restoreResumeGoal(req CycleRequest, cs CycleState, state State, fleet bool) (CycleRequest, error) {
	hash, goal := cs.GoalHash, cs.GoalText
	if hash == "" {
		if scope := loadLaneScope(cs.WorkspacePath); scope != nil {
			hash = scope.GoalHash
		}
	}
	if hash == "" && !fleet {
		hash = state.CurrentBatch.GoalHash
	}
	if hash != "" && req.GoalHash != "" && req.GoalHash != hash {
		return req, fmt.Errorf("resume identity mismatch: requested goal differs from checkpoint goal")
	}
	if goal != "" && req.Context["goal"] != "" && req.Context["goal"] != goal {
		return req, fmt.Errorf("resume identity mismatch: requested goal text differs from checkpoint goal")
	}
	ctx := make(map[string]string, len(req.Context)+1)
	for k, v := range req.Context {
		ctx[k] = v
	}
	req.Context = ctx
	if hash != "" {
		req.GoalHash = hash
	}
	if goal == "" {
		goal = req.Context["goal"]
	}
	if goal == "" {
		path := filepath.Join(dossier.CyclesDir(req.ProjectRoot), fmt.Sprintf("cycle-%d.json", cs.CycleID))
		if raw, err := os.ReadFile(path); err == nil {
			if d, err := dossier.ParseJSON(raw); err == nil && d.Cycle == cs.CycleID && d.RunID == cs.RunID {
				goal = d.Goal
			}
		}
	}
	if goal == "" {
		goal = req.GoalHash
	}
	if goal == "" {
		goal = fmt.Sprintf("Resume cycle %d (legacy checkpoint omitted original goal)", cs.CycleID)
		fmt.Fprintf(os.Stderr, "[resume] WARN cycle %d: legacy checkpoint has no original goal; preserving cycle identity with an explicit descriptive goal\n", cs.CycleID)
	}
	req.Context["goal"] = goal
	return req, nil
}
