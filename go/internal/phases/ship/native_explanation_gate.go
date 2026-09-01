package ship

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
)

// verifyNativeExplanation is the canonical ship boundary. It deliberately
// lives below the Phase adapter so direct `evolve ship` invocations cannot
// bypass the Build explanation contract.
func verifyNativeExplanation(ctx context.Context, opts *Options) error {
	if opts.Class != ClassCycle {
		return nil
	}
	cycle := opts.CycleID
	worktree := opts.ActiveWorktree
	workspace := opts.WorkspacePath
	baseSHA := opts.WorktreeBaseSHA
	runID := opts.RunID
	version := opts.ExplanationDocumentationVersion
	if workspace == "" {
		// Standalone `evolve ship` has no PhaseRequest. Resolve from the
		// host-owned global state, never the Builder-writable run mirror.
		state, err := readStateMap(filepath.Join(opts.ProjectRoot, ".evolve", "cycle-state.json"))
		if err != nil {
			return fmt.Errorf("read host cycle identity: %w", err)
		}
		cycle, _ = stateInt(state, "cycle_id")
		version, _ = stateInt(state, "explanation_documentation_version")
		worktree = stateString(state, "active_worktree")
		workspace = stateString(state, "workspace_path")
		baseSHA = stateString(state, "worktree_base_sha")
		runID = stateString(state, "run_id")
		// Freeze the one host-owned identity snapshot that this Run will verify
		// and mutate. Later stages must not re-read mutable cycle mirrors.
		opts.CycleID = cycle
		opts.ExplanationDocumentationVersion = version
		opts.ActiveWorktree = worktree
		opts.WorkspacePath = workspace
		opts.WorktreeBaseSHA = baseSHA
		opts.RunID = runID
	}
	binding := explanationdocs.CycleBinding{
		ProjectRoot:     opts.ProjectRoot,
		Worktree:        worktree,
		Workspace:       workspace,
		BaseSHA:         baseSHA,
		Cycle:           cycle,
		RunID:           runID,
		ContractVersion: version,
	}
	// The activation belt lives in explanationdocs (single home; audit runs
	// the same check — architecture review 2026-09-01). Inactive means the
	// host AGREES this is a legacy cycle; a ship that still demands the
	// handoff keeps its refusal.
	hostActive, err := explanationdocs.CrossCheckActivation(binding)
	if err != nil {
		return err
	}
	if !hostActive {
		// Genuine legacy — the host agrees. A Require=true refusal here would
		// be dead code: ship.go derives RequireBuildExplanationHandoff from
		// version != 0, and the belt already refused every inactive+version!=0
		// identity (2026-09-01 re-review wiring proof).
		return nil
	}
	var verified *phaseio.ExplanationView
	var active bool
	if _, statErr := os.Stat(worktree); os.IsNotExist(statErr) {
		landed, idempotent, bindErr := checkPostPushIdempotency(ctx, opts)
		if bindErr != nil {
			return fmt.Errorf("verify cleaned-worktree ship binding: %w", bindErr)
		}
		if !idempotent {
			return fmt.Errorf("sealed Build worktree is unavailable without an exact post-push binding")
		}
		verified, active, err = explanationdocs.VerifyLanded(ctx, binding, landed)
	} else if statErr != nil {
		return fmt.Errorf("inspect sealed Build worktree: %w", statErr)
	} else {
		verified, active, err = explanationdocs.Verify(ctx, binding)
	}
	if err != nil {
		return err
	}
	if !active {
		return nil
	}
	if opts.RequireBuildExplanationHandoff && !explanationdocs.SameView(opts.BuildExplanation, verified) {
		return fmt.Errorf("typed Build explanation handoff does not match the verified host snapshot")
	}
	return nil
}
