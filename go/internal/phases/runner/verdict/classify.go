package verdict

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
)

type verdictSource struct {
	artifact   string
	violations []deliverable.Violation
	unverified bool
	attempts   int
}

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

func (e *Engine) writeCleanStdout(d Dispatch) {
	if e.stdoutFilter == nil {
		return
	}
	if err := e.stdoutFilter(d.Workspace, d.Phase); err != nil {
		e.warn("Engine.writeCleanStdout", d, CodeStdoutFilterFailed, fmt.Sprintf("stdout filter failed: %v (the raw log stays the forensic source)", err),
			map[string]string{"workspace": d.Workspace})
	}
}

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

func violationDiagnostics(vs []deliverable.Violation) []core.Diagnostic {
	var out []core.Diagnostic
	for _, v := range vs {
		out = append(out, core.Diagnostic{Severity: "error", Message: fmt.Sprintf("deliverable contract violation [%s]: %s", v.Code, v.Message)})
	}
	return out
}

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
