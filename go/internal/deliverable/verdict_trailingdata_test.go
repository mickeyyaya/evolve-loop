package deliverable

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func TestVerdictPresent_Enforce_ToleratesTrailingBraceInSentinel(t *testing.T) {
	verdicts := []string{"PASS", "FAIL", "WARN", "SKIPPED"}
	content := "# Audit Report\n\n## Verdict\n\n**WARN**\n\n" +
		"<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"WARN\",\"schema_version\":2,\"failure\":{\"class\":\"code-audit-fail\",\"defects\":[\"M1: x\"]}}} -->\n"
	if !verdictPresent(content, verdicts, config.StageEnforce) {
		t.Fatal("stray trailing brace inside the sentinel comment must not defeat the verdict at enforce")
	}
}
