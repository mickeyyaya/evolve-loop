package audit

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// reconcileContinuation verifies inherited defect dispositions and checks that
// prose closure claims cite the records that make those claims auditable.
func (a *auditClassification) reconcileContinuation() {
	ledgerDiagnostics, ledgerBlocked, lineageCycles := reconcileContinuationDefects(a.req)
	a.diagnostics = append(a.diagnostics, ledgerDiagnostics...)
	if ledgerBlocked {
		a.override("continuation defect-ledger")
	}

	closureDiagnostics := closureClaimDiagnostics(a.artifact)
	if len(closureDiagnostics) == 0 {
		return
	}
	vouched := map[int]bool{a.req.Cycle: true}
	for _, cycle := range lineageCycles {
		vouched[cycle] = true
	}
	offenders := closureClaimOffenders(a.artifact)
	forced := false
	for i := range closureDiagnostics {
		inLineage := false
		if len(lineageCycles) > 0 && i < len(offenders) {
			refs := closureLineCycleRefs(offenders[i])
			inLineage = len(refs) > 0
			for _, ref := range refs {
				if !vouched[ref] {
					inLineage = false
					break
				}
			}
		}
		if inLineage {
			closureDiagnostics[i].Severity = "warning"
			closureDiagnostics[i].Message += " [demoted to advisory: this cycle's defect-ledger reconcile verified every inherited defect of the referenced lineage against its per-id disposition record — cite on the claim line anyway next time]"
		} else {
			forced = true
		}
	}
	a.diagnostics = append(a.diagnostics, closureDiagnostics...)
	if forced {
		a.override("closure-claim citation")
	}
}

// finalize applies policy and bookkeeping after every evidence gate has run.
// Predicate sealing deliberately remains after ledger emission; existing audit
// artifacts and chain-shadow records depend on that ordering.
func (a *auditClassification) finalize() {
	if a.verdictFound && core.IsVerdict(a.narrative) && a.narrative != core.VerdictFAIL && len(a.overrodeBy) > 0 {
		a.diagnostics = append(a.diagnostics, core.Diagnostic{
			Severity: "error",
			Message:  verdictConflictMessage(a.narrative, a.overrodeBy),
		})
	}

	if a.verdict == core.VerdictWARN && policy.StrictAuditFor(a.req.ProjectRoot) {
		a.verdict = core.VerdictFAIL
		a.diagnostics = append(a.diagnostics, core.Diagnostic{
			Severity: "error",
			Message:  "policy.json workflow.strict_audit promoted WARN to FAIL",
		})
	}
	if !a.verdictFound && a.acsErr == nil && a.redCount == 0 && strings.TrimSpace(a.artifact) != "" {
		a.diagnostics = append(a.diagnostics, core.Diagnostic{
			Severity: "error",
			Message:  "audit-report.md is non-empty with red_count=0 but declares no parseable verdict — treating as FAIL. Declare it as '## Verdict' + a bold verdict on the next line, or inline as '**Verdict: PASS**'.",
		})
	}

	if a.verdict == core.VerdictFAIL || a.verdict == core.VerdictWARN {
		if err := emitDefectLedger(a.artifact, a.req); err != nil {
			a.warn(fmt.Sprintf("defect ledger: could not record this cycle's defects (%s) — a later continuation will have nothing to reconcile against", err.Error()))
		}
	}
	if a.sealPredicate != nil && a.predicateErr == nil {
		if err := a.sealPredicate(); err != nil {
			a.fail("host predicate evidence", "host predicate evidence: "+err.Error())
		}
	}
	recordChainShadow(a.artifact, a.req, a.narrative, a.verdict, a.overrodeBy)
}
