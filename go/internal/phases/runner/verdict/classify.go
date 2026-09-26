package verdict

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
)

// verdictSource is what selectVerdictBytes chose, with the ladder's re-probe count.
type verdictSource struct {
	artifact   string
	violations []deliverable.Violation
	unverified bool
	attempts   int
}

// classify is the judge's second step. Diagnostics append in a pinned order:
// Classify's own, the fence's, the violations, then the reconcile trail.
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

// selectVerdictBytes applies the verdict-source rule: a contracted phase is judged on its deliverable,
// never the pane, whose scrollback can echo the contract's example sentinels. A probe error means no
// contract, so the pane stays the source; unverified bytes still reach Classify and the ship guard.
// See ADR-0072.
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

// writeCleanStdout is best-effort: a failure never blocks the phase, because the raw log stays the
// forensic source. The companion's filename belongs to the writer and is never spelled here.
func (e *Engine) writeCleanStdout(d Dispatch) {
	if e.stdoutFilter == nil {
		return
	}
	if err := e.stdoutFilter(d.Workspace, d.Phase); err != nil {
		e.warn("Engine.writeCleanStdout", d, CodeStdoutFilterFailed, fmt.Sprintf("stdout filter failed: %v (the raw log stays the forensic source)", err),
			map[string]string{"workspace": d.Workspace})
	}
}

// applyShipGuard turns a clean-ship claim (PASS or a non-canonical verdict) into FAIL when the
// deliverable failed its contract. WARN passes through: whether WARN ships is the orchestrator's policy.
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

// violationDiagnostics surfaces the contract codes behind a rejected deliverable.
func violationDiagnostics(vs []deliverable.Violation) []core.Diagnostic {
	var out []core.Diagnostic
	for _, v := range vs {
		out = append(out, core.Diagnostic{Severity: "error", Message: fmt.Sprintf("deliverable contract violation [%s]: %s", v.Code, v.Message)})
	}
	return out
}

// reconcileTrail marks a teardown the deliverable overrode; the agent's own verdict stands, so a
// reconciled FAIL routes as a real FAIL. A floor rescue lists what it overrode, never a silent bypass.
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

// unverifiedSignal is fault-only: it fires when an unverified deliverable ends in FAIL, never on a
// WARN or SKIPPED pass-through.
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
