package explanationdocs

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

func CrossCheckActivation(typed CycleBinding) (bool, error) {
	markerCycle := typed.Cycle
	if derived, ok := cycleFromRunWorkspace(typed.ProjectRoot, typed.Workspace); ok {
		markerCycle = derived
	}
	host, hostActive, err := ActivationForCycle(typed.ProjectRoot, markerCycle, typed.Workspace)
	if err != nil {
		return false, fmt.Errorf("resolve host activation: %w", err)
	}
	if !hostActive {
		if typed.ContractVersion == 0 {
			return false, nil
		}
		return false, fmt.Errorf("no host activation matches the typed cycle workspace")
	}
	for _, f := range []struct {
		name     string
		mismatch bool
	}{
		{"cycle", typed.Cycle != host.Cycle},
		{"run id", typed.RunID != host.RunID},
		{"contract version", typed.ContractVersion != host.ContractVersion},
		{"workspace", !sameWorkspacePath(typed.Workspace, host.Workspace)},
		{"worktree", typed.Worktree != host.Worktree},
		{"base SHA", typed.BaseSHA != host.BaseSHA},
	} {
		if f.mismatch {
			return false, fmt.Errorf("typed %s does not match host activation", f.name)
		}
	}
	return true, nil
}

func cycleFromRunWorkspace(projectRoot, workspace string) (int, bool) {
	runsRoot := filepath.Clean(filepath.Join(projectRoot, ".evolve", "runs"))
	workspace = filepath.Clean(workspace)
	if filepath.Dir(workspace) != runsRoot {
		return 0, false
	}
	raw := strings.TrimPrefix(filepath.Base(workspace), "cycle-")
	if raw == filepath.Base(workspace) {
		return 0, false
	}
	cycle, err := strconv.Atoi(raw)
	return cycle, err == nil && cycle > 0
}

func sameWorkspacePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}
