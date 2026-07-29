package core

// cyclerun_epilogue.go — the abnormal-exit epilogue (cycle-1048): NO exit path
// may leave a started cycle without its evidence trail. The loopAbort path
// returned immediately, skipping finalizeCycle — so the seal, dossier, and
// coherence floors never ran, and every downstream detector (monitors read
// dossiers; floors run in finalize) was blind to a cycle the loop itself had
// already logged as failed. Detectors must not depend on artifacts produced
// by the failure path they monitor: visibility is guaranteed by construction
// here (a defer), not by the success of the thing being watched.

import (
	"context"
	"fmt"
	"os"
)

// abnormalEpilogue is deferred by RunCycle and fires ONLY when the cycle did
// not reach the normal closeout (cycleCompletedNormally=false). Best-effort +
// loud: it must never mask the original error, and each step tolerates the
// others failing. Uses a fresh context — the cycle's own ctx is typically
// already canceled on these paths (the cycle-1048 shape).
//
// cause is the error RunCycle is about to return — the abort's ONE
// distinguishing fact (nil on a bare bounce AND on a panic, which reaches
// this defer with retErr unset: both stay honestly Unexplained). Appended to
// the constant template it makes the digest content-bearing and
// cause-distinct; without it three distinct same-phase aborts share one
// Unexplained fingerprint and the diagnosability breaker's only move is
// halting the batch (batch-19 cycle-1208: 1197/1199/1207 all
// "build|unknown|7c02ce1f4f95").
//
// Teardown-shaped causes (IsInfraTeardownError: artifact timeout, transient
// bridge death) are marked "teardown=" instead of "cause=". Their identical
// fingerprints STAY in the identical-fingerprint population DELIBERATELY
// (review HIGH, decision pinned): one systemic infra condition mowing down
// three lanes is exactly the recurring-infra shape ADR-0072's halt doctrine
// wants stopped at the ceiling — the marker makes that shape legible in the
// halt message instead of reading as "identical defects".
func (cr *cycleRun) abnormalEpilogue(cause error) {
	if cr.cycleCompletedNormally {
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
	dossierGoal := cr.req.Context["goal"]
	if dossierGoal == "" {
		dossierGoal = cr.req.GoalHash
	}
	if derr := writeCycleDossier(cr.o.gitMutationLock, cr.req.ProjectRoot, cr.cs.WorkspacePath, cr.cycle, dossierGoal, cr.cs.RunID, VerdictFAIL,
		[]SkippedPhase{{Phase: "closeout", Reason: "abnormal exit in phase " + cr.cs.Phase}}, cr.result.SpineFailOpens); derr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d: abnormal-epilogue dossier not written: %v\n", cr.cycle, derr)
	}
	// ADR-0076 slice C (G1, cycle-1078): error-path aborts never reach
	// finalizeCycle, so the preserved worktree would carry no continuation
	// manifest and the resumption machinery would have nothing to bind. Stamp
	// here too — after the failure digest above, so FindingsPath has content.
	// Idempotent with the finalize-path stamp; no-op when no worktree exists.
	cr.o.stampContinuationManifest(epilogueCtx, cr.cs, cr.cycle, cr.req.ProjectRoot)
	// State floor: the canonical record must never claim a live phase for a
	// dead cycle (the two-hour stale phase=retro residue).
	cr.cs.Phase = "aborted"
	cr.cs.ActiveAgent = ""
	if werr := cr.o.storage.WriteCycleState(epilogueCtx, cr.cs); werr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d: abnormal-epilogue state write failed: %v\n", cr.cycle, werr)
	}
}
