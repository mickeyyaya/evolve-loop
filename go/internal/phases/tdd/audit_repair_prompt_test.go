package tdd

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const auditRepairFixtureReason = "EGPS: red_count=1 [record_absent_from_inbox_root_exactly_once] (cycle ships only when red_count==0)"

func TestComposePrompt_AuditRepairFindingsReachTheAgent(t *testing.T) {
	req := core.PhaseRequest{Context: map[string]string{
		core.CtxKeyAuditRepairFindings: auditRepairFixtureReason,
	}}

	out := hooks{}.ComposePrompt("BODY", req)

	if !strings.Contains(out, auditRepairFixtureReason) {
		t.Errorf("the audit's own reason never reached the tdd prompt; the repair would rebuild blind:\n%s", out)
	}
	if !strings.Contains(out, "```") {
		t.Error("audit-repair findings must be fenced as DATA, not injected as prose the agent may follow")
	}
}

func TestComposePrompt_NoAuditRepairFindings_LeavesPromptUnchanged(t *testing.T) {
	base := hooks{}.ComposePrompt("BODY", core.PhaseRequest{Context: map[string]string{}})

	if strings.Contains(base, "Audit Repair") {
		t.Errorf("an ordinary tdd prompt must carry no audit-repair section:\n%s", base)
	}
}
