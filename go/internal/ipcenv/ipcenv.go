// Package ipcenv defines IPC-protocol environment variable keys used to
// coordinate evolve sub-processes across process boundaries. These constants
// are NOT operator dials (no flagregistry row); they are fixed protocol
// constants whose string values are the observable inter-process contract.
package ipcenv

const FleetKey = "EVOLVE_FLEET"                // SSOT IPC-protocol-allowed
const FleetScopeKey = "EVOLVE_FLEET_SCOPE"     // SSOT IPC-protocol-allowed
const WorktreeRootKey = "EVOLVE_WORKTREE_ROOT" // SSOT IPC-protocol-allowed
