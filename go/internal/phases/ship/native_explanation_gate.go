package ship

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
)

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
		opts.CycleID = cycle
		opts.AuditRound, _ = stateInt(state, "audit_dispatches")
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
	hostActive, err := explanationdocs.CrossCheckActivation(binding)
	if err != nil {
		return err
	}
	if !hostActive {
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
