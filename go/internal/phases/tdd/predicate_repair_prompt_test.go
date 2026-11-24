package tdd

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestComposePrompt_ThePredicateRepairBriefReachesTheAgentAsData(t *testing.T) {
	failure := "./acs/cycle1761 (-tags acs): unit tests FAIL\n--- FAIL: TestC1761_009_OnlyTargetPackagesTouched"
	out := hooks{}.ComposePrompt("BODY", core.PhaseRequest{Context: map[string]string{core.CtxKeyPredicateRepair: failure}})
	plain := hooks{}.ComposePrompt("BODY", core.PhaseRequest{Context: map[string]string{}})

	if !strings.Contains(out, "## Predicate Repair") || !strings.Contains(out, "````text\n"+failure+"\n````") {
		t.Errorf("the repeated predicate failure must reach the tdd prompt fenced as data:\n%s", out)
	}
	if strings.Contains(plain, "Predicate Repair") {
		t.Error("a dispatch with no brief got the predicate repair section")
	}
}

func TestComposePrompt_ThePredicateRepairNamesTheTestAuditCause(t *testing.T) {
	failure := "test-audit ratchet: testaudit: acs/cycle10 TestX error-text-match has no baseline row"
	out := hooks{}.ComposePrompt("BODY", core.PhaseRequest{Context: map[string]string{core.CtxKeyPredicateRepair: failure}})

	if !strings.Contains(strings.Replace(out, failure, "", 1), "test-audit finding") || !strings.Contains(out, "````text\n"+failure+"\n````") {
		t.Errorf("the repair section must name the test-audit cause and keep the data fence:\n%s", out)
	}
}
