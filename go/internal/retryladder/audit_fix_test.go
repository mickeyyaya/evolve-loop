package retryladder_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

func smallBounds() retryladder.AuditFixBounds {
	return retryladder.AuditFixBounds{MaxFiles: 3, MaxLines: 40, MaxCodeLines: 10, MaxTokens: 6000, NoNewProductionLogic: true}
}

func TestChooseAuditFix_PicksASmallFullyPrescribedFix(t *testing.T) {
	in := retryladder.AuditFixInput{
		Findings: 1, Prescriptions: 1, AllPrescribed: true,
		Estimate: retryladder.AuditFixEstimate{Files: 1, Lines: 20, CodeLines: 0, Tokens: 240},
		Bounds:   smallBounds(),
	}
	got := retryladder.ChooseAuditFix(in)
	if !got.Choose || got.Rule != retryladder.AuditFixRuleSmallPrescribed || got.Input != in {
		t.Fatalf("ChooseAuditFix = %+v, want audit_fix by the small-prescribed rule with its input", got)
	}
	if retryladder.RungAuditFix != "audit_fix" {
		t.Fatalf("rung name drifted: %q", retryladder.RungAuditFix)
	}
}

func TestChooseAuditFix_KeepsTheRepairRungOtherwise(t *testing.T) {
	ok := retryladder.AuditFixInput{
		Findings: 2, Prescriptions: 2, AllPrescribed: true,
		Estimate: retryladder.AuditFixEstimate{Files: 2, Lines: 30, CodeLines: 5, Tokens: 360}, Bounds: smallBounds(),
	}
	for _, tc := range []struct {
		name string
		edit func(*retryladder.AuditFixInput)
		rule string
	}{
		{"one audit_fix per cycle", func(in *retryladder.AuditFixInput) { in.Used = true }, retryladder.AuditFixRuleUsed},
		{"no finding", func(in *retryladder.AuditFixInput) { in.Findings, in.Prescriptions = 0, 0 }, retryladder.AuditFixRuleNoFindings},
		{"a finding without a prescription", func(in *retryladder.AuditFixInput) { in.AllPrescribed = false }, retryladder.AuditFixRuleUnprescribed},
		{"too many files", func(in *retryladder.AuditFixInput) { in.Estimate.Files = 4 }, retryladder.AuditFixRuleFiles},
		{"too many lines", func(in *retryladder.AuditFixInput) { in.Estimate.Lines = 41 }, retryladder.AuditFixRuleLines},
		{"new production logic", func(in *retryladder.AuditFixInput) { in.Estimate.CodeLines = 11 }, retryladder.AuditFixRuleCodeLines},
		{"over the token cap", func(in *retryladder.AuditFixInput) { in.Estimate.Tokens = 6001 }, retryladder.AuditFixRuleTokens},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := ok
			tc.edit(&in)
			if got := retryladder.ChooseAuditFix(in); got.Choose || got.Rule != tc.rule {
				t.Fatalf("ChooseAuditFix = %+v, want no audit_fix by rule %q", got, tc.rule)
			}
		})
	}
	codeAllowed := ok
	codeAllowed.Estimate.CodeLines, codeAllowed.Bounds.NoNewProductionLogic = 30, false
	if got := retryladder.ChooseAuditFix(codeAllowed); !got.Choose {
		t.Fatalf("without the production-logic limit, code lines do not block: %+v", got)
	}
}

func TestEstimateAuditFix_ReadsThePathsAndSpansOfThePrescription(t *testing.T) {
	defects := []string{"H1: go/internal/core/retro_remediation_filter_test.go:29 counts every entry"}
	prescriptions := []string{
		"Make inboxFiles in go/internal/core/retro_remediation_filter_test.go count only *.json items",
		"Update go/internal/core/testdata/floor.golden and go/internal/core/floor.go:40-44",
	}
	got := retryladder.EstimateAuditFix(defects, prescriptions)
	want := retryladder.AuditFixEstimate{
		Files:     3,
		Lines:     1 + retryladder.AuditFixLinesPerUnspannedFile + 5,
		CodeLines: 5,
	}
	want.Tokens = want.Lines * retryladder.AuditFixTokensPerLine
	if got != want {
		t.Fatalf("EstimateAuditFix = %+v, want %+v", got, want)
	}
	if empty := retryladder.EstimateAuditFix(nil, []string{"rerun the suite"}); empty != (retryladder.AuditFixEstimate{}) {
		t.Fatalf("a prescription without a path estimated %+v", empty)
	}
}

func TestAllPrescribed_NeedsAConcretePrescriptionForEachFinding(t *testing.T) {
	if !retryladder.AllPrescribed([]string{"H1"}, []string{"fix x"}) {
		t.Fatalf("one finding with one prescription is prescribed")
	}
	for name, p := range map[string][2][]string{
		"no finding":           {nil, {"fix"}},
		"fewer prescriptions":  {{"H1", "M1"}, {"fix"}},
		"a blank prescription": {{"H1"}, {"  "}},
	} {
		if retryladder.AllPrescribed(p[0], p[1]) {
			t.Fatalf("%s counted as prescribed", name)
		}
	}
}
