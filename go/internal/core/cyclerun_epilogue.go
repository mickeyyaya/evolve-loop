package core

import (
	"context"
	"errors"
	"fmt"
	"os"
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

// abnormalEpilogue is deferred by RunCycle and fires ONLY when the cycle did
// not reach the normal closeout (cycleCompletedNormally=false). A graceful
// cancellation is a resumable pause and is checkpointed separately, so it
// must not create or commit terminal failure evidence. Best-effort + loud:
// the hard-failure path must never mask the original error, and each step
// tolerates the others failing.
//
// cause is the error RunCycle is about to return — the abort's ONE
// distinguishing fact (nil on a bare bounce AND on a panic, which reaches
// this defer with retErr unset: both stay honestly Unexplained). Appended to
// the constant template it makes the digest content-bearing and
// cause-distinct; without it, distinct same-phase aborts share one
// Unexplained fingerprint and the diagnosability breaker's only move is
// halting the batch.
//
// Teardown-shaped causes (IsInfraTeardownError: artifact timeout, transient
// bridge death) are marked "teardown=" instead of "cause=". Their identical
// fingerprints stay in the identical-fingerprint population deliberately:
// one systemic infra condition mowing down several lanes is exactly the
// recurring-infra shape the halt doctrine wants stopped at the ceiling — the
// marker makes that shape legible in the halt message instead of reading as
// "identical defects".
// See ADR-0072.
func (cr *cycleRun) abnormalEpilogue(cause error) {
	// Quota exhaustion and graceful cancellation are resource/operator pauses
	// with their own checkpoints. A FAIL closeout would make a resumable cycle
	// look terminal and its dossier commit would diverge from the cycle branch.
	if cr.cycleCompletedNormally || errors.Is(cause, ErrAllFamiliesExhausted) ||
		(cr.ctx != nil && cr.ctx.Err() != nil) {
		return
	}
	epilogueCtx := context.Background()
	reason := abnormalEpilogueReason(cr.cs.Phase)
	if cause != nil {
		// causeHead: cycle-normalized + TAIL-kept truncation, so identical
		// causes across cycles share a fingerprint (recurrence countable) and
		// distinct roots under long identical prefixes never collapse. The
		// boilerplate detector is anchored to the bare template, so any
		// suffixed form is content-bearing by design.
		if IsInfraTeardownError(cause) {
			reason += " teardown=" + causeHead(cause.Error())
		} else {
			reason += " cause=" + causeHead(cause.Error())
		}
	}
	// Evidence floor: a digest for the breaker/disposition machinery even when
	// the abort predates the retro paths (idempotent with both).
	cr.o.ensureFailureDigest(cr.cycle, cr.req.ProjectRoot, cr.cs.WorkspacePath,
		cr.cs.Phase, reason)
	// Record floor: exactly one dossier per started cycle, on every path.
	// closeout genuinely did NOT run here (the cycle died mid-phase), so this is a
	// TRUE skipped_phases entry with a skip cause — distinct from the ran-but-
	// declined records the floor guard collects in result.VerdictsNotAdopted. Recorded
	// on the result first (one record, one projection: CycleResult.SkippedPhases is the
	// field the dossier projects, so it must be where the skip actually lives).
	cr.result.SkippedPhases = append(cr.result.SkippedPhases,
		SkippedPhase{Phase: "closeout", Reason: "abnormal exit in phase " + cr.cs.Phase})
	// The cycle's event stream ends with a seal on this path too — FAIL, the
	// abort reason as the termination reason — so "how did this cycle end"
	// has one answer in signals.ndjson on every path.
	// See ADR-0101.
	sealed := cr.result
	sealed.FinalVerdict, sealed.TerminationReason = VerdictFAIL, reason
	cr.emitCycleClose(sealed, "cycleRun.abnormalEpilogue")
	if derr := writeCycleDossier(cr.o.gitMutationLock, cr.dossierParams(VerdictFAIL)); derr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d: abnormal-epilogue dossier not written: %v\n", cr.cycle, derr)
	}
	// Error-path aborts never reach finalizeCycle, so the preserved worktree
	// would carry no continuation manifest for the resumption machinery to
	// bind. Stamp here too, after the failure digest above so FindingsPath
	// has content; idempotent with the finalize-path stamp, and a no-op with
	// no worktree.
	// See ADR-0076.
	cr.o.stampContinuationManifest(epilogueCtx, cr.cs, cr.cycle, cr.req.ProjectRoot)
	// State floor: the canonical record must never claim a live phase for a
	// dead cycle.
	cr.cs.Phase = "aborted"
	cr.cs.ActiveAgent = ""
	if werr := cr.o.storage.WriteCycleState(epilogueCtx, cr.cs); werr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d: abnormal-epilogue state write failed: %v\n", cr.cycle, werr)
	}
}
