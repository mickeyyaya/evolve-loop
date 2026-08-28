package policy

// retry_policy_for_test.go — the accessor that finally consumes ADR-0072's
// declarative retry policy. Action, MaxRetries and FixType have been declared in
// the category table since the policy shipped and were read by nothing; the retry
// decision used a parallel knob instead. This pins the table as the one authority.

import "testing"

func TestRetryPolicyFor(t *testing.T) {
	fp := DefaultSystemFailurePolicy()

	tests := []struct {
		name        string
		category    string
		wantOK      bool
		wantLevel   string
		wantAction  string
		wantRetries int
		wantFixType string
	}{
		{
			name:     "code-audit-fail is a task-level retry-with-fix, capped at 2",
			category: CategoryCodeAuditFail, wantOK: true, wantLevel: LevelTask,
			wantAction: ActionRetryWithFix, wantRetries: 2, wantFixType: "address-audit-findings",
		},
		{
			name:     "code-build-fail is likewise retryable",
			category: CategoryCodeBuildFail, wantOK: true, wantLevel: LevelTask,
			wantAction: ActionRetryWithFix, wantRetries: 2, wantFixType: "build-repair",
		},
		{
			name:     "infra-systemic is a system-level halt, not a retry",
			category: CategoryInfraSystemic, wantOK: true, wantLevel: LevelSystem,
			wantAction: ActionHaltAndDiagnose,
		},
		{
			name:     "intent-malformed is task-level but not retryable",
			category: CategoryIntentMalformed, wantOK: true, wantLevel: LevelTask,
			wantAction: ActionDeferOrQuarantine,
		},
		{
			// ok=false must be treated as "no retry" by callers, never default-allow.
			name: "an unknown category is not known", category: "nobody-declared-this",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := fp.RetryPolicyFor(tc.category)

			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if got.Level != tc.wantLevel {
				t.Errorf("Level = %q, want %q", got.Level, tc.wantLevel)
			}
			if got.Action != tc.wantAction {
				t.Errorf("Action = %q, want %q", got.Action, tc.wantAction)
			}
			if tc.wantRetries != 0 && got.MaxRetries != tc.wantRetries {
				t.Errorf("MaxRetries = %d, want %d", got.MaxRetries, tc.wantRetries)
			}
			if tc.wantFixType != "" && got.FixType != tc.wantFixType {
				t.Errorf("FixType = %q, want %q", got.FixType, tc.wantFixType)
			}
		})
	}
}
