// Package ipcenv defines IPC-protocol environment variable keys used to
// coordinate evolve sub-processes across process boundaries.
package ipcenv

import (
	"strconv"
	"strings"
)

const FleetKey = "EVOLVE_FLEET"                // SSOT IPC-protocol-allowed
const FleetScopeKey = "EVOLVE_FLEET_SCOPE"     // SSOT IPC-protocol-allowed
const WorktreeRootKey = "EVOLVE_WORKTREE_ROOT" // SSOT IPC-protocol-allowed

const FleetWidthKey = "EVOLVE_FLEET_WIDTH" // SSOT IPC-protocol-allowed

const FleetReachHolderKey = "EVOLVE_FLEET_REACH_HOLDER" // SSOT IPC-protocol-allowed

const CycleStateFileKey = "EVOLVE_CYCLE_STATE_FILE" // SSOT IPC-protocol-allowed

const TmuxSocketKey = "EVOLVE_TMUX_SOCKET" // SSOT IPC-protocol-allowed

const DispatchIDKey = "EVOLVE_DISPATCH_ID" // SSOT IPC-protocol-allowed
const MsgTokenKey = "EVOLVE_MSG_TOKEN"     // SSOT IPC-protocol-allowed

const DispatchDepthKey = "EVOLVE_DISPATCH_DEPTH" // SSOT IPC-protocol-allowed
const PhaseKey = "EVOLVE_PHASE"                  // SSOT IPC-protocol-allowed

func ProtocolKeys() []string {
	return []string{FleetKey, FleetScopeKey, FleetWidthKey, FleetReachHolderKey, WorktreeRootKey, CycleStateFileKey, TmuxSocketKey, DispatchIDKey, MsgTokenKey, DispatchDepthKey, PhaseKey}
}

func DispatchDepth(getenv func(string) string) int {
	n, err := strconv.Atoi(strings.TrimSpace(getenv(DispatchDepthKey)))
	if err != nil || n < 0 {
		return 0
	}
	return n
}
