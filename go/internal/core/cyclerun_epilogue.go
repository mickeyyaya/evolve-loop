package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
)

// checkpointInterruptedPhase preserves the current phase when the cycle
// context is canceled. The normal phase-boundary checkpoint names the last
// completed phase, which is deliberately conservative for crashes but would
// make a graceful operator interrupt repeat expensive completed work. A typed
// quota pause already owns a richer checkpoint and must remain authoritative.
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
	if cr.cycleCompletedNormally || errors.Is(cause, ErrAllFamiliesExhausted) ||
		(cr.ctx != nil && cr.ctx.Err() != nil) {
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
