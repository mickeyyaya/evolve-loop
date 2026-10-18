package cyclehealth

import "testing"

func TestClassifyOutcome_TheRecordedPlannedNoWorkEndIsNoWork(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, timing string
		want         Outcome
	}{
		{"cycle 1758: triage's host ending records the no-work reason", `[{"phase":"scout","verdict":"PASS"},{"phase":"triage","verdict":"PASS"},{"phase":"triage","verdict":"SKIPPED","abort_reason":"triage-empty-commitment"}]`, OutcomeNoWork},
		{"a claim failure is explained, never no-work", `[{"phase":"triage","verdict":"PASS"},{"phase":"triage","verdict":"FAIL","abort_reason":"triage-empty-commitment-claimable-work"}]`, OutcomeFailedExplained},
		{"a ship PASS still wins", `[{"phase":"triage","verdict":"SKIPPED","abort_reason":"triage-empty-commitment"},{"phase":"ship","verdict":"PASS"}]`, OutcomeShipped},
		{"nothing recorded stays the alarm", `[{"phase":"scout","verdict":"PASS"},{"phase":"triage","verdict":"PASS"}]`, OutcomeFailedUnexplained},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeTiming(t, dir, tc.timing)
			if got, detail := ClassifyOutcome(dir); got != tc.want {
				t.Errorf("outcome=%s (detail=%q), want %s", got, detail, tc.want)
			}
		})
	}
}

func TestClassifyOutcome_AProtectedPredicateRouteIsAnExplainedFailureThatNamesTheRoute(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTiming(t, dir, `[{"phase":"tdd","verdict":"PASS"},{"phase":"tdd","verdict":"FAIL","abort_reason":"tdd-protected-predicate-route"}]`)
	got, detail := ClassifyOutcome(dir)
	if got != OutcomeFailedExplained || detail != "tdd-protected-predicate-route" {
		t.Errorf("outcome=%s detail=%q: a route is counted as a failure and named, never neutral", got, detail)
	}
}
