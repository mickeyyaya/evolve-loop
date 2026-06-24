// budget_flags_test.go pins the behavior of stripRemovedBudgetFlags — the
// pre-parse shim that removes the retired cost-budget flags
// (--budget-usd / --budget / --batch-cap-usd) from argv so they are no longer
// part of the CLI parameter surface, while keeping old invocations from
// crashing (graceful strip + one-line WARN, not a parse error).
package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestStripRemovedBudgetFlags(t *testing.T) {
	cases := []struct {
		name     string
		in       []string
		want     []string
		wantWarn bool
	}{
		{
			name:     "space-separated value, double dash",
			in:       []string{"--budget-usd", "5", "fix bug"},
			want:     []string{"fix bug"},
			wantWarn: true,
		},
		{
			name:     "equals form",
			in:       []string{"--budget-usd=5", "fix bug"},
			want:     []string{"fix bug"},
			wantWarn: true,
		},
		{
			name:     "single dash",
			in:       []string{"-budget-usd", "5", "fix bug"},
			want:     []string{"fix bug"},
			wantWarn: true,
		},
		{
			name:     "budget alias",
			in:       []string{"--budget", "5", "fix bug"},
			want:     []string{"fix bug"},
			wantWarn: true,
		},
		{
			name:     "batch-cap-usd",
			in:       []string{"--batch-cap-usd", "20", "fix bug"},
			want:     []string{"fix bug"},
			wantWarn: true,
		},
		{
			name:     "no budget flag is untouched and silent",
			in:       []string{"--cycles", "3", "fix bug"},
			want:     []string{"--cycles", "3", "fix bug"},
			wantWarn: false,
		},
		{
			name:     "does not eat an unrelated preceding flag",
			in:       []string{"--strategy", "balanced", "--budget-usd", "5", "fix bug"},
			want:     []string{"--strategy", "balanced", "fix bug"},
			wantWarn: true,
		},
		{
			name:     "positional goal containing the word budget is preserved",
			in:       []string{"--budget-usd", "3", "fix", "the", "budget", "leak"},
			want:     []string{"fix", "the", "budget", "leak"},
			wantWarn: true,
		},
		{
			name:     "multiple budget flags warn only once",
			in:       []string{"--budget-usd", "5", "--batch-cap-usd", "10", "fix bug"},
			want:     []string{"fix bug"},
			wantWarn: true,
		},
		{
			name:     "trailing flag with no value does not panic",
			in:       []string{"fix bug", "--budget-usd"},
			want:     []string{"fix bug"},
			wantWarn: true,
		},
		{
			// A negative value starts with '-' but must be consumed as the flag's
			// value, not left behind for flag.Parse to choke on.
			name:     "negative value is consumed",
			in:       []string{"--budget-usd", "-1", "fix bug"},
			want:     []string{"fix bug"},
			wantWarn: true,
		},
		{
			// A following real flag (non-numeric) must NOT be swallowed as a value.
			name:     "following flag is not eaten as a value",
			in:       []string{"--budget-usd", "--cycles", "3", "fix bug"},
			want:     []string{"--cycles", "3", "fix bug"},
			wantWarn: true,
		},
		{
			// "NaN"/"Inf" parse as floats but are not real budget values; a goal
			// token of that form must survive (treated as a positional, not a value).
			name:     "non-finite token is not swallowed as a value",
			in:       []string{"--budget-usd", "NaN", "fix bug"},
			want:     []string{"NaN", "fix bug"},
			wantWarn: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			warns := 0
			var lastMsg string
			got := stripRemovedBudgetFlags(tc.in, func(m string) {
				warns++
				lastMsg = m
			})
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("stripped args = %q, want %q", got, tc.want)
			}
			switch {
			case tc.wantWarn && warns != 1:
				t.Errorf("warn called %d times, want exactly 1 (one consolidated notice)", warns)
			case !tc.wantWarn && warns != 0:
				t.Errorf("warn called %d times, want 0 (no budget flag present)", warns)
			}
			if tc.wantWarn && !strings.Contains(lastMsg, "--cycles") {
				t.Errorf("warn message must point operators to --cycles; got %q", lastMsg)
			}
		})
	}
}
