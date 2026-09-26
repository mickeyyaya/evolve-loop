package deliverable

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

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

	// At enforce an out-of-vocabulary sentinel gets no prose rescue, though "PASS" appears in the prose.
	oov := "# Report\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"BOGUS\",\"schema_version\":1} -->\nPASS appears in prose\n"
	if verdictPresent(oov, verdicts, config.StageEnforce) {
		t.Error("enforce: out-of-vocab sentinel must be rejected (no prose rescue)")
	}
	// Below enforce the prose scan rescues the same sentinel.
	if !verdictPresent(oov, verdicts, config.StageAdvisory) {
		t.Error("advisory: out-of-vocab sentinel falls through to the prose rescue")
	}
}
