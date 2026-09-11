package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/ledgerverify"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func (b *loopBatchCoordinator) verifySequentialCycle(cycle sequentialCycle, state *sequentialBatchState) batchDecision {
	if state.dispPolicy == dispatchPolicyOff {
		return batchDecision{flow: batchProceed}
	}
	verifyContext := ledgerverify.LoadVerifyContext(cycle.workspace, b.cfg.EvolveDir)
	if completedTriageNoWork(cycle) {
		if signals, err := router.Digest(cycle.workspace, []string{"triage"}); err == nil {
			verifyContext.EmptyTriageTermination = signals.HasEmptyTriageCommitment()
		}
	}
	verification, err := ledgerverify.VerifyCycle(
		context.Background(), b.deps.Ledger, cycle.cycle, ledgerverify.Options(verifyContext),
	)
	if err != nil {
		fmt.Fprintf(b.stderr, "[loop] verify cycle %d: %v\n", cycle.cycle, err)
		return batchDecision{flow: batchProceed}
	}
	if verification.OK {
		return batchDecision{flow: batchProceed}
	}

	var emitter *dispatchevents.Writer
	if dirExists(cycle.workspace) {
		emitter = dispatchevents.NewWriter(cycle.workspace)
		_ = emitter.EmitVerifyFailed(cycle.cycle, verification.Missing)
	}
	classification := cycleclassify.Classify(cycle.workspace)
	if emitter != nil {
		_ = emitter.EmitClassification(cycle.cycle, string(classification.Class))
	}
	fmt.Fprintf(b.stderr, "[loop] cycle %d incomplete: missing %v classification=%s\n", cycle.cycle, verification.Missing, classification.Class)

	if state.dispPolicy == dispatchPolicyStop {
		b.result.StopReason = "verify_failed_stop"
		b.result.emitFatal(b.stdout, b.stderr, b.cfg, cycle.cycle)
		return batchDecision{flow: batchReturn, exitCode: 2}
	}
	if classification.Class == cycleclassify.ClassIntegrityBreach {
		b.result.StopReason = "integrity_breach"
		b.result.emitFatal(b.stdout, b.stderr, b.cfg, cycle.cycle)
		return batchDecision{flow: batchReturn, exitCode: 2}
	}
	if classification.Marker == cycleclassify.MarkerQuotaLikelyEmptyOutput {
		return b.pauseForEmptyOutput(cycle, classification)
	}
	return b.recordRecoverableVerification(cycle, classification)
}

// completedTriageNoWork binds the shortened ledger floor to the result returned
// by the orchestrator. The artifact is revalidated by the caller, but cannot
// grant this exemption by itself.
func completedTriageNoWork(cycle sequentialCycle) bool {
	return core.IsTriageNoWorkResult(cycle.result)
}

func (b *loopBatchCoordinator) pauseForEmptyOutput(cycle sequentialCycle, classification cycleclassify.Result) batchDecision {
	fmt.Fprintf(b.stderr, "QUOTA-PAUSE: cycle=%d source=%s (empty-output session — likely subscription quota wall)\n", cycle.cycle, classification.Source)
	fmt.Fprintln(b.stderr, "[loop]   resume when quota resets: evolve loop --resume")
	if _, err := failurelog.Record(
		filepath.Join(b.cfg.EvolveDir, "state.json"),
		filepath.Join(b.cfg.ProjectRoot, ".evolve", "runs"),
		failurelog.RecordRequest{
			Cycle:          cycle.cycle,
			Classification: string(classification.Class),
			ReportPath:     filepath.Join(cycle.workspace, "orchestrator-report.md"),
			Now:            time.Now().UTC(),
		},
	); err != nil {
		fmt.Fprintf(b.stderr, "[loop] WARN: could not record quota-pause failure for cycle %d: %v\n", cycle.cycle, err)
	}
	b.result.StopReason = "quota-pause"
	b.result.emit(b.stdout)
	return batchDecision{flow: batchReturn, exitCode: 5}
}

func (b *loopBatchCoordinator) recordRecoverableVerification(cycle sequentialCycle, classification cycleclassify.Result) batchDecision {
	_, err := failurelog.Record(
		filepath.Join(b.cfg.EvolveDir, "state.json"),
		filepath.Join(b.cfg.ProjectRoot, ".evolve", "runs"),
		failurelog.RecordRequest{
			Cycle:          cycle.cycle,
			Classification: string(classification.Class),
			ReportPath:     filepath.Join(cycle.workspace, "orchestrator-report.md"),
			Now:            time.Now().UTC(),
		},
	)
	if err != nil && !errors.Is(err, failurelog.ErrStateMissing) {
		fmt.Fprintf(b.stderr, "[loop] ABORT: state.json unwritable mid-batch (cycle %d): %v\n", cycle.cycle, err)
		b.result.StopReason = "state_unwritable"
		b.result.emit(b.stdout)
		return batchDecision{flow: batchReturn, exitCode: 1}
	}
	if errors.Is(err, failurelog.ErrStateMissing) {
		fmt.Fprintf(b.stderr, "[loop] WARN: state.json missing — cannot persist failed approach for cycle %d\n", cycle.cycle)
	}
	b.result.RecoverableFailures++
	fmt.Fprintf(b.stderr, "[loop] RECOVERABLE-FAILURE recorded: cycle=%d classification=%s\n", cycle.cycle, classification.Class)
	return batchDecision{flow: batchProceed}
}
