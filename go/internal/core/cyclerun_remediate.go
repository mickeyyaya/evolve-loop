package core

// cyclerun_remediate.go — graduated remediation (operator directive
// 2026-07-21; inbox graduated-remediation-fix-forward, ADR pending): when a
// configured DETERMINISTIC gate phase returns a FAIL verdict, dispatch the
// builder ONCE per round with the gate's report as a correction directive,
// then re-run the SAME gate and adopt its fresh verdict. The economics: a
// mechanical, gate-prescribed defect (missing tests, format, naming) costs a
// bounded in-phase fix (~1-2M tokens) instead of discarding a sound cycle
// (~12.5M) — the 983/992/1007/1019/1020 waste class, capped by cycle-1019
// where the audit-PASSed ADR-0072 S5 implementation was thrown away over
// three missing test files the gate itself had prescribed, and cycle-1020
// then re-implemented it from scratch and failed the same gate the same way.
//
// Integrity floors:
//   - the SAME gate must pass — remediation never overrides a verdict, it
//     re-earns one; every downstream phase (audit, EGPS, ship gates) runs
//     unchanged after it;
//   - the round cap is hard (config workflow.remediation_rounds, default 1 at
//     the composition root; ZERO in core's zero-value config so untouched
//     tests and legacy paths are byte-identical);
//   - only phases listed in workflow.remediable_phases participate —
//     deterministic gates only by contract; judgment phases must never be
//     listed;
//   - provenance is loud: CycleResult.Remediations records every round and
//     outcome, so a remediated cycle is never a silent PASS.

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards/treediff"
	"github.com/mickeyyaya/evolve-loop/go/internal/mintregistry"
)

// remediationDenied are the judgment/floor phases that must NEVER be
// remediable regardless of configuration: remediation is a re-earned verdict
// from a DETERMINISTIC gate, and granting the builder a re-roll against an
// LLM-judgment verdict (audit, adversarial-review, premise-challenge,
// plan-review) or a control phase would be a gamed verdict, not a fix.
var remediationDenied = map[Phase]bool{
	PhaseAudit:                  true,
	PhaseRetro:                  true,
	PhaseDebugger:               true,
	Phase("adversarial-review"): true,
	Phase("premise-challenge"):  true,
	Phase("plan-review"):        true,
}

// maybeRemediate runs between reviewAndGuard and recordAndBranch: on a FAIL
// verdict from a remediable gate with rounds remaining, it re-dispatches the
// builder with the gate report as directive, re-runs the gate, and replaces
// dr.resp with the fresh verdict. Always returns loopNext — a failed
// remediation leaves the original FAIL path untouched.
func (cr *cycleRun) maybeRemediate(next Phase, dr *dispatchResult) (loopAction, error) {
	if dr.resp.Verdict != VerdictFAIL {
		return loopNext, nil
	}
	wf := cr.o.workflowConfig
	if wf.RemediationRounds <= 0 || !remediableListed(wf.RemediablePhases, next) || next == PhaseBuild {
		return loopNext, nil
	}
	if remediationDenied[next] {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN remediation: phase %q is configured remediable but is a judgment/control phase — refused (deterministic gates only)\n", next)
		return loopNext, nil
	}
	builder, ok := cr.o.runners[PhaseBuild]
	if !ok {
		return loopNext, nil
	}
	if cr.remediationRounds == nil {
		cr.remediationRounds = map[Phase]int{}
	}
	if cr.remediationRounds[next] >= wf.RemediationRounds {
		return loopNext, nil
	}
	cr.remediationRounds[next]++
	round := cr.remediationRounds[next]

	report := filepath.Join(cr.cs.WorkspacePath, string(next)+"-report.md")
	fmt.Fprintf(os.Stderr, "[orchestrator] remediation: gate %s FAILed — dispatching builder fix round %d/%d (report: %s)\n",
		next, round, wf.RemediationRounds, report)

	// Record the ORIGINAL failing gate attempt through the ADR-0044 C1
	// chokepoint before anything else — the re-run gets its own window below,
	// so the record honestly shows gate-FAIL, fix, gate-rerun.
	cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, "", cr.cs.PhaseStartedAt))

	// Tree-diff guard parity with a normal dispatch (the one class this
	// codebase keeps hardening against): snapshot before the fix dispatch,
	// recover + check after; a main-tree leak voids the round.
	var fixGuard *treediff.Guard
	var fixBefore []string
	fixSnapOK := false
	if cr.o.gitDirtyPaths != nil {
		fixGuard = treediff.New(cr.o.gitDirtyPaths)
		if snap, serr := fixGuard.Snapshot(cr.ctx, cr.req.ProjectRoot); serr == nil {
			fixBefore, fixSnapOK = snap, true
		}
	}

	fixStarted := cr.o.now().UTC().Format("2006-01-02T15:04:05Z07:00")
	breq := dr.phaseReq
	// The gate's request is the template (same workspace/worktree/roots), but
	// the whole-cycle model-routing clamp is PER-PHASE — the builder must run
	// under its own profile, not the gate's (latent under routing=auto).
	// Known limitation: breq.BuildPlan stays empty (it is only populated for a
	// first-class build dispatch) — the remediation directive carries the gate
	// report instead, which is the context that matters for a scoped fix.
	breq.ModelRoutingCLI = ""
	breq.ModelRoutingTier = ""
	breq.CorrectionDirective = fmt.Sprintf(
		"REMEDIATION round %d: the deterministic gate %q FAILED after your build. "+
			"Read its report at %s and fix EXACTLY the enumerated defects in the worktree — "+
			"typically missing tests for new code. Do not widen scope; do not touch unrelated "+
			"files. The same gate re-runs immediately after you finish.", round, next, report)
	obsCancel := cr.o.observer.Start(cr.ctx, string(PhaseBuild), breq)
	bresp, berr := builder.Run(cr.ctx, breq)
	if obsCancel != nil {
		obsCancel()
	}
	// ADR-0044 C1: the fix dispatch burned tokens — record it whatever happens,
	// under its own label so it never clobbers the build phase's own records.
	fixAbort := ""
	if berr != nil {
		fixAbort = fmt.Sprintf("remediation fix dispatch: %v", berr)
	}
	cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(Phase("build-remediation"), bresp, 1, fixAbort, fixStarted))
	if berr != nil || bresp.Verdict == VerdictFAIL {
		note := fmt.Sprintf("%s: round %d builder-failed", next, round)
		cr.result.Remediations = append(cr.result.Remediations, note)
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN remediation: builder fix failed (%v, verdict=%s) — gate FAIL stands\n", berr, bresp.Verdict)
		return loopNext, nil
	}
	if fixSnapOK {
		// Same recovery-then-check the normal build path gets; a surviving
		// main-tree leak voids the round (original FAIL stands, loudly).
		recoverBuildLeak(cr.ctx, cr.req.ProjectRoot, cr.cs.ActiveWorktree, cr.mainDirtyBaseline, true)
		if res := fixGuard.Check(cr.ctx, cr.req.ProjectRoot, fixBefore); !res.SnapshotMissed && !res.OK() {
			// Same classification vocabulary as the main guard, including the
			// verified-mint exemption — in fleet mode a concurrent lane's mint
			// landing during the fix window must not spuriously void the round.
			mints, _ := mintregistry.ActiveNames(mintregistry.Path(cr.req.ProjectRoot), cr.o.now())
			mints = verifiedActiveMints(cr.req.ProjectRoot, mints)
			var real []string
			for _, lp := range res.Leaked {
				if isLegitimateMainTreePath(lp) || isActiveMintPhasePath(mints, lp) {
					continue
				}
				real = append(real, lp)
			}
			if len(real) > 0 {
				note := fmt.Sprintf("%s: round %d voided (fix leaked to main tree: %v)", next, round, real)
				cr.result.Remediations = append(cr.result.Remediations, note)
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN remediation: %s — gate FAIL stands\n", note)
				return loopNext, nil
			}
		}
	}

	// Fresh timing window for the re-run: recordAndBranch later records the
	// FINAL verdict against cs.PhaseStartedAt — without this reset the window
	// would silently span the fix work recorded separately above.
	cr.cs.PhaseStartedAt = cr.o.now().UTC().Format("2006-01-02T15:04:05Z07:00")
	greq := dr.phaseReq
	greq.CorrectionDirective = fmt.Sprintf("RE-VERIFY after remediation round %d: run your full check fresh against the current worktree.", round)
	obsCancel = cr.o.observer.Start(cr.ctx, string(next), greq)
	resp2, rerr := dr.runner.Run(cr.ctx, greq)
	if obsCancel != nil {
		obsCancel()
	}
	if rerr != nil {
		note := fmt.Sprintf("%s: round %d re-run dispatch-failed", next, round)
		cr.result.Remediations = append(cr.result.Remediations, note)
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN remediation: gate re-run dispatch failed (%v) — original FAIL stands\n", rerr)
		return loopNext, nil
	}
	dr.resp = resp2
	cr.result.Remediations = append(cr.result.Remediations,
		fmt.Sprintf("%s: round %d -> %s", next, round, resp2.Verdict))
	fmt.Fprintf(os.Stderr, "[orchestrator] remediation: gate %s re-ran -> %s (round %d/%d)\n",
		next, resp2.Verdict, round, wf.RemediationRounds)
	return loopNext, nil
}

// remediableListed reports whether phase is in the configured remediable set.
func remediableListed(list []string, p Phase) bool {
	for _, s := range list {
		if s == string(p) {
			return true
		}
	}
	return false
}
