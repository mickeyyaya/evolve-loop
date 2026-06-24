package deliverable

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// ADR-0050 §3.10 Slice 5: verdictPresent's prose substring scan is the legacy
// fallback for older templates; at enforce the evolve-verdict sentinel is
// mandatory. Below enforce the prose scan stays active (byte-identical).
func TestVerdictPresent_EnforceRequiresSentinel(t *testing.T) {
	verdicts := []string{"PASS", "FAIL", "WARN", "SKIPPED"}
	prose := "# Report\n## Verdict\n**PASS**\n" // contains the "PASS" substring, no sentinel
	sentinel := "# Report\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\",\"schema_version\":1} -->\n"

	if !verdictPresent(prose, verdicts, config.StageAdvisory) {
		t.Error("advisory: prose verdict must be accepted (legacy fallback active)")
	}
	if verdictPresent(prose, verdicts, config.StageEnforce) {
		t.Error("enforce: prose-only report must be rejected — sentinel mandatory")
	}
	if !verdictPresent(sentinel, verdicts, config.StageEnforce) {
		t.Error("enforce: a valid sentinel must be accepted")
	}
	if !verdictPresent(sentinel, verdicts, config.StageOff) {
		t.Error("off: a valid sentinel must be accepted")
	}

	// An out-of-vocabulary sentinel at enforce is rejected: it is not an allowed
	// verdict, and with the prose rescue gated off it falls straight to false
	// (CodeBadVerdict) — no silent prose pass. Even though "PASS" appears in the
	// prose body, enforce must not rescue it.
	oov := "# Report\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"BOGUS\",\"schema_version\":1} -->\nPASS appears in prose\n"
	if verdictPresent(oov, verdicts, config.StageEnforce) {
		t.Error("enforce: out-of-vocab sentinel must be rejected (no prose rescue)")
	}
	// Below enforce the same out-of-vocab sentinel IS rescued by the prose scan
	// (the "PASS" substring) — the byte-identical legacy behavior.
	if !verdictPresent(oov, verdicts, config.StageAdvisory) {
		t.Error("advisory: out-of-vocab sentinel falls through to the prose rescue")
	}
}
