package explanationdocs

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// CrossCheckActivation is the single home of the activation belt (architecture
// review 2026-09-01, HIGH): CycleBinding.ContractVersion==0 encodes BOTH
// "legacy cycle" and "a caller dropped the field", and only the host
// activation marker can tell them apart. Ship carried this cross-check inline
// while audit had none — a zero version silently disabled audit's explanation
// gate. Both gates now consult the host here.
//
// Host active: every typed field must equal the host's; any mismatch —
// including a zero ContractVersion against an active host — errors loudly.
// Host inactive: (false, nil) ONLY when typed.ContractVersion==0 — genuine
// legacy, and the host agrees. A non-zero typed version with no matching
// host activation is a stale or foreign identity and errors.
//
// The marker cycle prefers the one derived from a run workspace path
// (.evolve/runs/cycle-N) over the typed field, exactly as ship always did —
// the workspace is host-provisioned and harder to mistype than an int field.
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
	// Field-by-field so the operator's message names the divergence (the
	// realistic live trigger is a continuation base advance moving the base
	// SHA off the sealed marker). Error path returns active=false: Go's
	// convention is that non-error results are undefined under err != nil.
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

// cycleFromRunWorkspace derives the cycle number from a canonical run
// workspace path (<projectRoot>/.evolve/runs/cycle-<N>). Moved here from the
// ship gate so the belt's marker-cycle preference has one home; unexported —
// the belt is its only consumer.
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

// sameWorkspacePath is the ONE workspace-equality belief, shared by the belt
// and resolveActivation (the 2026-09-01 re-review found the two comparing
// differently: Clean vs raw — a trailing separator passed one and failed the
// other 30 lines later).
func sameWorkspacePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}
