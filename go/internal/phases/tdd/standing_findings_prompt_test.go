package tdd

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Cycle 1679 (2026-09-15): audit round 4 passed WARN with three MEDIUM
// defects and the cycle went to ship; the ship hit GIT_FLEET_REBASE_NEEDED and
// recovered via a rebuild whose brief carried NO audit section (the repair
// brief seeds only after a rejection), so round 5 found the same defects
// "standing" and WARNed again. A recovery rebuild is re-audited by the same
// rubric: the last audit's actionable findings ride the brief as data.
func TestComposePrompt_StandingAuditFindingsReachTheAgent(t *testing.T) {
	const finding = "M1 MEDIUM claim-discrepancy: README.md:336 says five cycle predicates while the tree carries eight"
	req := core.PhaseRequest{Context: map[string]string{core.CtxKeyStandingAuditFindings: finding}}

	out := hooks{}.ComposePrompt("BODY", req)

	if !strings.Contains(out, finding) || !strings.Contains(out, "Standing Audit Findings") {
		t.Errorf("the last audit's standing findings never reached the prompt; the recovery rebuild would rebuild blind:\n%s", out)
	}
	if strings.Contains(out, "Audit Repair") {
		t.Errorf("standing findings are not a rejection — the audit PASSED with WARN; no Audit Repair section:\n%s", out)
	}
	if !strings.Contains(out, "```") {
		t.Error("standing findings must be fenced as DATA, not injected as prose the agent may follow")
	}
}
