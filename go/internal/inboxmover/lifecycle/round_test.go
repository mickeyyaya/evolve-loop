package lifecycle

import "testing"

func TestPlanRound(t *testing.T) {
	tests := []struct {
		name                         string
		failures, ceiling            int
		try                          int
		carry, raise, changeStrategy bool
		wantPlan                     string
	}{
		{"first run is plain", 0, 4, 1, false, false, false, "raise-with-findings"},
		{"second run carries findings and raises effort", 1, 4, 2, true, true, false, "raise-with-findings"},
		{"third run changes the strategy", 2, 4, 3, true, true, true, "fresh-deep-plan"},
		{"later run keeps the changed strategy", 3, 4, 4, true, true, true, "fresh-deep-plan"},
		{"the final run of a ceiling of two changes the strategy", 1, 2, 2, true, true, true, "fresh-deep-plan"},
		{"no ceiling still changes the strategy on the third run", 2, 0, 3, true, true, true, "fresh-deep-plan"},
		{"no ceiling raises on the second run only", 1, 0, 2, true, true, false, "raise-with-findings"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PlanRound(tc.failures, tc.ceiling)
			if got.Try != tc.try || got.CarryFindings != tc.carry || got.RaiseEffort != tc.raise || got.ChangeStrategy != tc.changeStrategy {
				t.Fatalf("PlanRound(%d, %d) = %+v", tc.failures, tc.ceiling, got)
			}
			if plan := got.Plan(); plan != tc.wantPlan {
				t.Fatalf("Plan() = %q, want %q", plan, tc.wantPlan)
			}
		})
	}
}

func TestCutAtRune_BacksOffAContinuationByteAndStopsAtTheStart(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in    string
		limit int
		want  string
	}{
		{"abc", 5, "abc"},
		{"ab界", 3, "ab"},
		{"\x80\x80\x80\x80", 2, ""},
	}
	for _, c := range cases {
		if got := cutAtRune(c.in, c.limit); got != c.want {
			t.Errorf("cutAtRune(%q, %d) = %q, want %q", c.in, c.limit, got, c.want)
		}
	}
}
