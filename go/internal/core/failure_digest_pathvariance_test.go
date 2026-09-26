package core

import "testing"

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

func TestNormalizeReasonForFingerprint_ExistingPinsStayGreen(t *testing.T) {
	if a, b := normalizeReasonForFingerprint("verdict conflict narrative=PASS"), normalizeReasonForFingerprint("verdict conflict narrative=WARN"); a != b {
		t.Errorf("narrative=<verdict> pin regressed: %q vs %q", a, b)
	}
	if a, b := normalizeReasonForFingerprint("protectedsurface red in 1.478s"), normalizeReasonForFingerprint("protectedsurface red in 1.495s"); a != b {
		t.Errorf("go-test duration pin regressed: %q vs %q", a, b)
	}
}
