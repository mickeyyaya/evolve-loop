package core

import (
	"encoding/json"
	"fmt"
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

// RunIDFromWorkspace resolves the run identity recorded in a run workspace's
// run.json mirror. It is the SINGLE resolver every out-of-process ledger writer
// uses to stamp run_id, so the identity ship's run-scoped binding lookup keys on
// has one derivation rather than one per writer.
//
// Cycle-1571 H1: PR #503 made run_id load-bearing at the ship gate (a binding
// lookup refuses an entry that is not THIS run's), on the premise that every
// recorder already stamped it. Three of the four agent_subprocess writers did
// not — they run in a separate process from the orchestrator, so the in-memory
// currentRunID that stampingLedger uses is simply unavailable to them. The run
// workspace they are already handed carries the id on disk.
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
		// anything else — a permission error, a truncated read — is a real fault
		// that would otherwise be indistinguishable from "no run id yet", which
		// is the same asserted-not-verified shape that produced this defect.
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
