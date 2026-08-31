package core

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
)

type buildExplanationHandoff struct {
	View  *phaseio.ExplanationView
	State BuildExplanationState
	Error string
}

// projectBuildExplanation projects the fully reviewed, host-owned snapshot and
// makes every absence explicit. It never trusts the mutable workspace copy.
func projectBuildExplanation(projectRoot string, cs CycleState) buildExplanationHandoff {
	if cs.ExplanationDocumentationVersion == 0 {
		return buildExplanationHandoff{State: BuildExplanationLegacy}
	}
	host, active, err := explanationdocs.ActivationForCycle(projectRoot, cs.CycleID, cs.WorkspacePath)
	if err != nil {
		return buildExplanationHandoff{State: BuildExplanationInvalid, Error: compactHandoffError(err)}
	}
	if !active || host.RunID != cs.RunID || host.ContractVersion != cs.ExplanationDocumentationVersion {
		return buildExplanationHandoff{State: BuildExplanationInvalid, Error: "host Build explanation activation does not match cycle state"}
	}
	if host.Worktree == "" || host.BaseSHA == "" {
		return buildExplanationHandoff{State: BuildExplanationNotYetBuilt}
	}
	if filepath.Clean(cs.ActiveWorktree) != filepath.Clean(host.Worktree) || cs.WorktreeBaseSHA != host.BaseSHA {
		return buildExplanationHandoff{State: BuildExplanationInvalid, Error: "sealed host Build context does not match cycle state"}
	}
	view, err := explanationdocs.LoadSnapshot(host)
	if err != nil {
		return buildExplanationHandoff{State: BuildExplanationInvalid, Error: compactHandoffError(err)}
	}
	return buildExplanationHandoff{View: view, State: BuildExplanationAvailable}
}

func (h buildExplanationHandoff) apply(req *PhaseRequest) {
	req.BuildExplanation = h.View
	req.BuildExplanationState = h.State
	req.BuildExplanationError = h.Error
}

func compactHandoffError(err error) string {
	message := strings.Join(strings.Fields(err.Error()), " ")
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}

func activateBuildExplanationContract(projectRoot string, cs CycleState) error {
	binding := explanationBinding(projectRoot, cs)
	// Activation precedes Triage continuation adoption. The final worktree/base
	// are sealed separately immediately before Build dispatch.
	binding.Worktree = ""
	binding.BaseSHA = ""
	return explanationdocs.Activate(binding)
}

type resumeCheckpointIdentity struct {
	workspace string
	cycle     int
	fleet     bool
}

func authoritativeResumeIdentity(projectRoot, statePath, declaredWorkspace string) (resumeCheckpointIdentity, error) {
	if statePath == "" {
		return resumeCheckpointIdentity{workspace: declaredWorkspace}, nil
	}
	statePath, err := filepath.Abs(statePath)
	if err != nil {
		return resumeCheckpointIdentity{}, fmt.Errorf("resume identity mismatch: resolve checkpoint path: %w", err)
	}
	runsRoot, err := filepath.Abs(filepath.Join(projectRoot, ".evolve", "runs"))
	if err != nil {
		return resumeCheckpointIdentity{}, fmt.Errorf("resume identity mismatch: resolve runs root: %w", err)
	}
	workspace := filepath.Dir(statePath)
	rel, relErr := filepath.Rel(runsRoot, workspace)
	if relErr != nil || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return resumeCheckpointIdentity{workspace: declaredWorkspace}, nil // host-global, non-fleet cycle-state.json
	}
	if filepath.Base(statePath) != CycleStateFile || declaredWorkspace == "" || filepath.Clean(declaredWorkspace) != workspace {
		return resumeCheckpointIdentity{}, fmt.Errorf("resume identity mismatch: fleet checkpoint path does not match declared workspace")
	}
	name := filepath.Base(workspace)
	if !strings.HasPrefix(name, "cycle-") || strings.Contains(strings.TrimPrefix(name, "cycle-"), string(filepath.Separator)) {
		return resumeCheckpointIdentity{}, fmt.Errorf("resume identity mismatch: fleet workspace has no cycle identity")
	}
	cycle, err := strconv.Atoi(strings.TrimPrefix(name, "cycle-"))
	if err != nil || cycle <= 0 {
		return resumeCheckpointIdentity{}, fmt.Errorf("resume identity mismatch: fleet workspace has invalid cycle identity")
	}
	return resumeCheckpointIdentity{workspace: workspace, cycle: cycle, fleet: true}, nil
}

func requireResumeExplanationIdentity(projectRoot, workspace string, authoritativeCycle int, cs CycleState, checkpointCycle int) error {
	if authoritativeCycle <= 0 || checkpointCycle != 0 && checkpointCycle != authoritativeCycle {
		return fmt.Errorf("resume identity mismatch: checkpoint/cycle-state cycle does not match the authoritative host cycle")
	}
	host, active, err := explanationdocs.ActivationForCycle(projectRoot, authoritativeCycle, workspace)
	if err != nil {
		return fmt.Errorf("resume Build explanation activation: %w", err)
	}
	if !active {
		if cs.ExplanationDocumentationVersion == 0 {
			return nil
		}
		return fmt.Errorf("resume Build explanation activation: no host marker matches workspace %q", workspace)
	}
	if authoritativeCycle != host.Cycle || cs.CycleID != host.Cycle ||
		cs.RunID != host.RunID || cs.ExplanationDocumentationVersion != host.ContractVersion ||
		filepath.Clean(cs.WorkspacePath) != filepath.Clean(host.Workspace) {
		return fmt.Errorf("resume identity mismatch: checkpoint/cycle-state identity does not match host Build explanation activation")
	}
	if host.Worktree != "" && (filepath.Clean(cs.ActiveWorktree) != filepath.Clean(host.Worktree) || cs.WorktreeBaseSHA != host.BaseSHA) {
		return fmt.Errorf("resume identity mismatch: worktree/base does not match host Build explanation activation")
	}
	return explanationdocs.RequireActivation(host)
}

func sealBuildExplanationContext(projectRoot string, cs CycleState) error {
	if cs.ExplanationDocumentationVersion == 0 {
		return nil
	}
	if cs.ActiveWorktree == "" || cs.WorktreeBaseSHA == "" {
		return fmt.Errorf("active Build explanation contract requires worktree and base SHA")
	}
	return explanationdocs.SealBuild(explanationBinding(projectRoot, cs))
}

func explanationBinding(projectRoot string, cs CycleState) explanationdocs.CycleBinding {
	return explanationdocs.CycleBinding{
		ProjectRoot:     projectRoot,
		Worktree:        cs.ActiveWorktree,
		Workspace:       cs.WorkspacePath,
		BaseSHA:         cs.WorktreeBaseSHA,
		Cycle:           cs.CycleID,
		RunID:           cs.RunID,
		ContractVersion: cs.ExplanationDocumentationVersion,
	}
}

// NewExplanationLifecycleReviewer is the final reviewer in the production
// chain. It seals a Build output only after every earlier reviewer approved,
// preserving phase ownership without freezing a rejected attempt.
func NewExplanationLifecycleReviewer() DeliverableReviewer {
	return explanationLifecycleReviewer{}
}

type explanationLifecycleReviewer struct{}

func (explanationLifecycleReviewer) Review(ctx context.Context, in ReviewInput) ReviewResult {
	if in.ExplanationDocumentationVersion == 0 {
		return ReviewResult{Approve: true}
	}
	binding := explanationdocs.CycleBinding{
		ProjectRoot:     in.ProjectRoot,
		Worktree:        in.Worktree,
		Workspace:       in.Workspace,
		BaseSHA:         in.WorktreeBaseSHA,
		Cycle:           in.Cycle,
		RunID:           in.RunID,
		ContractVersion: in.ExplanationDocumentationVersion,
	}
	if in.Phase != string(PhaseBuild) {
		return ReviewResult{Approve: true}
	}
	err := explanationdocs.SealResult(ctx, binding)
	if err != nil {
		return ReviewResult{Approve: false, Retry: true, Reason: fmt.Sprintf("explanation documentation host handoff: %v", err)}
	}
	return ReviewResult{Approve: true}
}
