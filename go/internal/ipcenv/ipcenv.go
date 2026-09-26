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

// CycleStateFileKey names a fleet lane's own per-run cycle-state file (runs/cycle-N/cycle-state.json),
// so two concurrent lanes never share <evolveDir>/cycle-state.json. It applies only to the evolve dir
// that holds it; see paths.CycleStateFileFor.
const CycleStateFileKey = "EVOLVE_CYCLE_STATE_FILE" // SSOT IPC-protocol-allowed
