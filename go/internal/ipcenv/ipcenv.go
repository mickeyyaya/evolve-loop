// Package ipcenv defines IPC-protocol environment variable keys used to
// coordinate evolve sub-processes across process boundaries. These constants
// are NOT operator dials (no flagregistry row); they are fixed protocol
// constants whose string values are the observable inter-process contract.
package ipcenv

const FleetKey = "EVOLVE_FLEET"                // SSOT IPC-protocol-allowed
const FleetScopeKey = "EVOLVE_FLEET_SCOPE"     // SSOT IPC-protocol-allowed
const WorktreeRootKey = "EVOLVE_WORKTREE_ROOT" // SSOT IPC-protocol-allowed

// FleetWidthKey advertises the fleet supervisor's effective lane width to each
// launched cycle, so the orchestrator can scale contention-class ship-error
// recovery budgets (shipRecoveryBudget: max(2, width+1)) with how many siblings
// are actually racing main. Read from CycleRequest.Env, never os.Getenv, so
// fleet siblings cannot leak width into each other. Unset/garbage ⇒ solo (1).
const FleetWidthKey = "EVOLVE_FLEET_WIDTH" // SSOT IPC-protocol-allowed

// CycleStateFileKey overrides the absolute path a process reads/writes cycle
// state at, replacing the host-global <evolveDir>/cycle-state.json default.
// Under the fleet supervisor each concurrent lane sets this to its OWN per-run
// file (runs/cycle-N/cycle-state.json), so two lockstep lanes never clobber a
// shared singleton's Phase/CycleID — the race that stalled a lane's phase-gate
// (guards.Phase.Decide reads cycle state) before it could reach audit. Child
// guard subprocesses inherit it, so the orchestrator and its gate checks agree
// on THIS lane's phase. Unset ⇒ host-global default (sequential loop unchanged).
const CycleStateFileKey = "EVOLVE_CYCLE_STATE_FILE" // SSOT IPC-protocol-allowed
