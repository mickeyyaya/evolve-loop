package core

import (
	"path/filepath"
	"strconv"
)

// RunStateFile is the per-run mirror of cycle-state.json inside the run
// workspace (CB.4, concurrency campaign). The storage adapter dual-writes
// every WriteCycleState here; the worktree provisioner symlinks the cycle
// worktree's .evolve/cycle-state.json at it, so guard hooks running inside
// the worktree read the run's OWN state — under concurrent runs the global
// cycle-state.json holds whichever run wrote last.
const RunStateFile = "run.json"

// CycleStateFile is the global per-cycle state file under .evolve/. The single
// home for the filename (was a string literal repeated across storage /
// checkpoint / inboxmover / resume / reset). Every read-modify-writer of this
// file serializes on the sidecar "<dir>/cycle-state.json.lock" via
// flock.WithPathLock (ADR-0049 G7) so concurrent fleet cycles never lose each
// other's update.
const CycleStateFile = "cycle-state.json"

// RunWorkspacePath is the single source for a cycle's run-workspace
// directory: <projectRoot>/.evolve/runs/cycle-<N>. Phase artifacts, the
// tmux session registry (CB.5) and the run.json guard mirror (CB.4) all
// live here.
func RunWorkspacePath(projectRoot string, cycle int) string {
	return filepath.Join(projectRoot, ".evolve", "runs", "cycle-"+strconv.Itoa(cycle))
}
