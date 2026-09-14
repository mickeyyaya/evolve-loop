package verdict

// classify.go — the classify step: select the authoritative verdict bytes,
// write the clean-stdout companion, run the phase's Classify, apply the ship
// guard, surface the contract violations, assemble the response and — on a
// reconciled teardown — the reconcile trail. Diagnostics ORDER is preserved
// verbatim: Classify's own → the fence → the violations → the reconcile
// warning → the ACS override.

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
)

// verdictSource is what selectVerdictBytes decided: the bytes Classify judges,
// the contract violations of an unverified contracted deliverable, and the
// re-probe count of the ladder that judged it.
type verdictSource struct {
	artifact   string
	violations []deliverable.Violation
	unverified bool
	attempts   int
}

// classify is the coordinator of the sixth step.
func (e *Engine) classify(ctx context.Context, d Dispatch, r reconciliation, classify Classify) core.PhaseResponse {
	src := e.selectVerdictBytes(ctx, d, r)
	e.writeCleanStdout(d)
	before, diags, nextPhase := classify(src.artifact)
	diags = append(diags, d.FenceDiagnostics...)
	verdict := applyShipGuard(before, src.unverified)
	diags = append(diags, violationDiagnostics(src.violations)...)
	resp := responseBase(d)
	resp.Verdict, resp.NextPhase, resp.Diagnostics = verdict, nextPhase, diags
	resp.ModelSource, resp.ResolvedModel = d.ModelSource, d.ResolvedModel
	if r.reconciled {
		e.reconcileTrail(&resp, d, r, verdict)
	}
	if src.unverified {
		e.unverifiedSignal(d, before, verdict, src)
	}
	return resp
}

// selectVerdictBytes applies the VERDICT SOURCE rule (ADR-0072 coherence).
// For a phase that HAS a deliverable contract, the on-disk report is the SOLE
// verdict source — the terminal pane (bres.Stdout) is never classified: the
// pane is bridge scrollback that can lose the real sentinel to a TUI `Write`
// collapse AND carry the contract's own prompt-echoed EXAMPLE sentinels, so
// classifying it fabricates a verdict the agent never emitted (cycle-603,
// recurring 877→921). SINGLE READ: the classified bytes ARE the verified
// bytes (Result.Content); the path is never re-read here.
//   - err != nil  → no contract (or an IO fault): the pane stays the source.
//   - res.OK      → contracted + well-formed: classify the VERIFIED bytes.
//   - !res.OK     → contracted + malformed/absent after the settle WAIT: a
//     COHERENT deliverable-production FAIL. The bytes still reach Classify (a
//     phase may derive a legitimate NON-SHIP verdict from partial content —
//     intent delta's "[intent-unchanged]" → SKIPPED); the ship guard then
//     stops a verification-FAILED deliverable from laundering a ship-eligible
//     verdict, and the contract codes are surfaced as diagnostics.
//
// The reconcile step already verified the deliverable, so its probe's bytes
// are reused instead of verifying — or reading — again. This ladder honours
// ctx (the agent exited 0; nothing more is coming).
func (e *Engine) selectVerdictBytes(ctx context.Context, d Dispatch, r reconciliation) verdictSource {
	pane := d.Bridge.Stdout
	if r.reconciled {
		return verdictSource{artifact: classifiedArtifact(r.verified, d.ArtifactPath, pane), attempts: r.attempts}
	}
	s := e.settle(ctx, identityOf(d), d.Phase, rootsFor(d))
	if s.err != nil {
		return verdictSource{artifact: pane, attempts: s.attempts}
	}
	src := verdictSource{artifact: classifiedArtifact(s.res, d.ArtifactPath, pane), attempts: s.attempts}
	if !s.res.OK {
		src.violations, src.unverified = s.res.Violations, true
	}
	return src
}

// writeCleanStdout writes the clean-stdout companion next to the raw log,
// best-effort: a failure NEVER blocks the phase — the raw log stays the
// forensic source and cyclecost / phaseobserver read it directly. A nil
// filter (the host's DisableStdoutFilter) writes nothing. The event names the
// workspace and (through Event.Phase) the phase; the companion's filename is
// the injected writer's belief (logfilter), never re-spelled here.
func (e *Engine) writeCleanStdout(d Dispatch) {
	if e.stdoutFilter == nil {
		return
	}
	if err := e.stdoutFilter(d.Workspace, d.Phase); err != nil {
		e.warn("Engine.writeCleanStdout", d, CodeStdoutFilterFailed, fmt.Sprintf("stdout filter failed: %v (the raw log stays the forensic source)", err),
			map[string]string{"workspace": d.Workspace})
	}
}

// applyShipGuard (anti-gaming): a deliverable that FAILED its well-formedness
// contract must NEVER launder a CLEAN-ship verdict past the failed contract —
// the missing-challenge-token case. PASS is the only clean-ship claim;
// downgrade it (and any non-canonical verdict) to a coherent FAIL.
// FAIL/SKIPPED/WARN pass through: FAIL and SKIPPED are non-ship, and WARN is
// NOT a clean ship — whether it ships is the orchestrator's policy call
// (workflow.strict_audit promotes WARN→FAIL there), not the runner's to
// preempt. Routing is verdict-driven, so downgrading re-routes without
// touching nextPhase.
func applyShipGuard(verdict string, unverified bool) string {
	if !unverified {
		return verdict
	}
	switch verdict {
	case core.VerdictFAIL, core.VerdictWARN, core.VerdictSKIPPED:
		return verdict
	}
	return core.VerdictFAIL
}

// violationDiagnostics surfaces the contract codes behind a coherent
// deliverable-production FAIL, so the retro/operator sees WHY the deliverable
// was rejected — empty on the happy path.
func violationDiagnostics(vs []deliverable.Violation) []core.Diagnostic {
	var out []core.Diagnostic
	for _, v := range vs {
		out = append(out, core.Diagnostic{Severity: "error", Message: fmt.Sprintf("deliverable contract violation [%s]: %s", v.Code, v.Message)})
	}
	return out
}

// reconcileTrail records that a bridge infra teardown was a red herring: the
// deliverable was well-formed (via Verify) or rescued by the ACS floor, so the
// phase COMPLETED — nil error, the agent's own Classify verdict authoritative
// (a reconciled FAIL routes as a real audit-fail, not an infra retry).
// Reconciled=true is the ledger's reconciled_timeout disposition; the ONE
// RUNNER_RECONCILED event carries the reason, emitted here with the verdict
// known. An ACS-floor rescue also surfaces the OVERRIDDEN violations on the
// response, never a silent bypass (a hygiene flag such as stray_in_worktree
// that shipped on the deterministic verdict's authority stays visible).
func (e *Engine) reconcileTrail(resp *core.PhaseResponse, d Dispatch, r reconciliation, verdict string) {
	resp.Reconciled = true
	via := "verify"
	msg := fmt.Sprintf("bridge infra teardown (%v) but deliverable %s is well-formed; reconciled to %s from the agent's own report", d.BridgeErr, d.ArtifactPath, verdict)
	if r.acsFloorRescued {
		via = "acs_floor"
		msg = fmt.Sprintf("bridge infra teardown (%v): teardown-time deliverable.Verify returned not-OK, but the acssuite verdict is ship-eligible and %s carries this cycle's challenge token with a PASS sentinel — reconciled to %s via the ACS deterministic floor (verdict-incoherence family)", d.BridgeErr, d.ArtifactPath, verdict)
	}
	resp.Diagnostics = append(resp.Diagnostics, core.Diagnostic{Severity: "warning", Message: msg})
	if r.acsFloorRescued && r.overriddenCodes != "" {
		resp.Diagnostics = append(resp.Diagnostics, core.Diagnostic{Severity: "warning", Message: fmt.Sprintf("ACS floor overrode teardown deliverable.Verify violation(s) [%s] — the deterministic acssuite verdict took precedence; investigate if any is a genuine hygiene regression (e.g. a stray worktree artifact)", r.overriddenCodes)})
	}
	e.warn("Engine.reconcileTrail", d, CodeReconciled, msg, map[string]string{
		"via": via, "verdict": verdict, "deliverable": d.ArtifactPath, "teardown": teardownKind(d.BridgeErr), "exit": strconv.Itoa(d.Bridge.ExitCode),
		"settle_attempts": strconv.Itoa(r.attempts), "overridden_codes": r.overriddenCodes,
	})
}

// unverifiedSignal is FAULT-ONLY: a contracted deliverable still not OK after
// the settle window whose FINAL verdict is FAIL — the guard downgraded a
// clean-ship verdict, or Classify itself returned FAIL on the malformed or
// absent bytes. A legitimate WARN/SKIPPED pass-through emits nothing (the
// violation codes still reach the response diagnostics).
func (e *Engine) unverifiedSignal(d Dispatch, before, after string, src verdictSource) {
	if after != core.VerdictFAIL {
		return
	}
	codes := forensicCodes(src.violations)
	e.warn("Engine.unverifiedSignal", d, CodeDeliverableUnverified,
		fmt.Sprintf("contracted deliverable %s failed verification after the settle window (%s); verdict %s → %s", d.ArtifactPath, codes, before, after),
		map[string]string{"codes": codes, "downgraded": strconv.FormatBool(before != after), "verdict_before": before, "verdict": after,
			"settle_attempts": strconv.Itoa(src.attempts), "deliverable": d.ArtifactPath})
}
