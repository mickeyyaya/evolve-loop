package convergence_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func docText(t *testing.T, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestAmendmentOfADR0126Decision9IsRecorded(t *testing.T) {
	adr := docText(t, "docs/architecture/adr/0126-every-iterative-loop-converges-or-escalates.md")
	decision9 := adr[strings.Index(adr, "9. **The audit is untouched.**"):strings.Index(adr, "10. **The cycle pipeline converges too**")]
	for _, kept := range []string{"CRITICAL is never deferred", "no audit FAIL becomes a PASS", "with no deferrals"} {
		if !strings.Contains(decision9, kept) {
			t.Errorf("decision 9 lost %q", kept)
		}
	}
	for _, want := range []string{"The audit is untouched", "The audit keeps its full verdict rules", "ADR-0168", "0168-from-round-2-the-audit-re-reviews-the-repair-diff-and-a-late-finding-is-filed.md"} {
		if !strings.Contains(decision9, want) {
			t.Errorf("decision 9 lacks %q", want)
		}
	}
	amending := docText(t, "docs/architecture/adr/0168-from-round-2-the-audit-re-reviews-the-repair-diff-and-a-late-finding-is-filed.md")
	for _, sentence := range []string{"The audit is untouched", "The audit keeps its full verdict rules", "CRITICAL is never deferred", "no audit FAIL becomes a PASS", "no deferrals"} {
		if !strings.Contains(amending, sentence) {
			t.Errorf("ADR-0168 does not name %q", sentence)
		}
	}
}

func TestConvergencePolicyStatesTheAmendedAuditRule(t *testing.T) {
	policyDoc := docText(t, "docs/architecture/convergence-policy.md")
	section11 := policyDoc[strings.Index(policyDoc, "### 11. What this does not do"):]
	for _, want := range []string{"From round 2, the audit files a late MEDIUM or LOW (ADR-0168)", "A late CRITICAL or HIGH still blocks", "can only force FAIL"} {
		if !strings.Contains(section11, want) {
			t.Errorf("convergence-policy section 11 lacks %q", want)
		}
	}
	preamble := policyDoc[strings.Index(policyDoc, "### 4. Rules for judges"):strings.Index(policyDoc, "### 5. Output first")]
	if !strings.Contains(preamble, "ADR-0168") {
		t.Error("section 4 does not point to ADR-0168")
	}
}
