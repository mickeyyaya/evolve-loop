package audit

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	warnShipsRule = "WARN ships under the fluent default; only `workflow.strict_audit` in `.evolve/policy.json` makes ship refuse it."

	warnFailsRule = "this project sets `workflow.strict_audit` in `.evolve/policy.json`, so the host turns your WARN into FAIL and ship refuses it."

	addedCommentRule = "New and changed code carries no comments ([docs/conventions/code-comments.md](docs/conventions/code-comments.md); its §What a comment may say lists the only exceptions): each added comment is a LOW finding, advisory and never a WARN or FAIL on its own, and never ask for a doc comment or a comment that explains code."

	explanationCorrectionRule = "Use `NEEDS_CORRECTION` whenever the prose of the cycle document is inaccurate or incomplete. The gate records `NEEDS_CORRECTION` as an advisory (ADR-0102), and the verdict stays a judgment of the change: when the code and tests are verified and only the document is wrong, keep the verdict PASS (or WARN) with `NEEDS_CORRECTION`. Before ship the host runs one document-only correction (X2): Build corrects only the document lines your Evidence names, and the audit re-reviews the document, with no TDD or code build. So begin each document defect in Evidence with the document's path and line (`docs/explain/builds/cycle-42-run-42.md:12 claims …`). FAIL only for a defect in the code or tests, listed as its own entry beginning with its own path; an inaccurate document alone is never the audit's FAIL."
)

func warnRule(projectRoot string) string {
	if policy.StrictAuditFor(projectRoot) {
		return warnFailsRule
	}
	return warnShipsRule
}

func auditRulesPromptBlock(req core.PhaseRequest) string {
	return "\n## Audit Rules\n\nThe host renders these rules from one source, the audit phase code. They override any other statement of them.\n\n" +
		"- **WARN:** " + warnRule(req.ProjectRoot) + "\n" +
		"- **Comments:** " + addedCommentRule + "\n" +
		"- **Explanation review:** " + explanationCorrectionRule + "\n"
}
