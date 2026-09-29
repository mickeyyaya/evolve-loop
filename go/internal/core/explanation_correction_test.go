package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const (
	correctionCycle    = 1745
	correctionRunID    = "01M3MAK46KK0HAVQVSBXMHAQ4Q"
	correctionDocument = "docs/explain/builds/cycle-1745-01m3mak46kk0havqvsbxmhaq4q.md"
)

var cycle1745Defects = []string{
	correctionDocument + ":8 states Report.Markdown shrank 60 -> 9 lines; the ratchet scanner (sizeratchet.go:82 formula) measures 8 for report.go:143-150 — correct the number",
	correctionDocument + ":35 claims the differential probe pins the equal-wall Packages order; the probe sorts packages by Pkg (go/acs/cycle1745/testdata/testlatency_probe.go.txt:70-72) so it neither pins nor depends on it — restate the reason as behavior-change out of scope for an extract-method lane",
}

func correctionState() CycleState {
	return CycleState{CycleID: correctionCycle, RunID: correctionRunID}
}

func TestExplanationCorrectionDocument_Cycle1745DefectsNameOnlyTheDocument(t *testing.T) {
	fb := &phasecontract.FailureBlock{Class: "code-audit-fail", Defects: cycle1745Defects}

	document, ok := explanationCorrectionDocument(correctionState(), fb)

	if !ok || document != correctionDocument {
		t.Fatalf("explanationCorrectionDocument = (%q, %v), want (%q, true): both 1745 defects locate in the cycle's own document", document, ok, correctionDocument)
	}
}

func TestExplanationCorrectionDocument_LabelledAndQuotedLocationsStillNameTheDocument(t *testing.T) {
	for _, defect := range []string{
		"H1: " + correctionDocument + ":8-10 and :18-21 say both sides are pinned",
		"H1 " + correctionDocument + ":19-24 says blocks were lifted",
		"H1 HIGH explanation-documentation NEEDS_CORRECTION: " + correctionDocument + ":12 overstates the scope",
		"H2: explanation document NEEDS_CORRECTION: Rationale (" + correctionDocument + ":30) cites a count the evidence lacks",
		"M1: `" + correctionDocument + "#L41` names the wrong caller",
		"/Users/op/runtime/.evolve/worktrees/cycle-1745/" + correctionDocument + ":8 is stale",
		"./" + correctionDocument + ":8 is stale",
	} {
		fb := &phasecontract.FailureBlock{Class: "code-audit-fail", Defects: []string{defect}}
		if _, ok := explanationCorrectionDocument(correctionState(), fb); !ok {
			t.Errorf("defect %q locates in the cycle's document, but was not classed explanation-only", defect)
		}
	}
}

func TestExplanationCorrectionDocument_ADefectOutsideTheDocumentKeepsTheRepairRound(t *testing.T) {
	cases := []struct {
		name    string
		cs      CycleState
		defects []string
	}{
		{"one code defect beside a document defect", correctionState(), []string{cycle1745Defects[0], "H1: go/internal/cyclesimulator/characterization_test.go:263-281 builds a raw git repo"}},
		{"a code location that cites the document afterwards", correctionState(), []string{"H1: go/internal/core/x.go:12 contradicts " + correctionDocument + ":3"}},
		{"a bare report file before the document", correctionState(), []string{"M1: build-report.md:227 and " + correctionDocument + ":55-56 claim a pure straight-cut move"}},
		{"a document defect with no path at all", correctionState(), []string{"M1: explanation doc :97 claims only date-carrying ids see different output"}},
		{"another cycle's document", correctionState(), []string{"docs/explain/builds/cycle-1744-01m3mak46kk0havqvsbxmhaq4q.md:8 is stale"}},
		{"another run's document for the same cycle", correctionState(), []string{"docs/explain/builds/cycle-1745-01m3zzzzzzzzzzzzzzzzzzzzzz.md:8 is stale"}},
		{"a runner gate diagnosed the FAIL too", CycleState{CycleID: correctionCycle, RunID: correctionRunID, AuditFailReasons: []string{"go vet: internal/x: unreachable code"}}, cycle1745Defects},
		{"no defects", correctionState(), nil},
		{"a cycle state that names no document", CycleState{CycleID: correctionCycle}, cycle1745Defects},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fb := &phasecontract.FailureBlock{Class: "code-audit-fail", Defects: tc.defects}
			if document, ok := explanationCorrectionDocument(tc.cs, fb); ok {
				t.Fatalf("classed explanation-only (document %q); a FAIL with any defect outside the document keeps today's routing", document)
			}
		})
	}
	if _, ok := explanationCorrectionDocument(correctionState(), nil); ok {
		t.Fatal("an audit with no failure block was classed explanation-only")
	}
}
