package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
)

// resumeBootstrap is the validated persistent identity of one paused cycle.
// It is resolved while the storage lock is held and before any phase dispatch.
type resumeBootstrap struct {
	request    CycleRequest
	state      State
	cycleState CycleState
	startPhase Phase
	cycle      int
}

func (o *Orchestrator) loadResumeBootstrap(
	ctx context.Context,
	req CycleRequest,
	resumePoint *ResumePoint,
	startPhase Phase,
) (resumeBootstrap, error) {
	boot := resumeBootstrap{request: req, startPhase: startPhase}
	state, err := o.storage.ReadState(ctx)
	if err != nil {
		return boot, fmt.Errorf("read state: %w", err)
	}
	cs, err := o.storage.ReadCycleState(ctx)
	if err != nil {
		return boot, fmt.Errorf("read cycle-state: %w", err)
	}
	if cs.Phase == string(PhaseEnd) {
		return boot, fmt.Errorf("resume: cycle %d already completed", cs.CycleID)
	}

	identity, err := authoritativeResumeIdentity(req.ProjectRoot, resumePoint.StatePath, cs.WorkspacePath)
	if err != nil {
		return boot, err
	}
	hostCycle := cs.CycleID
	if allocated := max(state.LastCycleNumber, state.LastAllocatedCycleNumber); allocated > 0 {
		hostCycle = allocated
	} else if hostCycle == 0 {
		hostCycle = resumePoint.CycleID
	}
	resumeCycle := hostCycle
	if identity.fleet {
		resumeCycle = identity.cycle
	}

	recoveryBinding := cs
	if recoveryBinding.CycleID == 0 {
		recoveryBinding.CycleID = resumeCycle
	}
	if newBase, recoverable, recoveryErr := explanationdocs.RecoverRebaseSplit(ctx, explanationBinding(req.ProjectRoot, recoveryBinding)); recoveryErr != nil {
		return boot, fmt.Errorf("recover rebased Build checkpoint: %w", recoveryErr)
	} else if recoverable {
		cs.WorktreeBaseSHA = newBase
		if err := o.storage.WriteCycleState(ctx, cs); err != nil {
			return boot, fmt.Errorf("persist recovered rebased Build checkpoint: %w", err)
		}
		boot.startPhase = PhaseBuild
	}
	if err := requireResumeExplanationIdentity(req.ProjectRoot, identity.workspace, resumeCycle, cs, resumePoint.CycleID); err != nil {
		return boot, err
	}
	if cs.CycleID != 0 && resumePoint.CycleID != 0 && cs.CycleID != resumePoint.CycleID {
		return boot, fmt.Errorf("resume identity mismatch: cycle-state cycle %d does not match checkpoint cycle %d", cs.CycleID, resumePoint.CycleID)
	}
	if cs.ActiveWorktree != "" && resumePoint.WorktreePath != "" && filepath.Clean(cs.ActiveWorktree) != filepath.Clean(resumePoint.WorktreePath) {
		return boot, fmt.Errorf("resume identity mismatch: cycle-state worktree %q does not match checkpoint worktree %q", cs.ActiveWorktree, resumePoint.WorktreePath)
	}

	req, err = restoreResumeGoal(req, cs, state, identity.fleet)
	if err != nil {
		return boot, err
	}
	cs.GoalHash, cs.GoalText = req.GoalHash, req.Context["goal"]
	cycle := cs.CycleID
	if cycle == 0 {
		cycle = resumePoint.CycleID
	}
	if cs.RunID == "" {
		cs.RunID = MintRunID(o.now())
	}

	boot.request = req
	boot.state = state
	boot.cycleState = cs
	boot.cycle = cycle
	return boot, nil
}

type resumeInputs struct {
	env               map[string]string
	context           map[string]string
	result            CycleResult
	preResumeHEAD     string
	mainDirtyBaseline map[string]bool
}

// snapshotResumeInputs copies caller-owned maps and restores derived runtime
// inputs after the run identity and lease have been activated.
func (o *Orchestrator) snapshotResumeInputs(ctx context.Context, boot *resumeBootstrap) resumeInputs {
	req := boot.request
	cs := &boot.cycleState

	envSnap := make(map[string]string, len(req.Env)+1)
	for key, value := range req.Env {
		envSnap[key] = value
	}
	// SSOT IPC-protocol-allowed: resume bootstrap → resumed cycle handoff.
	envSnap["EVOLVE_"+"RESUME_MODE"] = "1"
	ctxSnap := make(map[string]string, len(req.Context))
	for key, value := range req.Context {
		ctxSnap[key] = value
	}
	if scope := loadLaneScope(cs.WorkspacePath); scope != nil {
		ctxSnap["fleet_scope"] = strings.Join(scope.TodoIDs, ",")
	}

	result := CycleResult{Cycle: boot.cycle, FinalVerdict: o.resumeFinalVerdict(*cs)}
	cs.FinalVerdict = result.FinalVerdict
	preResumeHEAD := cs.PreCycleHEAD
	if preResumeHEAD == "" {
		preResumeHEAD, _ = o.gitHEAD()
		cs.PreCycleHEAD = preResumeHEAD
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN resume cycle %d: legacy checkpoint has no pre-cycle HEAD; earlier ship throughput cannot be reconstructed from this checkpoint\n", boot.cycle)
	}
	return resumeInputs{
		env:               envSnap,
		context:           ctxSnap,
		result:            result,
		preResumeHEAD:     preResumeHEAD,
		mainDirtyBaseline: porcelainDirtySet(ctx, req.ProjectRoot),
	}
}
