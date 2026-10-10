package audit

import (
	"strings"
	"testing"
)

func TestClosureClaimOffenders_SubstringAndNegationFalsePositives(t *testing.T) {
	t.Parallel()
	benign := []string{
		"The minted-path fix (cycle-1424) is disclosed in the footer; the underlying defect is still open.",
		"foreclosed options for cycle-1339 are listed below",
		"the cycle-1428 defect is NOT closed — evidence pending",
		"cycle-1371: this is not closed yet; do not retire it",
		"the handle is closed in the deferred cleanup",
	}
	for _, line := range benign {
		if got := closureClaimOffenders(line + "\n"); len(got) != 0 {
			t.Errorf("false positive on benign line %q -> %v", line, got)
		}
	}
}

func TestClosureClaimOffenders_RealClaimsStillCaught(t *testing.T) {
	t.Parallel()
	offending := []string{
		"the cycle-1424 defect is verified closed",
		"closed the cycle-1405 finding during this lane's build",
		"Cycle 1255's CRITICAL is closed.",
		"the cycle-1424 defect is verified closed; the unrelated item is still open",
	}
	for _, line := range offending {
		if got := closureClaimOffenders(line + "\n"); len(got) != 1 {
			t.Errorf("real uncited closure claim missed: %q -> %v", line, got)
		}
	}
	cited := "the cycle-1424 defect is verified closed — see defect-dispositions.json"
	if got := closureClaimOffenders(cited + "\n"); len(got) != 0 {
		t.Errorf("cited claim wrongly flagged: %v", got)
	}
}

func TestClosureClaimDiagnostics_Cycle1431ShapeClean(t *testing.T) {
	t.Parallel()
	report := strings.Join([]string{
		"## Verdict", "**PASS**", "",
		"**Root cause:** the minted dispatch path (cycle-1424) is disclosed in the bridge footer;",
		"the seam is behaviour-neutral and the tracked defect is still open.",
	}, "\n")
	if diags := closureClaimDiagnostics(report); len(diags) != 0 {
		t.Fatalf("the cycle-1431 false-RED reproduced: %v", diags)
	}
}
