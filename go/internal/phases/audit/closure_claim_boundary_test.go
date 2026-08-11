package audit

// closure_claim_boundary_test.go — RED contract for the cycle-1431 verify-wave
// halt (auto-filed P0; prior firings 1339/1371/1428): closureClaimOffenders
// used an unbounded strings.Contains, so the substring "closed" inside
// "disclosed" — on a line that literally ended "still open" — tripped the
// gate, force-FAILed a narrative-PASS audit, and halted the batch as
// infra-systemic. Two false-positive classes close here: (1) substring
// matches ("disclosed", "foreclosed"); (2) negated/openness-asserting lines
// ("is NOT closed", "still open") — a report SAYING a defect remains open is
// the opposite of a closure claim.

import (
	"strings"
	"testing"
)

func TestClosureClaimOffenders_SubstringAndNegationFalsePositives(t *testing.T) {
	t.Parallel()
	benign := []string{
		// The live cycle-1431 shape: "closed" only inside "disclosed", line asserts openness.
		"The minted-path fix (cycle-1424) is disclosed in the footer; the underlying defect is still open.",
		"foreclosed options for cycle-1339 are listed below",
		"the cycle-1428 defect is NOT closed — evidence pending",
		"cycle-1371: this is not closed yet; do not retire it",
		"the handle is closed in the deferred cleanup", // no cycle ref — pre-existing carve-out
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
		// Compound line (diff-review HIGH): a real STRONG claim with an
		// appended openness clause must still offend — the guard may never
		// become a one-token bypass of the citation demand.
		"the cycle-1424 defect is verified closed; the unrelated item is still open",
	}
	for _, line := range offending {
		if got := closureClaimOffenders(line + "\n"); len(got) != 1 {
			t.Errorf("real uncited closure claim missed: %q -> %v", line, got)
		}
	}
	// A cited claim stays legal.
	cited := "the cycle-1424 defect is verified closed — see defect-dispositions.json"
	if got := closureClaimOffenders(cited + "\n"); len(got) != 0 {
		t.Errorf("cited claim wrongly flagged: %v", got)
	}
}

// The live halt's exact evidence must not reproduce: a full report whose only
// "closed" token is inside "disclosed" yields ZERO diagnostics.
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
