package tdd

// audit_repair_prompt_test.go — WIRING PROOF for the audit-repair loop
// (operating-policy §3.3). The repair branch is worthless if the audit's
// reasoning never reaches the agent asked to act on it: the cycle would rebuild
// blind and re-earn the same rejection, twice, at full cost.
//
// This is the check PR #503 shipped without, and the reason its hardening could
// have gone permanently dark unnoticed. It asserts the verbatim reason text
// survives prompt composition, and the paired negative pins that an absent key
// leaves the legacy prompt byte-identical.
//
// Measured motivation: cycle-1577 resumed cycle-1574's failed task and its
// scout, fault-localization and bug-reproduction prompts contained ZERO
// references to the failure; only build mentioned it, once.

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// The exact EGPS reason that failed cycle-1574 — a real string from a real
// rejection, so the proof is not tautological over a synthetic token.
const auditRepairFixtureReason = "EGPS: red_count=1 [record_absent_from_inbox_root_exactly_once] (cycle ships only when red_count==0)"

func TestComposePrompt_AuditRepairFindingsReachTheAgent(t *testing.T) {
	req := core.PhaseRequest{Context: map[string]string{
		core.CtxKeyAuditRepairFindings: auditRepairFixtureReason,
	}}

	out := hooks{}.ComposePrompt("BODY", req)

	if !strings.Contains(out, auditRepairFixtureReason) {
		t.Errorf("the audit's own reason never reached the tdd prompt; the repair would rebuild blind:\n%s", out)
	}
	// Fenced as DATA, mirroring the continuation-findings precedent: the text
	// quotes agent-authored lines and must never read as instructions.
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
