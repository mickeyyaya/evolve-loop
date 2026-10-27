package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"
)

const LaneLaunchWaitDelay = 10 * time.Second

const defaultDeadlineStampBudget = LaneLaunchWaitDelay / 2

func (o *Orchestrator) deadlineStampBudget() time.Duration {
	if o.stampBudget > 0 {
		return o.stampBudget
	}
	return defaultDeadlineStampBudget
}

func (cr *cycleRun) checkpointInterruptedPhase(cause error) {
	if cr.ctx.Err() == nil || cr.cycleCompletedNormally || ResumeBoundaryCheckpointer == nil {
		return
	}
	if errors.Is(cause, ErrAllFamiliesExhausted) {
		return
	}
	if err := ResumeBoundaryCheckpointer(cr.cs, cr.req.ProjectRoot, cr.o.now()); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN interrupt checkpoint failed for active phase %s: %v (resume may repeat the prior completed phase)\n", cr.cs.Phase, err)
	}
}

func (cr *cycleRun) abnormalEpilogue(cause error) {
	if cr.cycleCompletedNormally || errors.Is(cause, ErrAllFamiliesExhausted) {
		return
	}
	if cr.ctx != nil && cr.ctx.Err() != nil {
		cr.deadlineStamp()
		return
	}
	epilogueCtx := context.Background()
	reason := abnormalEpilogueReason(cr.cs.Phase) + epilogueCauseSuffix(cause)
	cr.o.ensureFailureDigest(cr.cycle, cr.req.ProjectRoot, cr.cs.WorkspacePath,
		cr.cs.Phase, reason)
	cr.result.SkippedPhases = append(cr.result.SkippedPhases,
		SkippedPhase{Phase: "closeout", Reason: "abnormal exit in phase " + cr.cs.Phase})
	sealed := cr.result
	sealed.FinalVerdict, sealed.TerminationReason = VerdictFAIL, reason
	cr.emitCycleClose(sealed, "cycleRun.abnormalEpilogue")
	if derr := writeCycleDossier(cr.o.gitMutationLock, cr.dossierParams(VerdictFAIL)); derr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d: abnormal-epilogue dossier not written: %v\n", cr.cycle, derr)
	}
	cr.o.stampContinuationManifest(epilogueCtx, cr.cs, cr.cycle, cr.req.ProjectRoot)
	cr.cs.Phase = "aborted"
	cr.cs.ActiveAgent = ""
	if werr := cr.o.storage.WriteCycleState(epilogueCtx, cr.cs); werr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d: abnormal-epilogue state write failed: %v\n", cr.cycle, werr)
	}
}

func (cr *cycleRun) deadlineStamp() {
	if cr.cs.ActiveWorktree == "" {
		return
	}
	budget := cr.o.deadlineStampBudget()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(cr.ctx), budget)
	defer cancel()
	fmt.Fprintf(cr.o.errOut(), "[orchestrator] cycle %d continuation: deadline stamp after the cycle context ended (%v): salvage snapshot and stamp run detached, budget %s\n",
		cr.cycle, context.Cause(cr.ctx), budget)
	cr.o.stampContinuationManifest(ctx, cr.cs, cr.cycle, cr.req.ProjectRoot)
}

func epilogueCauseSuffix(cause error) string {
	switch {
	case cause == nil:
		return ""
	case IsInfraTeardownError(cause):
		return " teardown=" + timeoutCauseCode(cause.Error()) + causeHead(cause.Error())
	default:
		return " cause=" + causeHead(cause.Error())
	}
}

var artifactTimeoutCauseToken = regexp.MustCompile(`artifact-timeout: cause=([a-z_]+)`)

func timeoutCauseCode(errText string) string {
	m := artifactTimeoutCauseToken.FindStringSubmatch(errText)
	if m == nil {
		return ""
	}
	return m[1] + ": "
}
