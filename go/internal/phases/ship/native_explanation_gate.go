package ship

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
	markerCycle := cycle
	if derived, ok := cycleFromRunWorkspace(opts.ProjectRoot, workspace); ok {
		markerCycle = derived
	}
	host, hostActive, err := explanationdocs.ActivationForCycle(opts.ProjectRoot, markerCycle, workspace)
	if err != nil {
		return fmt.Errorf("resolve host activation: %w", err)
	}
	if hostActive {
		if cycle != host.Cycle || runID != host.RunID || version != host.ContractVersion ||
			filepath.Clean(workspace) != filepath.Clean(host.Workspace) ||
			worktree != host.Worktree || baseSHA != host.BaseSHA {
			return fmt.Errorf("typed cycle identity does not match host activation")
		}
	} else {
		if version == 0 && !opts.RequireBuildExplanationHandoff {
			return nil
		}
		return fmt.Errorf("no host activation matches the typed cycle workspace")
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
