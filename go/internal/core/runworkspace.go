package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

// ResolveCycleStatePath is the cycle-state file this process reads and writes for evolveDir: a fleet
// lane's own per-run file when its override lies inside evolveDir, else <evolveDir>/cycle-state.json.
// Every cycle-state reader and writer calls it; see paths.CycleStateFileFor.
func ResolveCycleStatePath(evolveDir string) string {
	return paths.CycleStateFileFor(evolveDir, os.Getenv(ipcenv.CycleStateFileKey))
}

// RunStateFile is the per-run mirror of cycle-state.json inside the run
// workspace. The storage adapter dual-writes every WriteCycleState here; the
// worktree provisioner symlinks the cycle worktree's .evolve/cycle-state.json
// at it, so guard hooks running inside the worktree read the run's OWN state
// — under concurrent runs the global cycle-state.json holds whichever run
// wrote last.
const RunStateFile = "run.json"

// RunIDFromWorkspace resolves the run identity recorded in a run workspace's
// run.json mirror. It is the SINGLE resolver every out-of-process ledger writer
// uses to stamp run_id, so the identity ship's run-scoped binding lookup keys on
// has one derivation rather than one per writer.
//
// Some agent-subprocess writers run in a separate process from the
// orchestrator, so the in-memory currentRunID that stampingLedger uses is
// simply unavailable to them; the run workspace they are already handed
// carries the id on disk instead.
//
// Fail-SOFT by design: an unresolvable id returns "" and the caller OMITS the
// field rather than stamping an empty identity. The fail-CLOSED half belongs to
// the consumer — ship refuses to bind an unstamped entry — and inventing or
// zero-filling an identity here would defeat exactly that.
func RunIDFromWorkspace(workspace string) string {
	if workspace == "" {
		return ""
	}
	path := filepath.Join(workspace, RunStateFile)
	b, err := os.ReadFile(path)
	if err != nil {
		// A missing run.json is legitimate (standalone dispatch outside a cycle);
		// any other read error is a real fault, not "no run id yet".
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

// CycleStateFile is the global per-cycle state file under .evolve/, the
// single home for the filename. Every read-modify-writer of this file
// serializes on the sidecar "<dir>/cycle-state.json.lock" via
// flock.WithPathLock so concurrent fleet cycles never lose each other's
// update.
// See ADR-0049.
const CycleStateFile = "cycle-state.json"

// RunWorkspacePath is core's single spelling of a cycle's run-workspace
// directory, <projectRoot>/.evolve/runs/cycle-<N> — a projection of
// paths.RunWorkspace, the layout's one home. Phase artifacts, the tmux
// session registry and the run.json guard mirror all live here.
// See ADR-0103.
func RunWorkspacePath(projectRoot string, cycle int) string {
	return paths.RunWorkspace(projectRoot, cycle)
}
