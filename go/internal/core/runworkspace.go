package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func ResolveCycleStatePath(evolveDir string) string {
	return paths.CycleStateFileFor(evolveDir, os.Getenv(ipcenv.CycleStateFileKey))
}

const RunStateFile = "run.json"

func RunIDFromWorkspace(workspace string) string {
	if workspace == "" {
		return ""
	}
	path := filepath.Join(workspace, RunStateFile)
	b, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[core] WARN run-id resolve: read %s: %v (entry will be written unstamped and cannot be bound)\n", path, err)
		}
		return ""
	}
	var probe struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		fmt.Fprintf(os.Stderr, "[core] WARN run-id resolve: parse %s: %v (entry will be written unstamped and cannot be bound)\n", path, err)
		return ""
	}
	return probe.RunID
}

const CycleStateFile = "cycle-state.json"

func RunWorkspacePath(projectRoot string, cycle int) string {
	return paths.RunWorkspace(projectRoot, cycle)
}

func SignalStreamPath(projectRoot string, cycle int) string {
	if cycle > 0 {
		return filepath.Join(RunWorkspacePath(projectRoot, cycle), signalcenter.StreamFileName)
	}
	return filepath.Join(paths.EvolveDirOf(projectRoot), signalcenter.StreamFileName)
}
