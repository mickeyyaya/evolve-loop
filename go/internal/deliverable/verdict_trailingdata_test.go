package deliverable

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// Gate-level pin for the cycle-1478 halt shape: at contract-gate ENFORCE the
// prose fallback is off (ADR-0050 §3.10 Slice 5), so the sentinel parse is the
// only road to a verdict — a sentinel with a stray trailing brace inside the
// comment must therefore satisfy verdictPresent, or a one-byte slip becomes
// CodeBadVerdict -> three blocks (two correction re-dispatches, the second a
// salvage retry) -> circuit-open -> ADR-0072 halt (batch-20260815c,
// cycle-1478).
func TestVerdictPresent_Enforce_ToleratesTrailingBraceInSentinel(t *testing.T) {
	verdicts := []string{"PASS", "FAIL", "WARN", "SKIPPED"}
	content := "# Audit Report\n\n## Verdict\n\n**WARN**\n\n" +
		"<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"WARN\",\"schema_version\":2,\"failure\":{\"class\":\"code-audit-fail\",\"defects\":[\"M1: x\"]}}} -->\n"
	if !verdictPresent(content, verdicts, config.StageEnforce) {
		t.Fatal("stray trailing brace inside the sentinel comment must not defeat the verdict at enforce")
	}
}
