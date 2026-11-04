package checkpoint

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestGrantOperatorResume_RestoresTheBudgetOfAnUnknownResetAndKeepsTheRest(t *testing.T) {
	t.Parallel()
	path := writeState(t, t.TempDir(), `{"cycle_id":2138,"phase":"audit","checkpoint":{"enabled":true,"reason":"quota-likely","resumeFromPhase":"audit","autoResumeAttempts":0,"autoResumeMaxAttempts":0,"quotaResetSource":"unknown","operatorAction":"no quota reset time is known"}}`)
	if _, admitted, err := RecordResume(path, true); err != nil || admitted {
		t.Fatalf("before the grant = (admitted %v, err %v), want refused", admitted, err)
	}

	cp, granted, err := GrantOperatorResume(path)

	if err != nil || !granted || cp.AutoResumeAttempts != 0 || cp.AutoResumeMaxAttempts != DefaultAutoResumeAttempts || cp.OperatorAction != "" || cp.QuotaResetSource != operatorGrantSource {
		t.Fatalf("grant = %+v granted %v err %v, want attempts 0 of %d, no operator action, source %s", cp, granted, err, DefaultAutoResumeAttempts, operatorGrantSource)
	}
	var state struct {
		Phase      string     `json:"phase"`
		Checkpoint Checkpoint `json:"checkpoint"`
	}
	raw, _ := os.ReadFile(path)
	if err := json.Unmarshal(raw, &state); err != nil || state.Phase != "audit" || state.Checkpoint.Reason != ReasonQuotaLikely || state.Checkpoint.ResumeFromPhase != "audit" || !state.Checkpoint.Enabled {
		t.Errorf("state after the grant = %+v (err %v), want the phase, the reason and the resume phase kept", state, err)
	}
	if _, admitted, err := RecordResume(path, true); err != nil || !admitted {
		t.Errorf("after the grant = (admitted %v, err %v), want admitted", admitted, err)
	}
}

func TestGrantOperatorResume_ClearsAFutureResetTimeAndKeepsALargerSpentCap(t *testing.T) {
	t.Parallel()
	path := writeState(t, t.TempDir(), `{"cycle_id":7,"checkpoint":{"enabled":true,"reason":"quota-likely","autoResumeAttempts":9,"autoResumeMaxAttempts":9,"quotaResetAt":"2999-01-01T00:00:00Z","quotaResetSource":"parsed"}}`)
	cp, granted, err := GrantOperatorResume(path)
	if err != nil || !granted || cp.AutoResumeAttempts != 0 || cp.AutoResumeMaxAttempts != 9 || cp.QuotaResetAt != "" {
		t.Fatalf("grant = %+v granted %v err %v, want attempts 0 of the larger cap 9 and no reset time", cp, granted, err)
	}
}

func TestGrantOperatorResume_LeavesACheckpointThatIsNotASpentQuotaPauseUnchanged(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"a lane already adopted it": `{"cycle_id":7,"checkpoint":{"enabled":true,"reason":"quota-likely","autoResumeAttempts":1,"autoResumeMaxAttempts":3}}`,
		"another pause reason":      `{"cycle_id":7,"checkpoint":{"enabled":true,"reason":"operator-requested","autoResumeAttempts":0,"autoResumeMaxAttempts":0}}`,
		"a disabled checkpoint":     `{"cycle_id":7,"checkpoint":{"enabled":false,"reason":"quota-likely","autoResumeAttempts":0,"autoResumeMaxAttempts":0}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path := writeState(t, t.TempDir(), body)
			before, _ := os.ReadFile(path)

			_, granted, err := GrantOperatorResume(path)

			after, _ := os.ReadFile(path)
			if err != nil || granted || string(before) != string(after) {
				t.Fatalf("granted %v err %v changed %v, want no grant and no write", granted, err, string(before) != string(after))
			}
		})
	}
}

func TestGrantOperatorResume_AnUnreadableBudgetIsAnError(t *testing.T) {
	t.Parallel()
	path := writeState(t, t.TempDir(), unreadableBudgetState)
	if _, _, err := GrantOperatorResume(path); err == nil {
		t.Fatal("an unreadable checkpoint must not be granted")
	}
}

func TestResumeReady_IsTheBudgetRuleOfAdmissionPlusAPassedReset(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name            string
		attempts, max   int
		resetAt         string
		ready, budgeted bool
	}{
		{"budget left, no reset time", 0, 3, "", true, true},
		{"budget spent", 3, 3, "", false, false},
		{"a reset time that passed", 1, 3, now.Add(-time.Minute).Format(time.RFC3339), true, true},
		{"a reset at this instant", 1, 3, now.Format(time.RFC3339), true, true},
		{"a reset time still ahead", 1, 3, now.Add(time.Minute).Format(time.RFC3339), false, true},
		{"an unknown reset with no budget", 0, 0, "", false, false},
	}
	for _, tc := range cases {
		if got, err := ResumeReady(tc.attempts, tc.max, tc.resetAt, now); got != tc.ready || err != nil {
			t.Errorf("%s: ResumeReady = %v err %v, want %v", tc.name, got, err, tc.ready)
		}
		if got := BudgetLeft(tc.attempts, tc.max); got != tc.budgeted {
			t.Errorf("%s: BudgetLeft = %v, want %v", tc.name, got, tc.budgeted)
		}
		if _, admitted := nextResume(Checkpoint{AutoResumeAttempts: tc.attempts, AutoResumeMaxAttempts: tc.max}, true); admitted != tc.budgeted {
			t.Errorf("%s: admission = %v, want the BudgetLeft rule %v", tc.name, admitted, tc.budgeted)
		}
	}
}
