package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func timingOf(t *testing.T, h exhaustion, phase Phase) phaseTimingEntry {
	t.Helper()
	for _, e := range h.cr.phaseTimings {
		if e.Phase == string(phase) {
			return e
		}
	}
	t.Fatalf("no outcome of %s in %+v", phase, h.cr.phaseTimings)
	return phaseTimingEntry{}
}

func TestReviewWithCorrections_ABuildRejectedAfterItsCorrectionsRecordsACodedFail(t *testing.T) {
	t.Parallel()
	h := exhaustionHarness(t, PhaseBuild, codeReviewSpec(), "scout")

	_, _ = h.cr.reviewWithCorrections(PhaseBuild, h.dr)

	e := timingOf(t, h, PhaseBuild)
	codes := cyclestate.ErrorCodes(e.Diagnostics)
	if e.Phase != string(PhaseBuild) || e.Verdict != VerdictFAIL || len(codes) != 1 || codes[0] != cyclestate.DiagCodeBuildRejected {
		t.Fatalf("build entry = %+v; the rejection must be a coded FAIL, or the retro's PASS after it decides the class", e)
	}
	if e.Diagnostics[0].Message != e.AbortReason {
		t.Errorf("diagnostic %q, want the abort reason %q", e.Diagnostics[0].Message, e.AbortReason)
	}
}

func TestReviewWithCorrections_ARejectedPhaseOtherThanTheBuildKeepsItsUncodedOutcome(t *testing.T) {
	t.Parallel()
	spec := phasespec.PhaseSpec{Name: "gatekeeper", Role: string(phasespec.RoleEvaluate), After: "build"}
	h := exhaustionHarness(t, "gatekeeper", spec, "scout", "build")

	_, _ = h.cr.reviewWithCorrections("gatekeeper", h.dr)

	if e := timingOf(t, h, "gatekeeper"); e.Verdict != VerdictPASS || len(cyclestate.ErrorCodes(e.Diagnostics)) != 0 {
		t.Fatalf("gatekeeper entry = %+v; only the build rejection is charged to the item", e)
	}
}
