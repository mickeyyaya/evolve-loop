package core

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// ResolveCycleStatePath returns the absolute cycle-state file path THIS process
// must read/write. Under the fleet supervisor each concurrent lane sets
// ipcenv.CycleStateFileKey to its OWN per-run file (runs/cycle-N/cycle-state.json)
// so two lockstep lanes never share the host-global singleton — the Phase/CycleID
// clobber that made a lane's phase-gate (guards.Phase reads cycle state) see the
// wrong phase and stall before audit. Unset ⇒ <evolveDir>/cycle-state.json,
// byte-identical to the sequential loop.
//
// This is the SINGLE resolver every cycle-state reader/writer MUST call
// (storage, checkpoint, resume, reset, quota-pause) so no path re-derives the
// location with a raw filepath.Join and silently reopens the isolation hole.
func ResolveCycleStatePath(evolveDir string) string {
	if p := os.Getenv(ipcenv.CycleStateFileKey); p != "" {
		return p
	}
	return filepath.Join(evolveDir, CycleStateFile)
}

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
