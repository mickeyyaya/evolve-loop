package runner

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// classifyPhaseOutcome selects the authoritative verdict bytes, applies the
// deliverable ship guard, and assembles the public phase response.
func (b *BaseRunner) classifyPhaseOutcome(
	ctx context.Context,
	req core.PhaseRequest,
	prep phasePreparation,
	resolved phaseDispatchPlan,
	dispatch phaseDispatchResult,
	reconciliation phaseReconciliation,
) (core.PhaseResponse, error) {
	phase := prep.phase
	artifactPath := prep.artifactPath
	bres := dispatch.bridgeResponse
	bridgeErr := dispatch.bridgeErr
	durationMS := dispatch.durationMS
	resolvedModel := dispatch.resolvedModel
	fenceDiags := dispatch.fenceDiagnostics
	modelSource := resolved.modelSource
	reconciled := reconciliation.reconciled
	reconciledRes := reconciliation.verifiedResult
	acsFloorRescued := reconciliation.acsFloorRescued
	acsFloorOverriddenCodes := reconciliation.acsFloorOverriddenCodes

	artifact := bres.Stdout
	// VERDICT SOURCE (ADR-0072 coherence). For a phase that HAS a deliverable contract,
	// the on-disk report is the SOLE verdict source — the terminal pane (bres.Stdout) is
	// never classified. The pane is bridge scrollback: it can lose the real verdict
	// sentinel to a TUI `Write` collapse AND carry the Deliverable Contract's own
	// prompt-echoed EXAMPLE sentinels, so classifying it fabricates a verdict the agent
	// never emitted — a real PASS recorded as FAIL (cycle-603, then recurring 877→921
	// ≥10× on one goal_hash). The earlier "prefer the file when it verifies, else fall
	// back to the pane" design left that fabricated verdict reachable under any flush-
	// timing pressure; widening the settle window only lowered the odds. This removes the
	// pane from the contracted-verdict path entirely, so timing can no longer cause
	// incoherence — only latency.
	//
	// SINGLE READ (deliverable-verified-bytes-single-read): the classified bytes ARE the
	// verified bytes. verifyFn returns the content it judged (deliverable.Result.Content)
	// and this block consumes THAT — it never re-reads artifactPath. The earlier
	// verify-then-re-read pair left a window in which a process racing the just-finished
	// launch could swap the file, so the recorded verdict could belong to content no gate
	// ever checked; the invariant is now literal ("the file", not "the file as of the
	// Verify read"). classifiedArtifact owns the one-line decision — including the two
	// cases where the verified bytes are not this artifact's (a NoArtifact contract, and a
	// phase whose dispatched filename differs from its contract's); see its doc.
	//
	//   - verr != nil  → no contract for this phase (or an IO fault): well-formedness is
	//     undeterminable, so the pane/Classify remains the legitimate source. UNCHANGED.
	//   - res.OK       → contracted + well-formed: classify the VERIFIED bytes. Anti-gaming
	//     intact — those bytes passed Verify's challenge-token + section + ADR-0039 checks.
	//   - !res.OK      → contracted + malformed/absent after the settle WAIT: a COHERENT
	//     deliverable-production FAIL. The verified bytes still reach Classify (a phase may
	//     derive a legitimate NON-SHIP verdict from partial content — intent delta's
	//     "[intent-unchanged]" → SKIPPED); an ABSENT deliverable verifies as empty content,
	//     so Classify sees no sentinel → FAIL. The ship-guard below then stops a
	//     verification-FAILED deliverable from laundering a ship-eligible verdict, and the
	//     contract Codes are surfaced as diagnostics.
	//
	// The reconcile-on-teardown path already verified the deliverable above, so it reuses
	// that probe's bytes (reconciledRes) instead of verifying — or reading — again.
	var deliverableViolations []deliverable.Violation
	deliverableUnverified := false
	if reconciled {
		artifact = classifiedArtifact(reconciledRes, artifactPath, artifact)
	} else {
		roots := phasecontract.Roots{Workspace: req.Workspace, Worktree: req.Worktree, DispatchedArtifact: artifactPath, ExplanationDocumentationVersion: req.ExplanationDocumentationVersion}
		if req.ProjectRoot != "" {
			roots.EvolveDir = filepath.Join(req.ProjectRoot, ".evolve")
		}
		res, verr := b.verifyReconcileDeliverable(ctx, phase, roots)
		switch {
		case verr != nil:
			// Uncontracted phase (or IO fault): keep the pane as the verdict source.
		default:
			// Contracted phase → classify the verified bytes, never the pane.
			artifact = classifiedArtifact(res, artifactPath, artifact)
			if !res.OK {
				deliverableViolations = res.Violations
				deliverableUnverified = true
			}
		}
	}

	// Best-effort: write the <phase>-stdout.clean.txt companion next to
	// the raw log. Default-on; set Options.DisableStdoutFilter=true to skip.
	// Filter failures NEVER block the phase — they WARN and continue,
	// because the raw log remains the forensic source of truth and
	// cyclecost / phaseobserver still read it directly.
	if !b.disableStdoutFilter {
		if err := b.stdoutFilter(req.Workspace, phase); err != nil {
			log.Diag().Warnf("[runner] WARN stdout filter phase=%s: %v\n", phase, err)
		}
	}

	verdict, diags, nextPhase := b.hooks.Classify(artifact, req, bres)
	diags = append(diags, fenceDiags...)
	if deliverableUnverified {
		// SHIP-GUARD (anti-gaming). A deliverable that FAILED its well-formedness/anti-
		// gaming contract must NEVER launder a CLEAN-ship verdict past the failed contract —
		// the CodeMissingChallengeToken case (a PASS sentinel in a file that never echoed the
		// per-cycle challenge token). PASS is the only clean-ship claim; downgrade it (and any
		// non-canonical verdict) to a coherent FAIL. FAIL/SKIPPED/WARN pass through: FAIL and
		// SKIPPED are non-ship, and WARN is NOT a clean ship — it already flags issues, and
		// whether it ships is an orchestrator policy call (workflow.strict_audit promotes
		// WARN→FAIL there), not the runner's to preempt. Downgrading WARN here would break
		// fluent-mode WARN-ships (TestE2EPipeline_AuditWarn_FluentShips) and regress origin/main,
		// which also ships a failed-verify WARN via the pane. Routing is verdict-driven (cyclerun
		// maps resp.Verdict → FinalVerdict/lastVerdict), so downgrading re-routes without
		// touching nextPhase.
		switch verdict {
		case core.VerdictFAIL, core.VerdictWARN, core.VerdictSKIPPED:
		default:
			verdict = core.VerdictFAIL
		}
	}
	// Surface the contract Codes behind a coherent deliverable-production FAIL, so the
	// retro/operator sees WHY the deliverable was rejected — not a bare verdict with no
	// trail. Only populated when a CONTRACTED deliverable failed verification (the
	// !res.OK branch above); empty otherwise, so this is a no-op on the happy path.
	for _, v := range deliverableViolations {
		diags = append(diags, core.Diagnostic{
			Severity: "error",
			Message:  fmt.Sprintf("deliverable contract violation [%s]: %s", v.Code, v.Message),
		})
	}

	resp := core.PhaseResponse{
		Phase:         phase,
		Verdict:       verdict,
		ArtifactsDir:  req.Workspace,
		NextPhase:     nextPhase,
		CostUSD:       bres.CostUSD,
		Tokens:        bres.Tokens,
		DurationMS:    durationMS,
		BootMS:        bres.BootMS,
		Diagnostics:   diags,
		ModelSource:   modelSource,
		ResolvedModel: resolvedModel,
	}
	if reconciled {
		// A well-formed deliverable on a bridge infra teardown (timeout OR
		// transient) means the phase actually COMPLETED — the teardown was a red
		// herring (the bridge gave up on the wait window, or hit a transient
		// exit, just as, or after, the agent finished writing). So we treat it
		// exactly like a normal completed phase: nil error, the agent's own
		// Classify verdict authoritative. A reconciled FAIL therefore routes as a
		// real code-audit-fail (→ retro), NOT an infra-teardown retry — which is
		// both correct classification and avoids re-running a finished phase.
		// Reconciliation only ever upgrades a synthesized FAIL toward the agent's
		// real verdict; it never invents a PASS (Classify, incl. audit's EGPS
		// red_count gate, still decides).
		resp.Reconciled = true
		reconcileMsg := fmt.Sprintf("bridge infra teardown (%v) but deliverable %s is well-formed; reconciled to %s from the agent's own report", bridgeErr, artifactPath, verdict)
		if acsFloorRescued {
			// The deliverable did NOT pass teardown-time Verify — it was rescued by the
			// deterministic ACS floor. Record that accurately so the trail isn't misleading.
			reconcileMsg = fmt.Sprintf("bridge infra teardown (%v): teardown-time deliverable.Verify returned not-OK, but the acssuite verdict is ship-eligible and %s carries this cycle's challenge token with a PASS sentinel — reconciled to %s via the ACS deterministic floor (verdict-incoherence family)", bridgeErr, artifactPath, verdict)
		}
		resp.Diagnostics = append(resp.Diagnostics, core.Diagnostic{
			Severity: "warning",
			Message:  reconcileMsg,
		})
		if acsFloorRescued && acsFloorOverriddenCodes != "" {
			// Surface the OVERRIDDEN well-formedness violations on the RESPONSE (not just the
			// log), so the ACS-floor rescue is never a silent bypass: a hygiene flag such as
			// stray_in_worktree that shipped on the deterministic verdict's authority stays
			// visible to the operator/retro for investigation. Not a downgrade — routing keys
			// on resp.Verdict (PASS); this is an informational trail only.
			resp.Diagnostics = append(resp.Diagnostics, core.Diagnostic{
				Severity: "warning",
				Message:  fmt.Sprintf("ACS floor overrode teardown deliverable.Verify violation(s) [%s] — the deterministic acssuite verdict took precedence; investigate if any is a genuine hygiene regression (e.g. a stray worktree artifact)", acsFloorOverriddenCodes),
			})
		}
		log.Diag().Infof("[runner] RECONCILED phase=%s (%v) verdict=%s deliverable=%s acsFloor=%v\n", phase, bridgeErr, verdict, artifactPath, acsFloorRescued)
	}
	return resp, nil
}
