package retryladder_test

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

func TestChooseAuditFix_DeclinesAFindingThatCitesNoPath(t *testing.T) {
	defects := []string{
		"H1: go/internal/core/retro_remediation_filter_test.go:29 counts every entry",
		"M1: the floor tests of failurelearning fail with this diff",
	}
	prescriptions := []string{"fix the helper", "land the console item"}
	est := retryladder.EstimateAuditFix(defects, prescriptions)
	if est.Uncited != 1 {
		t.Fatalf("Uncited = %d, want 1 for the finding without a path", est.Uncited)
	}
	in := retryladder.AuditFixInput{Findings: 2, Prescriptions: 2, AllPrescribed: true, Estimate: est, Bounds: smallBounds()}
	if got := retryladder.ChooseAuditFix(in); got.Choose || got.Rule != retryladder.AuditFixRuleUncited {
		t.Fatalf("ChooseAuditFix = %+v, want a decline by rule %q", got, retryladder.AuditFixRuleUncited)
	}
	zero := retryladder.EstimateAuditFix([]string{"M1: rerun the suite"}, []string{"rerun it"})
	if zero.Lines != 0 || zero.Uncited != 1 {
		t.Fatalf("a finding without a path estimated %+v, want 0 lines and 1 uncited", zero)
	}
	in.Estimate = zero
	in.Findings, in.Prescriptions = 1, 1
	if got := retryladder.ChooseAuditFix(in); got.Choose {
		t.Fatalf("an estimate of 0 lines passed the caps: %+v", got)
	}
}

func TestCitedPaths_ListsEachCitedFileOnce(t *testing.T) {
	got := retryladder.CitedPaths(
		"H1: go/internal/a/a_test.go:29 and go/internal/a/a_test.go:40-44",
		"update go/internal/a/testdata/x.golden",
		"no path here",
	)
	want := []string{"go/internal/a/a_test.go", "go/internal/a/testdata/x.golden"}
	if !slices.Equal(got, want) {
		t.Fatalf("CitedPaths = %v, want %v", got, want)
	}
	if none := retryladder.CitedPaths("rerun the suite"); len(none) != 0 {
		t.Fatalf("CitedPaths of a text without a path = %v", none)
	}
}

func TestEstimatePatch_CountsTheChangedLinesOfEachFile(t *testing.T) {
	got := retryladder.EstimatePatch(map[string]int{"go/internal/a/a_test.go": 12, "go/internal/a/a.go": 3, "go/internal/a/x.golden": 2})
	want := retryladder.AuditFixEstimate{Files: 3, Lines: 17, CodeLines: 3, Tokens: 17 * retryladder.AuditFixTokensPerLine}
	if got != want {
		t.Fatalf("EstimatePatch = %+v, want %+v", got, want)
	}
}

func TestAuditFixOverBounds_NamesTheFirstLimitThatAPatchPasses(t *testing.T) {
	b := smallBounds()
	for _, tc := range []struct {
		est  retryladder.AuditFixEstimate
		rule string
	}{
		{retryladder.AuditFixEstimate{Files: 1, Lines: 20, Tokens: 240}, ""},
		{retryladder.AuditFixEstimate{Files: 4, Lines: 20}, retryladder.AuditFixRuleFiles},
		{retryladder.AuditFixEstimate{Files: 1, Lines: 41}, retryladder.AuditFixRuleLines},
		{retryladder.AuditFixEstimate{Files: 1, Lines: 20, CodeLines: 11}, retryladder.AuditFixRuleCodeLines},
		{retryladder.AuditFixEstimate{Files: 1, Lines: 20, Tokens: 6001}, retryladder.AuditFixRuleTokens},
	} {
		if got := retryladder.AuditFixOverBounds(tc.est, b); got != tc.rule {
			t.Errorf("AuditFixOverBounds(%+v) = %q, want %q", tc.est, got, tc.rule)
		}
	}
}
