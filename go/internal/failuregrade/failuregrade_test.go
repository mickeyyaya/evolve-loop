package failuregrade

import "testing"

func TestGrade(t *testing.T) {
	cases := []struct {
		name   string
		reason string
		ev     Evidence
		want   Tier
	}{
		// Challenge-token is always correctable (no evidence needed).
		{"missing challenge token → correct", "report failed: missing_challenge_token", Evidence{}, TierCorrect},
		{"missing token, evidence irrelevant → correct", "missing_challenge_token", Evidence{RebuildVerified: true}, TierCorrect},

		// Tree-diff churn grades to quarantine ONLY when proven benign.
		{"tree-diff guard + benign → quarantine", `tree-diff guard: phase "build" leaked paths: [x]`, Evidence{ChurnIsBenign: true}, TierQuarantine},
		{"tree-diff guard, NOT benign → abort (floor)", `tree-diff guard: phase "build" leaked paths: [x]`, Evidence{}, TierAbort},

		// SELF_SHA grades to repair ONLY after a verified rebuild; else it is
		// real tampering and must abort.
		{"self-sha + verified rebuild → repair", "ship: SELF_SHA_TAMPERED", Evidence{RebuildVerified: true}, TierRepair},
		{"self-sha, unverified → abort (real tampering)", "ship: SELF_SHA_TAMPERED", Evidence{}, TierAbort},
		{"self-sha, only churn-benign set → abort", "SELF_SHA_TAMPERED", Evidence{ChurnIsBenign: true}, TierAbort},

		// Floor invariant: anything outside the closed vocabulary aborts, even
		// with evidence set (evidence cannot conjure a grade for an unknown class).
		{"unknown reason → abort", "audit FAIL: H1 correctness defect", Evidence{ChurnIsBenign: true, RebuildVerified: true}, TierAbort},
		{"empty reason → abort", "", Evidence{}, TierAbort},
		{"prose mentioning a token but not the code → abort", "the report should echo a challenge token", Evidence{}, TierAbort},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Grade(c.reason, c.ev); got != c.want {
				t.Fatalf("Grade(%q, %+v) = %v, want %v", c.reason, c.ev, got, c.want)
			}
		})
	}
}

func TestTierString(t *testing.T) {
	for tier, want := range map[Tier]string{
		TierAbort:      "abort",
		TierCorrect:    "correct",
		TierQuarantine: "quarantine",
		TierRepair:     "repair",
		Tier(99):       "abort",
	} {
		if got := tier.String(); got != want {
			t.Errorf("Tier(%d).String() = %q, want %q", tier, got, want)
		}
	}
}
