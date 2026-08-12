package core

// failure_digest_pathvariance_test.go — RED contract for cycle-1440 task
// `fingerprint-normalizer-path-variance`.
//
// Defect (PR #442 diff-review LOW): normalizeReasonForFingerprint strips exactly
// two identity-noise tokens — narrative=<verdict> and go-test durations. Every
// other per-cycle-varying token still splits ONE recurring defect into N
// fingerprints, so the identical-fingerprint breaker (ceiling 3, standing rule
// three_consecutive_fails_halt) never reaches its ceiling and the batch keeps
// burning cycles on the same defect. The two live shapes are:
//
//	1. cycle-numbered PATHS — ".evolve/runs/cycle-1365/audit-report.md" vs the
//	   same artifact one cycle later. Same defect, two fingerprints.
//	2. ATTEMPT DENOMINATORS — "attempt 1/3" vs "attempt 2/3" in a retry-loop
//	   abort reason. Same defect, three fingerprints — exactly the count the
//	   breaker needs to see as ONE.
//
// Contract: both fold to a stable token; the DEFECT-identifying content (which
// gate, which predicate, which artifact FILE) stays untouched, so two different
// defects can never collapse into one fingerprint (the over-normalization
// hazard the current doc comment calls out as deliberately avoided).

import "testing"

// TestNormalizeReasonForFingerprint_CycleNumberedPathsFold is the primary case:
// the SAME abort shape recorded on two cycles must fingerprint identically.
func TestNormalizeReasonForFingerprint_CycleNumberedPathsFold(t *testing.T) {
	cases := []struct {
		name string
		a, b string
	}{
		{
			name: "runs artifact path",
			a:    "ship: audit binding missing .evolve/runs/cycle-1365/audit-report.md",
			b:    "ship: audit binding missing .evolve/runs/cycle-1372/audit-report.md",
		},
		{
			name: "worktree path",
			a:    "build: phase wrote outside .evolve/worktrees/cycle-42824668-1440/go",
			b:    "build: phase wrote outside .evolve/worktrees/cycle-11111111-1439/go",
		},
		{
			name: "bare cycle token in prose",
			a:    "all families exhausted for cycle 1365 (launch refused)",
			b:    "all families exhausted for cycle 1372 (launch refused)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, want := normalizeReasonForFingerprint(tc.a), normalizeReasonForFingerprint(tc.b); got != want {
				t.Errorf("cycle-numbered path variance still splits the fingerprint:\n a -> %q\n b -> %q", got, want)
			}
		})
	}
}

// TestNormalizeReasonForFingerprint_AttemptDenominatorFolds pins the second
// rule: a retry-loop abort reason naming its attempt index is ONE defect.
func TestNormalizeReasonForFingerprint_AttemptDenominatorFolds(t *testing.T) {
	cases := []struct{ a, b string }{
		{"artifact timeout waiting for build-report.md (attempt 1/3)", "artifact timeout waiting for build-report.md (attempt 2/3)"},
		{"launch refused: no CLI family available, retry 1 of 4", "launch refused: no CLI family available, retry 3 of 4"},
	}
	for _, tc := range cases {
		if got, want := normalizeReasonForFingerprint(tc.a), normalizeReasonForFingerprint(tc.b); got != want {
			t.Errorf("attempt-denominator variance still splits the fingerprint:\n a -> %q\n b -> %q", got, want)
		}
	}
}

// TestNormalizeReasonForFingerprint_DistinctDefectsStayDistinct is the
// load-bearing NEGATIVE test: over-normalization that collapsed two DIFFERENT
// defects would blind the breaker far worse than the variance it fixes. A
// no-op normalizer passes the two tests above only by collapsing everything —
// this one is what refutes that.
func TestNormalizeReasonForFingerprint_DistinctDefectsStayDistinct(t *testing.T) {
	cases := []struct {
		name string
		a, b string
	}{
		{
			name: "different artifact in the same cycle dir",
			a:    "ship: audit binding missing .evolve/runs/cycle-1365/audit-report.md",
			b:    "ship: audit binding missing .evolve/runs/cycle-1365/build-report.md",
		},
		{
			name: "different gate",
			a:    "ship blocked: repo_contract_gate RED",
			b:    "ship blocked: contract_gate RED",
		},
		{
			name: "different predicate id",
			a:    "acs predicate TestC1440_001_CarryoverRetires failed",
			b:    "acs predicate TestC1440_002_StageRefusalDeterministic failed",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if normalizeReasonForFingerprint(tc.a) == normalizeReasonForFingerprint(tc.b) {
				t.Errorf("two DIFFERENT defects collapsed into one fingerprint %q — over-normalization blinds the breaker",
					normalizeReasonForFingerprint(tc.a))
			}
		})
	}
}

// TestNormalizeReasonForFingerprint_ExistingPinsStayGreen is the regression
// guard: the two normalizations that already exist must survive the extension.
func TestNormalizeReasonForFingerprint_ExistingPinsStayGreen(t *testing.T) {
	if a, b := normalizeReasonForFingerprint("verdict conflict narrative=PASS"), normalizeReasonForFingerprint("verdict conflict narrative=WARN"); a != b {
		t.Errorf("narrative=<verdict> pin regressed: %q vs %q", a, b)
	}
	if a, b := normalizeReasonForFingerprint("protectedsurface red in 1.478s"), normalizeReasonForFingerprint("protectedsurface red in 1.495s"); a != b {
		t.Errorf("go-test duration pin regressed: %q vs %q", a, b)
	}
}
