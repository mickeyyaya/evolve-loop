package audit

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// auditClassification owns the mutable state for one audit classification.
// Keeping the state concrete makes the ordered audit lifecycle visible without
// passing a growing list of verdict, diagnostic, and predicate values between
// otherwise independent stages.
type auditClassification struct {
	hooks    hooks
	artifact string
	req      core.PhaseRequest

	verdict      string
	narrative    string
	verdictFound bool
	diagnostics  []core.Diagnostic
	overrodeBy   []string

	redCount      int
	acsErr        error
	predicateErr  error
	sealPredicate func() error
}

func newAuditClassification(h hooks, artifact string, req core.PhaseRequest) *auditClassification {
	verdict, verdictFound := extractAuditVerdict(artifact, h.phaseIO)
	classification := &auditClassification{
		hooks:        h,
		artifact:     artifact,
		req:          req,
		verdict:      verdict,
		narrative:    verdict,
		verdictFound: verdictFound,
	}
	if !verdictFound {
		classification.verdict = core.VerdictFAIL
	}
	if len(artifact) > auditReportMaxBytes {
		classification.warn(fmt.Sprintf(
			"audit-report.md size %d bytes exceeds the %d-byte budget — trim ## Issues to the top findings by severity and keep evidence in the evictable sections (advisory: the verdict and the artifact are unchanged)",
			len(artifact), auditReportMaxBytes,
		))
	}
	if err := validateExplanationReview(artifact, req); err != nil {
		classification.fail("explanation documentation qualitative review", err.Error())
	}
	return classification
}

func (a *auditClassification) fail(gate, message string) {
	a.diagnostics = append(a.diagnostics, core.Diagnostic{Severity: "error", Message: message})
	a.override(gate)
}

func (a *auditClassification) override(gate string) {
	a.overrodeBy = append(a.overrodeBy, gate)
	a.verdict = core.VerdictFAIL
}

func (a *auditClassification) warn(message string) {
	a.diagnostics = append(a.diagnostics, core.Diagnostic{Severity: "warning", Message: message})
}

// prepareEvidence invalidates agent-authored predicate evidence, regenerates
// it on the host, and applies the authoritative EGPS result. The returned seal
// stays pending until finalize so it covers the final defect-ledger state.
func (a *auditClassification) prepareEvidence() {
	verdictPath := filepath.Join(a.req.Workspace, "acs-verdict.json")
	if err := quarantineProbesForRequest(a.req); err != nil {
		a.warn(fmt.Sprintf("probe quarantine: %s", err.Error()))
	}

	if a.hooks.predicateEvidence != nil {
		a.predicateErr = acssuite.InvalidateEvidence(filepath.Join(a.req.Workspace, "audit-report.md"))
	}
	if a.hooks.genVerdict != nil || a.hooks.predicateEvidence != nil {
		if _, err := os.Lstat(verdictPath); err == nil {
			if retireErr := preservePredicateCandidate(verdictPath); a.predicateErr == nil {
				a.predicateErr = retireErr
			}
		} else if !os.IsNotExist(err) && a.predicateErr == nil {
			a.predicateErr = err
		}
	}
	if a.hooks.predicateEvidence != nil && a.predicateErr == nil {
		if a.hooks.genVerdict == nil {
			a.predicateErr = fmt.Errorf("host predicate generator is not configured")
		} else {
			a.sealPredicate, a.predicateErr = a.hooks.predicateEvidence(a.req)
		}
	}
	if a.hooks.genVerdict != nil && a.predicateErr == nil {
		a.predicateErr = a.hooks.genVerdict(a.req)
	}
	if a.predicateErr != nil {
		a.fail("host predicate execution", "host predicate execution: "+a.predicateErr.Error())
	}
	if a.hooks.explanationCheck != nil {
		if err := a.hooks.explanationCheck(a.req); err != nil {
			a.fail("explanation documentation gate unavailable", fmt.Sprintf("explanation documentation gate: %s", err.Error()))
		}
	}

	redCount, redIDs, phantomBindings, shipEligible, acsErr := readACSVerdict(verdictPath)
	a.redCount = redCount
	a.acsErr = acsErr

	var blocked bool
	var reason, label string
	switch {
	case acsErr != nil:
		blocked = true
		reason = fmt.Sprintf("acs-verdict.json: %s", acsErr.Error())
		label = "EGPS acs-verdict.json unreadable"
	case redCount > 0:
		blocked = true
		reason = egpsRedMessage(redCount, redIDs, phantomBindings)
		label = "EGPS red_count>0"
	case shipEligible != nil && !*shipEligible:
		blocked = true
		reason = "EGPS: acs-verdict.json ship_eligible=false — the authoritative acssuite SSOT rejects the ship even though red_count==0; a narrative PASS cannot override it"
		label = "EGPS ship_eligible=false"
	}
	if blocked {
		a.fail(label, reason)
	}
}
